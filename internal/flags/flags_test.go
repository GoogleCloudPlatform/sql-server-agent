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

package flags

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

func TestNewAgentFlags(t *testing.T) {
	af := NewAgentFlags("", "", "", "", "")
	if af.help != false {
		t.Errorf("NewAgentFlags() = %v, want %v", af.help, true)
	}
	if af.h != false {
		t.Errorf("NewAgentFlags() = %v, want %v", af.h, true)
	}
	if af.version != false {
		t.Errorf("NewAgentFlags() = %v, want %v", af.version, true)
	}
	if af.Onetime != false {
		t.Errorf("NewAgentFlags() = %v, want %v", af.Onetime, true)
	}
	if af.Action != "" {
		t.Errorf("NewAgentFlags() = %v, want %v", af.Action, "")
	}
}

func TestExecute(t *testing.T) {
	testcases := []struct {
		name     string
		af       *AgentFlags
		wantStr  string
		wantBool bool
	}{
		{
			name:     "flag --help is enabled",
			af:       &AgentFlags{help: true},
			wantStr:  `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`,
			wantBool: false,
		},
		{
			name:     "flag --h is enabled",
			af:       &AgentFlags{h: true},
			wantStr:  `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`,
			wantBool: false,
		},
		{
			name:     "flag --agent_version is enabled",
			af:       &AgentFlags{version: true},
			wantStr:  fmt.Sprintf("Google Cloud SQL Server Agent version: %v.", internal.AgentVersion),
			wantBool: false,
		},
		{
			name:     "flag --onetime is enabled",
			af:       &AgentFlags{Onetime: true},
			wantStr:  "",
			wantBool: true,
		},
		{
			name:     "flag --action is empty",
			af:       &AgentFlags{Action: ""},
			wantStr:  `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`,
			wantBool: false,
		},
		{
			name:     "flag --action has value",
			af:       &AgentFlags{Action: "run"},
			wantStr:  "",
			wantBool: true,
		},
		{
			name:     "having flag --h ignores other flags",
			af:       &AgentFlags{h: true, version: true},
			wantStr:  `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`,
			wantBool: false,
		},
		{
			name:     "having flag --help ignores other flags",
			af:       &AgentFlags{help: true, version: true},
			wantStr:  `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`,
			wantBool: false,
		},
		{
			name:     "having flag --logusage ignores other flags",
			af:       &AgentFlags{logStatus: "status", Onetime: true, logVersion: "version", logName: "name"},
			wantStr:  "",
			wantBool: false,
		},
		{
			name:     "having flag --logusage requires the non-empty value of flag --logname",
			af:       &AgentFlags{logStatus: "status", Onetime: true},
			wantStr:  "Please specify the name of the log -logname.",
			wantBool: false,
		},
		{
			name:     "having flag --logusage requires the non-empty value of flag --logversion",
			af:       &AgentFlags{logStatus: "status", Onetime: true, logName: "name"},
			wantStr:  "Please specify the version of the log -logversion.",
			wantBool: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			gotStr, gotBool := tc.af.Execute()
			if gotStr != tc.wantStr || gotBool != tc.wantBool {
				t.Errorf("Execute(%v) = %v, %v, want %v, %v", tc.af, gotStr, gotBool, tc.wantStr, tc.wantBool)
			}
		})
	}
}
