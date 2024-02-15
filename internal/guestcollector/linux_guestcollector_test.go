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
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/crypto/ssh"
	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/instanceinfo"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/remote"
)

type mockLinuxHelper struct {
	outputErr bool
	outputMap map[string]string
	output    string
}

func (m *mockLinuxHelper) DiskToDiskType(fields map[string]string) {
	if !m.outputErr {
		fields["testfield"] = "testvalue"
	}
}

func (m *mockLinuxHelper) ForLinux(deviceName string) (string, error) {
	if m.outputErr {
		return "", errors.New("output error")
	}
	return m.output + "_test", nil
}

func newMockLinuxHelper(outputErr bool) *mockLinuxHelper {
	return &mockLinuxHelper{outputErr: outputErr, output: "any output string"}
}

type mockClient struct {
	outputErr bool
	input     string
}

func (m *mockClient) NewSession() (*ssh.Session, error) {
	return &ssh.Session{Stdin: bytes.NewBufferString(m.input)}, nil
}

func newMockClient(outputErr bool) mockClient {
	return mockClient{outputErr: outputErr, input: "any input string"}
}

type mockRemote struct {
	runErr           bool
	lshwErr          bool
	createSessionErr bool
	input            string
	powerPlanInput   string
}

func newMockRemote(runErr bool, createSessionErr bool, lshwErr bool, powerPlanInput string) *mockRemote {
	return &mockRemote{
		runErr:           runErr,
		createSessionErr: createSessionErr,
		lshwErr:          lshwErr,
		input:            "any input string",
		powerPlanInput:   powerPlanInput,
	}
}

func (m *mockRemote) Run(cmd string, session remote.SSHSessionInterface) (string, error) {
	if m.runErr {
		return "", errors.New("run error")
	}
	if m.lshwErr {
		if cmd == localSSDCommand {
			return "", errors.New("lshw error")
		}
	}
	switch cmd {
	case localSSDCommand:
		return fmt.Sprintf(`[
			{
				"product" : "%s",
				"logicalname" : "/dev/sda",
				"size" : 10737418240
			}
		]`, persistentDisk), nil
	case localSSDCommandForSuse:
		return fmt.Sprintf(`
		Device: "%s"
		Device File: /dev/sda (/dev/sg0)
		Capacity: 64 GB (68719476736 bytes)`, persistentDisk), nil
	case powerPlanCommand:
		return m.powerPlanInput, nil
	case dataDiskAllocationUnitsCommand:
		return "", nil
	default:
		return "unknown", nil
	}
}

func (m *mockRemote) CreateSession(string) (remote.SSHSessionInterface, error) {
	if m.createSessionErr {
		return nil, errors.New("create session error")
	}
	return &mockSession{outputErr: false, input: m.input}, nil
}

func (m *mockRemote) CreateClient() error {
	return nil
}

func (m *mockRemote) SetupKeys(string) error { return nil }

type mockSession struct {
	outputErr bool
	input     string
}

func (m *mockSession) Close() error { return nil }

func (m *mockSession) Output(cmd string) ([]byte, error) {
	if m.outputErr {
		return []byte(""), errors.New("output error")
	}
	return []byte("output"), nil
}

