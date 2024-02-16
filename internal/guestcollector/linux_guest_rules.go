/*
Copyright 2023 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package guestcollector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/remote"
)

const (
	localSSDCommand                = "sudo lshw -class disk -json"
	localSSDCommandForSuse         = "sudo hwinfo --disk"
	powerPlanCommand               = "sudo tuned-adm active"
	dataDiskAllocationUnitsCommand = "sudo blockdev --getbsz /dev/"
)

// linuxAdditionalOsFields are all expected fields in OS collection in collection order.
// that are not part of windows os collection.
var linuxAdditionalOsFields = []string{}

// InitializeLinuxOSRules initializes all linux OS rules.
func (c *LinuxCollector) InitializeLinuxOSRules() {
	c.guestRuleCommandMap[internal.LocalSSDRule] = commandExecutor{
		command: localSSDCommand,
		isRule:  false,
		runCommand: func(ctx context.Context, command string, exec commandlineexecutor.Execute) (string, error) {
			// LocalSSDRule is collected differently, check DiskToDiskType method
			return "", nil
		},
		runRemoteCommand: func(ctx context.Context, command string, r remote.Executor) (string, error) {
			var isLinuxSuse bool
			lshwResult, err := remote.RunCommandWithPipes(command, r)
			if err != nil {
				lshwResult, err = remote.RunCommandWithPipes(localSSDCommandForSuse, r)
				if err != nil {
					return "", err
				}
				log.Logger.Debugw("Fetched the disk info by using hwinfo.")
				isLinuxSuse = true
			}

			var lshwFields lshwEntry
			if !isLinuxSuse {
				lshwFields, err = c.findLshwFields(lshwResult)
			} else {
				lshwFields, err = c.findHwinfoFields(lshwResult)
			}
			if err != nil {
				return "", err
			}

			diskType := internal.Other.String()
			if lshwFields.Product == persistentDisk {
				diskType = internal.PersistentSSD.String()
			} else if lshwFields.Product == ephemeralDisk && lshwFields.Size%402653184000 == 0 {
				diskType = internal.LocalSSD.String()
			}

			c.physicalDriveToDiskMap[lshwFields.LogicalName] = diskType

			res, errMar := json.Marshal(c.physicalDriveToDiskMap)
			if errMar != nil {
				return "", errMar
			}
			return string(res), nil
		},
	}
	c.guestRuleCommandMap[internal.PowerProfileSettingRule] = commandExecutor{
		command: powerPlanCommand,
		isRule:  true,
		runCommand: func(ctx context.Context, command string, exec commandlineexecutor.Execute) (string, error) {
			res, err := internal.CommandLineExecutorWrapper(ctx, "/bin/sh", fmt.Sprintf(" -c '%s'", command), exec)
			if err != nil {
				return "", fmt.Errorf("Check help docs, tuned package not installed or no power profile set. " + err.Error())
			}
			return findPowerProfile(res)
		},
		runRemoteCommand: func(ctx context.Context, command string, r remote.Executor) (string, error) {
			s, err := r.CreateSession("")
			if err != nil {
				return "", err
			}
			defer s.Close()
			res, err := r.Run(command, s)
			if err != nil {
				return "", fmt.Errorf("Check help docs, tuned package not installed or no power profile set. " + err.Error())
			}
			return findPowerProfile(res)
		},
	}
	c.guestRuleCommandMap[internal.DataDiskAllocationUnitsRule] = commandExecutor{
		command: dataDiskAllocationUnitsCommand,
		isRule:  true,
		runCommand: func(ctx context.Context, command string, exec commandlineexecutor.Execute) (string, error) {
			if c.disks == nil || len(c.disks) == 0 {
				return "", fmt.Errorf("data disk allocation failed. no disks found")
			}

			type resultEle struct {
				BlockSize string
				Caption   string
			}

			var result []resultEle

			for _, disk := range c.disks {
				if disk.Mapping == "" {
					continue
				}
				fullCommand := command + disk.Mapping
				blockSize, err := internal.CommandLineExecutorWrapper(ctx, "/bin/sh", fmt.Sprintf(" -c '%s'", fullCommand), exec)
				if err != nil {
					return "", err
				}
				result = append(result, resultEle{BlockSize: blockSize, Caption: disk.Mapping})
			}
			res, err := json.Marshal(result)
			if err != nil {
				return "", err
			}
			return string(res), nil
		},
		runRemoteCommand: func(ctx context.Context, command string, r remote.Executor) (string, error) {
			if c.physicalDriveToDiskMap == nil || len(c.physicalDriveToDiskMap) == 0 {
				return "", fmt.Errorf("data disk allocation failed. no disks found")
			}

			type resultEle struct {
				BlockSize string
				Caption   string
			}
			var result []resultEle

			for physicalDrive := range c.physicalDriveToDiskMap {
				fullCommand := command + physicalDrive
				s, err := r.CreateSession("")
				if err != nil {
					return "", err
				}
				blockSize, err := r.Run(fullCommand, s)
				s.Close()
				if err != nil || blockSize == "" {
					blockSize = "unknown"
				}
				result = append(result, resultEle{BlockSize: blockSize, Caption: physicalDrive})
			}
			res, err := json.Marshal(result)
			if err != nil {
				return "", err
			}
			return string(res), nil
		},
	}
	// TODO: b/324454053 - add disk readahead here and future rules here
}
