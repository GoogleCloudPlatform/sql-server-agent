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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

func TestInitializeLinuxOSRulesCount(t *testing.T) {
	testcases := []struct {
		name            string
		linuxRulesArr   []string
		guestCommandMap map[string]commandExecutor
	}{
		{
			name: "Initialize Linux OS Rules",
			linuxRulesArr: []string{
				internal.PowerProfileSettingRule,
				internal.LocalSSDRule,
				internal.DataDiskAllocationUnitsRule,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.linuxRulesArr, LinuxCollectionOSFields()); diff != "" {
				t.Errorf("IniatializeLinuxOSRules() returned mismatching collected OS fields (-got +want):\n%s", diff)
			}
		})
	}
}

func TestInitializeLinuxOSIsRule(t *testing.T) {
	testcases := []struct {
		name            string
		guestCommandMap map[string]commandExecutor
	}{
		{
			name: internal.PowerProfileSettingRule,
			guestCommandMap: map[string]commandExecutor{
				internal.PowerProfileSettingRule: commandExecutor{
					isRule: true,
				},
			},
		},
		{
			name: internal.DataDiskAllocationUnitsRule,
			guestCommandMap: map[string]commandExecutor{
				internal.DataDiskAllocationUnitsRule: commandExecutor{
					isRule: true,
				},
			},
		},
		{
			name: internal.LocalSSDRule,
			guestCommandMap: map[string]commandExecutor{
				internal.LocalSSDRule: commandExecutor{
					isRule: false,
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewLinuxCollector(nil, "", "", "", false, 22)
			if diff := cmp.Diff(collector.guestRuleCommandMap[tc.name].isRule, tc.guestCommandMap[tc.name].isRule); diff != "" {
				t.Errorf("IniatializeLinuxOSRules() returned mismatching collected OS fields (-got +want):\n%s", diff)
			}
			if collector.guestRuleCommandMap[tc.name].runCommand == nil {
				t.Errorf("IniatializeLinuxOSRules() returned nil run command")
			}
			if collector.guestRuleCommandMap[tc.name].runRemoteCommand == nil {
				t.Errorf("IniatializeLinuxOSRules() returned nil run remote command")
			}
		})
	}
}