func TestPhysicalDriveToDiskType(t *testing.T) {
	testcases := []struct {
		name         string
		disks        [](*instanceinfo.Disks)
		Command      func(string) (string, error)
		inputDetails map[string]string
		want         map[string]string
	}{
		{
			name: "success",
			disks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   "pd-ssd",
					Mapping:    "",
				},
			},
			inputDetails: map[string]string{
				"testfield": "testvalue",
			},
			Command: func(string) (string, error) {
				return "sda", nil
			},
			want: map[string]string{
				"testfield": "testvalue",
				"local_ssd": `{"sda":"pd-ssd"}`,
			},
		},
		{
			name: "error",
			disks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   "pd-ssd",
					Mapping:    "",
				},
			},
			inputDetails: map[string]string{
				"testfield": "testvalue",
			},
			Command: func(string) (string, error) {
				return "", errors.New("test error")
			},
			want: map[string]string{
				"testfield": "testvalue",
				"local_ssd": "unknown",
			},
		},
	}

	defer func(f func(string) (string, error)) { symLinkCommand = f }(symLinkCommand)
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			symLinkCommand = tc.Command

			DiskToDiskType(tc.inputDetails, tc.disks)
			got := tc.inputDetails

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("DiskToDiskType() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestCollectLinuxGuestRulesLocal(t *testing.T) {
	testcases := []struct {
		name             string
		isPersistentDisk bool
		NoMocking        bool
		powerPlanInput   string
		allDisks         []*instanceinfo.Disks
		want             internal.Details
	}{
		{
			name:      "local: expected output when no mocking is done",
			NoMocking: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:           "local: success happy path",
			powerPlanInput: "Current active profile: throughput-performance",
			allDisks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   internal.PersistentSSD.String(),
					Mapping:    "",
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": `[{"BlockSize":"4096","Caption":"sda"}]`,
						"local_ssd":                  fmt.Sprintf(`{"sda":"%s"}`, internal.PersistentSSD.String()),
						"power_profile_setting":      "High performance",
					},
				},
			},
		},
		{
			name:           "local: power plan balanced",
			powerPlanInput: "Current active profile: balanced",
			allDisks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   internal.PersistentSSD.String(),
					Mapping:    "",
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": `[{"BlockSize":"4096","Caption":"sda"}]`,
						"local_ssd":                  fmt.Sprintf(`{"sda":"%s"}`, internal.PersistentSSD.String()),
						"power_profile_setting":      "balanced",
					},
				},
			},
		},
		{
			name:           "local: power plan wrong output",
			powerPlanInput: "Unexpected output value",
			allDisks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   internal.PersistentSSD.String(),
					Mapping:    "",
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": `[{"BlockSize":"4096","Caption":"sda"}]`,
						"local_ssd":                  fmt.Sprintf(`{"sda":"%s"}`, internal.PersistentSSD.String()),
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name: "local: scratch disktype",
			allDisks: []*instanceinfo.Disks{
				&instanceinfo.Disks{
					DeviceName: "someDevice",
					DiskType:   internal.LocalSSD.String(),
					Mapping:    "",
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": `[{"BlockSize":"4096","Caption":"sda"}]`,
						"local_ssd":                  fmt.Sprintf(`{"sda":"%s"}`, internal.LocalSSD.String()),
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:           "local: local disks is empty",
			powerPlanInput: "Current active profile: balanced",
			allDisks:       []*instanceinfo.Disks{},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "balanced",
					},
				},
			},
		},
	}

	// happy path for disks, as its tested in the TestPhysicalDriveToDiskType() test
	defer func(f func(string) (string, error)) { symLinkCommand = f }(symLinkCommand)
	symLinkCommand = func(string) (string, error) {
		return "sda", nil
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewLinuxCollector(nil, "", "", "", false, 22)
			// happy path for disks, as its tested in the TestPhysicalDriveToDiskType() test
			collector.disks = tc.allDisks

			collector.localExecutor = func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
				switch params.ArgsToSplit {
				case fmt.Sprintf(" -c '%s'", powerPlanCommand):
					return commandlineexecutor.Result{
						StdOut: tc.powerPlanInput,
					}
				case fmt.Sprintf(" -c '%ssda'", dataDiskAllocationUnitsCommand):
					return commandlineexecutor.Result{
						StdOut: "4096",
					}
				default:
					return commandlineexecutor.Result{
						StdErr: "Error, create a new test command case",
					}
				}
			}
			if tc.NoMocking {
				// this unsets all prior mocking
				collector = NewLinuxCollector(nil, "", "", "", false, 22)
			}

			got := collector.CollectGuestRules(context.Background(), time.Minute)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("CollectGuestRules() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestCollectLinuxGuestRulesLocal_Fail(t *testing.T) {
	testcases := []struct {
		name                   string
		ssdRan                 bool
		powerPlanErr           bool
		dataDiskErr            bool
		mockRuleMap            bool
		mockExecutorErr        bool
		localExecutorNil       bool
		commandExecutorMapMock map[string]commandExecutor
		want                   internal.Details
	}{
		{
			name:   "local: running unexpected runCommand() local ssd still returned unknown",
			ssdRan: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:         "local: power plan error",
			powerPlanErr: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:        "local: data disk error",
			dataDiskErr: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:        "local: do not save to result if isRule is false",
			mockRuleMap: true,
			commandExecutorMapMock: map[string]commandExecutor{
				internal.PowerProfileSettingRule: commandExecutor{
					runCommand: func(ctx context.Context, command string, exec commandlineexecutor.Execute) (string, error) {
						return "testvalue", nil
					},
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:        "local: empty detail when runCommand returns error",
			mockRuleMap: true,
			commandExecutorMapMock: map[string]commandExecutor{
				internal.PowerProfileSettingRule: commandExecutor{
					runCommand: func(ctx context.Context, command string, exec commandlineexecutor.Execute) (string, error) {
						return "", fmt.Errorf("error")
					},
				},
			},
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:            "local: invalid command return empty result",
			mockExecutorErr: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:             "local: expected output when localExecutor is used",
			localExecutorNil: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"local_ssd": "unknown",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewLinuxCollector(nil, "", "", "", false, 22)
			collector.localExecutor = func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				if tc.ssdRan {
					return commandlineexecutor.Result{Error: errors.New("ssd error")}
				}
				if tc.powerPlanErr {
					return commandlineexecutor.Result{Error: errors.New("power plan error")}
				}
				if tc.dataDiskErr {
					return commandlineexecutor.Result{Error: errors.New("data disk error")}
				}
				return commandlineexecutor.Result{}
			}
			if tc.mockExecutorErr {
				for r, m := range collector.guestRuleCommandMap {
					m.command = "any query"
					collector.guestRuleCommandMap[r] = m
				}
			}
			if tc.localExecutorNil {
				collector.localExecutor = nil
			}

			got := collector.CollectGuestRules(context.Background(), time.Minute)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("CollectGuestRules() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}

}

func TestCollectLinuxGuestRulesRemote(t *testing.T) {
	testcases := []struct {
		name              string
		powerPlanInput    string
		runErr            bool
		lshwErr           bool
		createSessionErr  bool
		emptyRemoteRunner bool
		want              internal.Details
	}{
		{
			name:           "remote: success",
			powerPlanInput: "Current active profile: High performance",
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{map[string]string{
					"data_disk_allocation_units": `[{"BlockSize":"unknown","Caption":"sda"}]`,
					"local_ssd":                  `{"sda":"PERSISTENT-SSD"}`,
					"power_profile_setting":      "High performance",
				}},
			},
		},
		{
			name:           "remote: success by using hwinfo when lshw fails",
			powerPlanInput: "Current active profile: High performance",
			lshwErr:        true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{map[string]string{
					"data_disk_allocation_units": `[{"BlockSize":"unknown","Caption":"sda"}]`,
					"local_ssd":                  `{"sda":"PERSISTENT-SSD"}`,
					"power_profile_setting":      "High performance",
				}},
			},
		},
		{
			name:           "remote: success power plan is balanced",
			powerPlanInput: "Current active profile: balanced",
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{map[string]string{
					"data_disk_allocation_units": `[{"BlockSize":"unknown","Caption":"sda"}]`,
					"local_ssd":                  `{"sda":"PERSISTENT-SSD"}`,
					"power_profile_setting":      "balanced",
				}},
			},
		},
		{
			name:           "remote: power plan error.",
			powerPlanInput: "error",
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{map[string]string{
					"data_disk_allocation_units": `[{"BlockSize":"unknown","Caption":"sda"}]`,
					"local_ssd":                  `{"sda":"PERSISTENT-SSD"}`,
					"power_profile_setting":      "unknown",
				}},
			},
		},
		{
			name:   "remote: empty detail when runCommand returns error",
			runErr: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:             "remote: empty detail when createSession returns error",
			createSessionErr: true,
			want: internal.Details{
				Name: "OS",
				Fields: []map[string]string{
					map[string]string{
						"data_disk_allocation_units": "unknown",
						"local_ssd":                  "unknown",
						"power_profile_setting":      "unknown",
					},
				},
			},
		},
		{
			name:              "remote: empty remoteRunner",
			emptyRemoteRunner: true,
			want: internal.Details{
				Name:   "OS",
				Fields: []map[string]string{{"local_ssd": "unknown"}},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewLinuxCollector(nil, "", "", "", true, 22)
			if !tc.emptyRemoteRunner {
				collector.remoteRunner = newMockRemote(tc.runErr, tc.createSessionErr, tc.lshwErr, tc.powerPlanInput)
			} else {
				collector.remoteRunner = nil
			}

			got := collector.CollectGuestRules(context.Background(), time.Minute)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("CollectGuestRules() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

// TestCheckLinusOsReturnedCount compares the os returned fields for linux_guestcollector with the returned fields for OSCollectorResultFields
func TestCheckLinusOsReturnedCount(t *testing.T) {
	guestCollectorCount := len(LinuxCollectionOSFields())
	guestCollectorLinuxCount := 0

	testLC := NewLinuxCollector(nil, "", "", "", false, 22)

	for _, field := range LinuxCollectionOSFields() {
		_, ok := testLC.guestRuleCommandMap[field]
		if ok {
			guestCollectorLinuxCount++
		}
	}

	if guestCollectorLinuxCount != guestCollectorCount {
		t.Errorf("guestCollectorLinuxCount = %d, want %d", guestCollectorLinuxCount, guestCollectorCount)
	}
}

func TestForLinux(t *testing.T) {
	inputs := []struct {
		command func(string) (string, error)
		want    string
	}{
		{
			command: func(path string) (string, error) {
				return path, nil
			},
			want: "google-sda1",
		},
		{
			command: func(path string) (string, error) {
				return "", nil
			},
			want: "",
		},
		{
			command: func(path string) (string, error) {
				return path + "\n", nil
			},
			want: "google-sda1",
		},
	}
	defer func(f func(path string) (string, error)) { symLinkCommand = f }(symLinkCommand)
	for _, input := range inputs {
		t.Run("forLinux", func(t *testing.T) {
			symLinkCommand = input.command
			want := input.want
			got, err := forLinux("sda1")
			if err != nil {
				t.Errorf("forLinux(\"sda1\") returned unexpected error: %v", err)
			}
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("forLinux(\"sda1\") returned unexpected diff (-got +want):\n%s", diff)
			}
		})
	}
}

func TestForLinuxError(t *testing.T) {
	defer func(f func(path string) (string, error)) { symLinkCommand = f }(symLinkCommand)
	symLinkCommand = func(path string) (string, error) {
		return "", errors.New("test error")
	}

	if _, err := forLinux("sda1"); err == nil {
		t.Errorf("forLinux(\"sda1\") did not return an error")
	}
}

func TestFindLshwFields(t *testing.T) {
	testcases := []struct {
		name      string
		lshwInput string
		want      lshwEntry
	}{
		{
			name: "success with needed fields",
			lshwInput: fmt.Sprintf(`[
				{
					"logicalname" : "/dev/sda",
					"size" : 402653184000,
					"product" : "%s",
				}
			]`, ephemeralDisk),
			want: lshwEntry{Product: ephemeralDisk, Size: 402653184000, LogicalName: "sda"},
		},
		{
			name: "success with jumbled input",
			lshwInput: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"testuselessfield" : 012,
				"size" : 402653184000,
				"size2" : "!2312",
				"anotheruseless" : "any output"
				"product" : "%s",
			}`, ephemeralDisk),
			want: lshwEntry{Product: ephemeralDisk, Size: 402653184000, LogicalName: "sda"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			test, err := c.findLshwFields(tc.lshwInput)
			if err != nil {
				t.Errorf("findLshwFields() returned error: %v", err)
			}
			if diff := cmp.Diff(test, tc.want); diff != "" {
				t.Errorf("findLshwFields() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestFindHwinfoFields(t *testing.T) {
	testcases := []struct {
		name      string
		lshwInput string
		want      lshwEntry
	}{
		{
			name: "success with needed fields",
			lshwInput: fmt.Sprintf(`
			Device: "%s"
			Device File: /dev/sda (/dev/sg0)
			Capacity: 64 GB (68719476736 bytes)
		`, persistentDisk),
			want: lshwEntry{Product: persistentDisk, Size: 68719476736, LogicalName: "sda"},
		},
		{
			name: "success with jumbled input",
			lshwInput: fmt.Sprintf(` Unique ID: R7kM.empSTHgeyZC
			Parent ID: UH3v.4Ex5C38ZXm7
			SysFS ID: /class/block/sda
			SysFS BusID: 0:0:1:0
			SysFS Device Link: /devices/pci0000:00/0000:00:03.0/virtio0/host0/target0:0:1/0:0:1:0
			Hardware Class: disk
			Model: "Google PersistentDisk"
			Vendor: "Google"
			Device: "%s"
			Revision: "1"
			Driver: "virtio_scsi", "sd"
			Driver Modules: "virtio_scsi", "sd_mod"
			Device File: /dev/sda (/dev/sg0)
			Device Files: /dev/sda, /dev/disk/by-path/pci-0000:00:03.0-scsi-0:0:1:0, /dev/disk/by-id/google-persistent-disk-0, /dev/disk/by-id/scsi-0Google_PersistentDisk_persistent-disk-0
			Device Number: block 8:0-8:15 (char 21:0)
			BIOS id: 0x80
			Geometry (Logical): CHS 8354/255/63
			Size: 134217728 sectors a 512 bytes
			Capacity: 64 GB (68719476736 bytes)
			Config Status: cfg=new, avail=yes, need=no, active=unknown
			Attached to: #11 (Unclassified device)`, persistentDisk),
			want: lshwEntry{Product: persistentDisk, Size: 68719476736, LogicalName: "sda"},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			test, err := c.findHwinfoFields(tc.lshwInput)
			if err != nil {
				t.Errorf("findHwinfoFields() returned error: %v", err)
			}
			if diff := cmp.Diff(test, tc.want); diff != "" {
				t.Errorf("findHwinfoFields() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestFindHwinfoFields_BadInput(t *testing.T) {
	testcases := []struct {
		name      string
		lshwInput string
	}{
		{
			name:      "logical name failed",
			lshwInput: "",
		},
		{
			name: "product failed",
			lshwInput: `
			Device File: /dev/sda (/dev/sg0)
			Capacity: 64 GB (68719476736 bytes)`,
		},
		{
			name: "size failed",
			lshwInput: fmt.Sprintf(`
			Device: "%s"
			Device File: /dev/sda (/dev/sg0)
		`, persistentDisk),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			_, err := c.findHwinfoFields(tc.lshwInput)
			if err == nil {
				t.Errorf("findHwinfoFields() returned nil error, want error")
			}
		})
	}
}

func TestFindLshwField_BadInput(t *testing.T) {
	testcases := []struct {
		name      string
		lshwInput string
	}{
		{
			name:      "logical name failed",
			lshwInput: "",
		},
		{
			name: "product failed",
			lshwInput: `{
				"logicalname" : "/dev/sda",
			} `,
		},
		{
			name: "size failed",
			lshwInput: `{
				"logicalname" : "/dev/sda",
				"product" : "any product",
			} `,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			_, err := c.findLshwFields(tc.lshwInput)
			if err == nil {
				t.Errorf("findLshwFields() returned nil error, want error")
			}
		})
	}
}

func TestFindLshwFieldString(t *testing.T) {
	tests := []struct {
		name       string
		lshwResult string
		field      string
		want       string
	}{
		{
			name: "success logical name",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"size" : 402653184000,
				"product" : "%s"
			}`, ephemeralDisk),
			field: "logicalname",
			want:  "sda",
		},
		{
			name: "success product",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"size" : 402653184000,
				"product" : "%s"
			}`, ephemeralDisk),
			field: "product",
			want:  ephemeralDisk,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			got, err := c.findLshwFieldString(tc.lshwResult, tc.field)
			if err != nil {
				t.Errorf("findLshwFieldString(%v, %v) returned an unexpected error: %v", tc.lshwResult, tc.field, err)
			}
			if got != tc.want {
				t.Errorf("findLshwFieldString(%v, %v) = %v, want: %v", tc.lshwResult, tc.field, got, tc.want)
			}
		})
	}
}

func TestFindLshwFieldString_BadInput(t *testing.T) {
	tests := []struct {
		name       string
		lshwResult string
		field      string
		want       string
	}{
		{
			name: "could not find product field",
			lshwResult: `{
				"logicalname" : "/dev/sda",
				"size" : 123
			}`,
			field: "product",
		},
		{
			name: "incorrect product field type",
			lshwResult: `{
				"logicalname" : "/dev/sda",
				"product" : 123,
				"size" : 123
			}`,
			field: "product",
		},
	}

	for _, tc := range tests {
		t.Run(tc.lshwResult, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			_, err := c.findLshwFieldString(tc.lshwResult, tc.field)
			if err == nil {
				t.Errorf("findLshwFieldString(%v, %v) returned an unexpected error: %v", tc.lshwResult, tc.field, err)
			}
		})
	}
}

func TestFindLshwFieldInt(t *testing.T) {
	tests := []struct {
		name       string
		lshwResult string
		field      string
		want       int
	}{
		{
			name: "success with size randomly in json file",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"size" : 402653184000,
				"product" : "%s"
			}`, ephemeralDisk),
			field: "size",
			want:  402653184000,
		},
		{
			name: "success with size at the end of json file",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"product" : "%s"
				"size" : 402653184000
			}`, ephemeralDisk),
			field: "size",
			want:  402653184000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			got, err := c.findLshwFieldInt(tc.lshwResult, tc.field)
			if err != nil {
				t.Errorf("findLshwFieldInt(%v, %v) returned an unexpected error: %v", tc.lshwResult, tc.field, err)
			}
			if got != tc.want {
				t.Errorf("findLshwFieldInt(%v, %v) = %v, want: %v", tc.lshwResult, tc.field, got, tc.want)
			}
		})
	}
}

