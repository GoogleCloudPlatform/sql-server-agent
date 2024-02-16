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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/instanceinfo"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/remote"
)

/*
The production library uses the ExecuteCommand function from commandlineexecutor and the
EvalSymlinks function from filepath.  We need to be able to mock these functions in our unit tests.
*/
var (
	symLinkCommand = filepath.EvalSymlinks
)

const (
	persistentDisk = "PersistentDisk"
	ephemeralDisk  = "EphemeralDisk"
)

// highPerformanceProfile are all tuned power profiles that will be considered high performance best practice
var highPerformanceProfile = map[string]bool{
	"mssql":                  true,
	"throughput-performance": true,
}

// HighPerformanceProfiles public getter for highPerformanceProfile
func HighPerformanceProfiles() map[string]bool { return highPerformanceProfile }

// LinuxCollector is the collector for linux systems.
type LinuxCollector struct {
	ipaddr                 string
	username               string
	privateKeyPath         string
	disks                  [](*instanceinfo.Disks)
	physicalDriveToDiskMap map[string]string
	guestRuleCommandMap    map[string]commandExecutor
	lshwRegexMapping       map[string]*regexp.Regexp
	remote                 bool
	port                   int32
	remoteRunner           remote.Executor
	localExecutor          commandlineexecutor.Execute
}

type commandExecutor struct {
	command          string
	isRule           bool
	runCommand       func(context.Context, string, commandlineexecutor.Execute) (string, error)
	runRemoteCommand func(context.Context, string, remote.Executor) (string, error)
}

type disk struct {
	logicalname string
	diskType    string
}

type lshwEntry struct {
	Product     string `json:"product"`
	LogicalName string `json:"logicalname"`
	Size        int    `json:"size"`
}

var lshwFieldsToParse = []string{
	"product", "logicalname", "size", "Device File", "Device", "Capacity",
}

func lshwFields() []string { return lshwFieldsToParse }

// LinuxCollectionOSFields returns all expected fields in OS collection
func LinuxCollectionOSFields() []string {
	return append(defaultOSFields, linuxAdditionalOsFields...)
}

// NewLinuxCollector initializes and returns a new LinuxCollector object.
func NewLinuxCollector(disks []*instanceinfo.Disks, ipAddr, username, privateKeyPath string, isRemote bool, port int32) *LinuxCollector {
	c := LinuxCollector{
		ipaddr:                 ipAddr,
		username:               username,
		privateKeyPath:         privateKeyPath,
		disks:                  disks,
		guestRuleCommandMap:    map[string]commandExecutor{},
		physicalDriveToDiskMap: map[string]string{},
		lshwRegexMapping:       map[string]*regexp.Regexp{},
		remote:                 isRemote,
		port:                   port,
	}

	if c.remote {
		c.remoteRunner = remote.NewRemote(c.ipaddr, c.username, c.port)
		c.setUpRegex()
		if err := c.remoteRunner.SetupKeys(c.privateKeyPath); err != nil {
			log.Logger.Error(err)
			c.remoteRunner = nil
		} else if err := c.remoteRunner.CreateClient(); err != nil {
			log.Logger.Error(err)
			c.remoteRunner = nil
		}
	} else {
		c.localExecutor = commandlineexecutor.ExecuteCommand
	}

	c.InitializeLinuxOSRules()
	return &c
}

// setUpRegex initializes the needed regex's to parse output of a remote lshw and hwinfo call
func (c *LinuxCollector) setUpRegex() {
	for _, field := range lshwFields() {
		if field == "size" {
			expression := fmt.Sprintf(`"%s" : (\d+?)[\D]`, field)
			reg := regexp.MustCompile(expression)
			c.lshwRegexMapping[field] = reg
		} else if field == "logicalname" || field == "product" {
			expression := fmt.Sprintf(`"%s" : "(.*?)"`, field)
			reg := regexp.MustCompile(expression)
			c.lshwRegexMapping[field] = reg
		} else if field == "Capacity" {
			expression := fmt.Sprintf(`%s: .*\((\d+?)[\D]`, field)
			reg := regexp.MustCompile(expression)
			c.lshwRegexMapping[field] = reg
		} else if field == "Device" {
			expression := fmt.Sprintf(`%s: "(.*?)"`, field)
			reg := regexp.MustCompile(expression)
			c.lshwRegexMapping[field] = reg
		} else {
			expression := fmt.Sprintf(`%s: ([^\s]+)`, field)
			reg := regexp.MustCompile(expression)
			c.lshwRegexMapping[field] = reg
		}
	}
}

