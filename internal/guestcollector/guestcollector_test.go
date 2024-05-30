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

	"github.com/jonboulle/clockwork"
	"github.com/google/go-cmp/cmp"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

var fakeCloudProperties = agentstatus.NewCloudProperties("testProjectID", "testZone", "testInstanceName", "testProjectNumber", "testImage")
var fakeAgentProperties = agentstatus.NewAgentProperties("testName", "testVersion", false)
var fakeUsageMetricsLogger = agentstatus.NewUsageMetricsLogger(fakeAgentProperties, fakeCloudProperties, clockwork.NewRealClock(), []string{})

func TestCheckOSCollectedMetrics(t *testing.T) {
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
							internal.GCBDRAgentRunning:           "unknown",
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
							internal.GCBDRAgentRunning:           "unknown",
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
							internal.GCBDRAgentRunning:           "unknown",
							"testing":                            "any output",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := MarkUnknownOsFields(&tc.input)
			if err != nil {
				t.Fatalf("TestCheckOSCollectedMetrics(%q) unexpected error: %v", tc.input, err)
			}
			if diff := cmp.Diff(tc.input, tc.want); diff != "" {
				t.Errorf("TestCheckOSCollectedMetrics(%q) returned diff (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestCheckOSCollectedMetrics_BadInput(t *testing.T) {
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
			err := MarkUnknownOsFields(&tc.input)
			if err == nil {
				t.Fatalf("TestCheckOSCollectedMetrics(%q) expected error", tc.input)
			}
		})
	}
}