func TestFindLshwFieldInt_BadInput(t *testing.T) {
	tests := []struct {
		name       string
		lshwResult string
		field      string
	}{
		{
			name: "could not find field size",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"product" : "%s"
			}`, ephemeralDisk),
			field: "size",
		},
		{
			name: "size was not an int",
			lshwResult: fmt.Sprintf(`{
				"logicalname" : "/dev/sda",
				"size" : "402653184000"
				"product" : "%s"
			}`, ephemeralDisk),
			field: "size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewLinuxCollector(nil, "", "", "", true, 22)
			_, err := c.findLshwFieldInt(tc.lshwResult, tc.field)
			if err == nil {
				t.Errorf("findLshwFieldInt(%v, %v) returned an unexpected error: %v", tc.lshwResult, tc.field, err)
			}
		})
	}
}

func TestFindPowerProfile(t *testing.T) {
	tests := []struct {
		name             string
		powerProfileFull string
		want             string
	}{
		{
			name:             "success: normal vm power profile",
			powerProfileFull: "Current active profile: virtual-guest",
			want:             "virtual-guest",
		},
		{
			name:             "success: mssql (High performance) power profile",
			powerProfileFull: "Current active profile: mssql",
			want:             "High performance",
		},
		{
			name:             "success: throughput-performance (High performance) power profile",
			powerProfileFull: "Current active profile: throughput-performance",
			want:             "High performance",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findPowerProfile(tc.powerProfileFull)
			if err != nil {
				t.Errorf("findPowerProfile(%v) returned an unexpected error: %v", tc.powerProfileFull, err)
			}
			if got != tc.want {
				t.Errorf("findPowerProfile(%v) = %v, want: %v", tc.powerProfileFull, got, tc.want)
			}
		})
	}
}

func TestFindPowerProfile_BadInput(t *testing.T) {
	tests := []struct {
		name             string
		powerProfileFull string
	}{
		{
			name:             "empty power profile",
			powerProfileFull: "",
		},
		{
			name:             "invalid power profile",
			powerProfileFull: "any input without correct format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := findPowerProfile(tc.powerProfileFull)
			if err == nil {
				t.Errorf("findPowerProfile(%v) returned nil error, want error", tc.powerProfileFull)
			}
		})
	}
}

func TestCheckLinuxOSCollectedMetrics(t *testing.T) {
	tests := []struct {
		name  string
		input []internal.Details
		want  []internal.Details
	}{
		{
			name: "success for empty input",
			input: []internal.Details{
				internal.Details{Name: "OS"},
			},
			want: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule:     "unknown",
							internal.LocalSSDRule:                "unknown",
							internal.DataDiskAllocationUnitsRule: "unknown",
						},
					},
				},
			},
		},
		{
			name: "success for half collected input",
			input: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule: "test",
						},
					},
				},
			},
			want: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule:     "test",
							internal.LocalSSDRule:                "unknown",
							internal.DataDiskAllocationUnitsRule: "unknown",
						},
					},
				},
			},
		},
		{
			name: "success with additional field",
			input: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							"testing": "any output",
						},
					},
				},
			},
			want: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule:     "unknown",
							internal.LocalSSDRule:                "unknown",
							internal.DataDiskAllocationUnitsRule: "unknown",
							"testing":                            "any output",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testLC := NewLinuxCollector(nil, "", "", "", false, 22)
			err := testLC.MarkUnknownOSFields(&tc.input)
			if err != nil {
				t.Fatalf("TestCheckOSCollectedMetrics(%q) unexpected error: %v", tc.input, err)
			}
			if diff := cmp.Diff(tc.input, tc.want); diff != "" {
				t.Errorf("TestCheckOSCollectedMetrics(%q) returned diff (-got +want):\n%s", tc.input, diff)
			}
		})
	}
}

func TestCheckLinuxOSCollectedMetrics_BadInput(t *testing.T) {
	tests := []struct {
		name  string
		input []internal.Details
	}{
		{
			name: "fail for incorrect OS name",
			input: []internal.Details{
				internal.Details{
					Name: "NOT OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule: "any output",
							internal.LocalSSDRule:            "any output",
						},
					},
				},
			},
		},
		{
			name: "fail for too many details",
			input: []internal.Details{
				internal.Details{
					Name:   "OS",
					Fields: []map[string]string{},
				},
				internal.Details{
					Name:   "OS",
					Fields: []map[string]string{},
				},
			},
		},
		{
			name:  "fail for no details",
			input: []internal.Details{},
		},
		{
			name: "fail for too many fields",
			input: []internal.Details{
				internal.Details{
					Name: "OS",
					Fields: []map[string]string{
						map[string]string{
							internal.PowerProfileSettingRule:     "any output",
							internal.LocalSSDRule:                "any output",
							internal.DataDiskAllocationUnitsRule: "any output",
						},
						map[string]string{},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testLC := NewLinuxCollector(nil, "", "", "", false, 22)
			err := testLC.MarkUnknownOSFields(&tc.input)
			if err == nil {
				t.Fatalf("TestCheckOSCollectedMetrics(%q) expected error", tc.input)
			}
		})
	}
}
