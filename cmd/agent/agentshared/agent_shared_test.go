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

package agentshared

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/activation"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
)

type mockSQLCollector struct{}

func (c *mockSQLCollector) CollectMasterRules(ctx context.Context, timeout time.Duration) []internal.Details {
	return []internal.Details{
		{
			Name: "mockResult",
			Fields: []map[string]string{
				{
					"mockField": "mockValue",
				},
			},
		},
	}
}

type mockGuestOsCollector struct{}

func (c *mockGuestOsCollector) CollectGuestRules(ctx context.Context, timeout time.Duration) internal.Details {
	return internal.Details{
		Name: "mockResult",
		Fields: []map[string]string{
			{
				"mockField": "mockValue",
			},
		},
	}
}

func (c *mockGuestOsCollector) MarkUnknownOSFields(details *[]internal.Details) error {
	return nil
}

func TestCheckAgentStatus(t *testing.T) {
	testcases := []struct {
		name        string
		agentStatus activation.AgentStatus
		wantErr     bool
	}{
		{
			name: "success",
			agentStatus: &activation.MockAgentStatus{
				MockIsAgentActiveReturnValue: true,
			},
		},
		{
			name: "first time successful activation",
			agentStatus: &activation.MockAgentStatus{
				MockActivateResult: true,
			},
		},
		{
			name: "success with activated file creation error",
			agentStatus: &activation.MockAgentStatus{
				MockActivateError:  true,
				MockActivateResult: true,
			},
		},
		{
			name: "return error for failed activation",
			agentStatus: &activation.MockAgentStatus{
				MockActivateError: true,
			},
			wantErr: true,
		},
	}

	fp := "testFilepath"
	mockWlmService := &wlm.MockWlmService{}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckAgentStatus(tc.agentStatus, mockWlmService, fp, "testLocation", "testInstanceID", "testProjectID", "testInstance")
			if gotErr := got != nil; gotErr != tc.wantErr {
				t.Errorf("CheckAgentStatus()=%v, want error presence = %v", got, tc.wantErr)
			}
		})
	}
}

func TestLoggingSetup(t *testing.T) {
	testcases := []struct {
		name           string
		configLogLevel string
		want           string
	}{
		{
			name:           "success",
			configLogLevel: "WARNING",
			want:           "warn",
		},
		{
			name:           "log level set to info if configLogLevel is invalid",
			configLogLevel: "any",
			want:           "info",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			LoggingSetup(context.Background(), "any", tc.configLogLevel, "", false)
			if log.GetLevel() != tc.want {
				t.Errorf("LoggingSetup(any, %v) sets logger in the wrong level. want %v, got %v", tc.configLogLevel, tc.want, log.GetLevel())
			}
		})
	}
}

func TestLoggingSetupDefault(t *testing.T) {
	testcases := []struct {
		name string
		want string
	}{
		{
			name: "success",
			want: "info",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			LoggingSetupDefault(context.Background(), "any")
			if got := log.GetLevel(); got != tc.want {
				t.Errorf("LoggingSetup(any, %v) sets logger in the wrong level. want INFO, got %v", tc.want, got)
			}
		})
	}
}

func TestRunSQLCollection(t *testing.T) {
	got := RunSQLCollection(context.Background(), &mockSQLCollector{}, time.Second)
	want := []internal.Details{
		{
			Name: "mockResult",
			Fields: []map[string]string{
				{
					"mockField": "mockValue",
				},
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("runSQLCollection() returned wrong result (-got +want):\n%s", diff)
	}
}

func TestRunOSCollection(t *testing.T) {
	got := RunOSCollection(context.Background(), &mockGuestOsCollector{}, time.Second)
	want := []internal.Details{
		{
			Name: "mockResult",
			Fields: []map[string]string{
				{
					"mockField": "mockValue",
				},
			},
		},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("RunOSCollection() returned wrong result (-got +want):\n%s", diff)
	}
}

func TestAddPhysicalDriveLocal(t *testing.T) {
	testcases := []struct {
		name    string
		details []internal.Details
		windows bool
		want    []internal.Details
	}{
		{
			name: "testing windows physical drive",
			details: []internal.Details{
				{
					Name: "DB_LOG_DISK_SEPARATION",
					Fields: []map[string]string{
						{
							"physical_name": "C:\\test\\testdrive",
						},
					},
				},
				{
					Name: "fakeName",
					Fields: []map[string]string{
						{
							"no_physical_name": "C:\\test\\testdrive",
							"testing":          "randomTest",
						},
					},
				},
			},
			want: []internal.Details{
				{
					Name: "DB_LOG_DISK_SEPARATION",
					Fields: []map[string]string{
						{
							"physical_name":  "C:\\test\\testdrive",
							"physical_drive": "C",
						},
					},
				},
				{
					Name: "fakeName",
					Fields: []map[string]string{
						{
							"no_physical_name": "C:\\test\\testdrive",
							"testing":          "randomTest",
						},
					},
				},
			},
			windows: true,
		},
	}

	ctx := context.Background()

	for _, tc := range testcases {

		AddPhysicalDriveLocal(ctx, tc.details, tc.windows)
		if diff := cmp.Diff(tc.details, tc.want); diff != "" {
			t.Errorf("AddPhysicalDriveLocal() returned wrong result (-got +want):\n%s", diff)
		}
	}
}