// MarkUnknownOSFields checks the collected os fields; if nil or missing, then the data is marked as unknown
func (c *LinuxCollector) MarkUnknownOSFields(details *[]internal.Details) error {
	if len(*details) != 1 {
		return fmt.Errorf("CheckOSCollectedMetrics details should have only 1 field for OS collection, got %d", len(*details))
	}
	detail := (*details)[0]
	if detail.Name != "OS" {
		return fmt.Errorf("CheckOSCollectedMetrics details.name should be collecting for OS, got %s", detail.Name)
	}
	if len(detail.Fields) > 1 {
		return fmt.Errorf("CheckOSCollectedMetrics details.fields should have 1 field in OS collection, got %d", len(detail.Fields))
	}

	if len(detail.Fields) == 0 {
		fields := map[string]string{
			internal.PowerProfileSettingRule:     "unknown",
			internal.LocalSSDRule:                "unknown",
			internal.DataDiskAllocationUnitsRule: "unknown",
		}
		(*details)[0].Fields = append((*details)[0].Fields, fields)
		return nil
	}

	// for os collection, details only has one element and details.Fields only has one element
	// sql collections is different as there can be multiple details and multiple details.Fields
	for _, field := range LinuxCollectionOSFields() {
		_, ok := detail.Fields[0][field]
		if !ok {
			(*details)[0].Fields[0][field] = "unknown"
		}
	}
	return nil
}

// DiskToDiskType maps physical drive to disktype. EX: /dev/sda to local_ssd
func DiskToDiskType(fields map[string]string, disks []*instanceinfo.Disks) {
	logicalToTypeMap := map[string]string{}
	for _, devices := range disks {
		var err error
		devices.Mapping, err = forLinux(devices.DeviceName)
		if err != nil {
			log.Logger.Warnw("No mapping for instance disk", "disk", devices.DeviceName, "error", err)
		} else {
			// EX: sda -> PERSISTENT
			logicalToTypeMap[devices.Mapping] = devices.DiskType
		}
		log.Logger.Debugw("Instance disk is mapped to device name", "devicename", devices.DeviceName, "mapping", devices.Mapping)
	}
	r, err := json.Marshal(logicalToTypeMap)
	if err != nil {
		log.Logger.Errorw("An error occured while serializing disk info to JSON", "error", err)
	}
	if len(logicalToTypeMap) == 0 {
		fields[internal.LocalSSDRule] = "unknown"
	} else {
		fields[internal.LocalSSDRule] = string(r)
	}
}

/*
forLinux returns the name of the Linux physical disk mapped to "deviceName". (sda1, hda1, sdb1,
etc...)
*/
func forLinux(deviceName string) (string, error) {
	path, err := symLinkCommand("/dev/disk/by-id/google-" + deviceName)
	if err != nil {
		return "", err
	}

	if path != "" {
		path = strings.TrimSuffix(filepath.Base(path), "\n")
	}
	log.Logger.Debugw("Mapping for device", "name", deviceName, "mapping", path)
	return path, nil
}

func (c *LinuxCollector) findLshwFields(lshwResult string) (lshwEntry, error) {
	logicalName, logicalNameErr := c.findLshwFieldString(lshwResult, "logicalname")
	if logicalNameErr != nil {
		return lshwEntry{}, logicalNameErr
	}
	product, productErr := c.findLshwFieldString(lshwResult, "product")
	if productErr != nil {
		return lshwEntry{}, productErr
	}
	size, sizeErr := c.findLshwFieldInt(lshwResult, "size")
	if sizeErr != nil {
		return lshwEntry{}, sizeErr
	}

	return lshwEntry{LogicalName: logicalName, Product: product, Size: size}, nil
}

func (c *LinuxCollector) findHwinfoFields(lshwResult string) (lshwEntry, error) {
	logicalName, logicalNameErr := c.findLshwFieldString(lshwResult, "Device File")
	if logicalNameErr != nil {
		return lshwEntry{}, logicalNameErr
	}
	product, productErr := c.findLshwFieldString(lshwResult, "Device")
	if productErr != nil {
		return lshwEntry{}, productErr
	}
	size, sizeErr := c.findLshwFieldInt(lshwResult, "Capacity")
	if sizeErr != nil {
		return lshwEntry{}, sizeErr
	}

	return lshwEntry{LogicalName: logicalName, Product: product, Size: size}, nil
}

func (c *LinuxCollector) findLshwFieldString(lshwResult string, field string) (string, error) {
	reg, ok := c.lshwRegexMapping[field]
	if !ok {
		return "", fmt.Errorf("regexp did not find %s field", field)
	}
	match := reg.FindStringSubmatch(lshwResult)
	if len(match) <= 1 {
		return "", fmt.Errorf("regexp did not find %s field", field)
	}
	resultArr := strings.Split(match[1], "/")
	return resultArr[len(resultArr)-1], nil
}

func (c *LinuxCollector) findLshwFieldInt(lshwResult string, field string) (int, error) {
	reg, ok := c.lshwRegexMapping[field]
	if !ok {
		return 0, fmt.Errorf("regexp did not find %s field", field)
	}
	match := reg.FindStringSubmatch(lshwResult)
	if len(match) <= 1 {
		return 0, fmt.Errorf("regexp did not find %s field", field)
	}
	result, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("unable to convert %s from string to int: error %v", field, err)
	}

	return result, nil
}

// findPowerProfile takes input string of command tuned-adm active, and gets the power profile
func findPowerProfile(powerProfileFull string) (string, error) {
	powerProfile := strings.Split(powerProfileFull, ": ")

	if len(powerProfile) < 2 || powerProfile[0] != "Current active profile" {
		return "", fmt.Errorf(`Check help docs. Expected power profile format to be  "Current active profile: <profile>. Actual result: ` + powerProfileFull)
	}
	if HighPerformanceProfiles()[powerProfile[1]] {
		return "High performance", nil
	}

	return powerProfile[1], nil
}

// CollectGuestRules collects os guest os rules
func (c *LinuxCollector) CollectGuestRules(ctx context.Context, timeout time.Duration) internal.Details {
	details := internal.Details{
		Name: "OS",
	}
	fields := map[string]string{}

	if !c.remote {
		if c.localExecutor == nil {
			fields[internal.LocalSSDRule] = "unknown"
			details.Fields = append(details.Fields, fields)
			log.Logger.Error("Local executor is nil. Contact customer support.")
			return details
		}
		ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ch := make(chan bool, 1)
		go func() {
			DiskToDiskType(fields, c.disks)
			ch <- true
		}()
		select {
		case <-ctxWithTimeout.Done():
			log.Logger.Errorf("DiskToDiskType() for local linux disktype timeout")
		case <-ch:
		}

	} else {
		if c.remoteRunner == nil {
			fields[internal.LocalSSDRule] = "unknown"
			details.Fields = append(details.Fields, fields)
			log.Logger.Debugw("Remoterunner is nil. Remote collection attempted when ssh keys aren't set up correctly. Check customer support documentation.")
			return details
		}
	}

	for _, rule := range LinuxCollectionOSFields() {
		exe := c.guestRuleCommandMap[rule]
		func() {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			ch := make(chan bool, 1)
			go func() {
				if c.remote {
					res, err := exe.runRemoteCommand(ctx, exe.command, c.remoteRunner)
					if err != nil {
						if strings.Contains(err.Error(), "Check help docs") {
							log.Logger.Warnw("Failed to run remote command. Install command on linux vm to collect more data", "command", exe.command, "error", err)
						} else {
							log.Logger.Errorw("Failed to run remote command", "command", exe.command, "error", err)
						}
						fields[rule] = "unknown"
						ch <- false
						return
					} else if res == "null" {
						fields[rule] = "unknown"
						ch <- false
						return
					}
					fields[rule] = res
				} else if exe.isRule { // local calls are only made if isrule is true
					res, err := exe.runCommand(ctx, exe.command, c.localExecutor)
					if err != nil {
						if strings.Contains(err.Error(), "Check help docs") {
							log.Logger.Warnw("Failed to run remote command. Install command on linux vm to collect more data", "command", exe.command, "error", err)
						} else {
							log.Logger.Errorw("Failed to run command", "command", exe.command, "error", err)
						}
						fields[rule] = "unknown"
						ch <- false
						return
					} else if res == "null" {
						fields[rule] = "unknown"
						ch <- false
						return
					}
					fields[rule] = res
				}
				ch <- true
			}()

			select {
			case <-ctxWithTimeout.Done():
				log.Logger.Errorf("Running linux guest rule %s timeout", rule)
			case <-ch:
			}

		}()

	}
	details.Fields = append(details.Fields, fields)
	return details
}
