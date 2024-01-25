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

package activation

import (
	"os"
	"path"
	"testing"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
)

func TestIsAgentActivated(t *testing.T) {
	testcases := []struct {
		name               string
		want               bool
		activatedFileExist bool
		currentAgentStatus Status
		wantAgentStatus    Status
	}{
		{
			name:               "returns true for active agent",
			want:               true,
			activatedFileExist: true,
			currentAgentStatus: Activated,
			wantAgentStatus:    Activated,
		},
		{
			name:               "returns true if active agent that restarts",
			want:               true,
			activatedFileExist: true,
			currentAgentStatus: Installed,
			wantAgentStatus:    Activated,
		},
		{
			name:               "returns false for inactive agent",
			want:               false,
			activatedFileExist: false,
			currentAgentStatus: Installed,
			wantAgentStatus:    Installed,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tempFilePath := path.Join(t.TempDir(), "google-cloud-sql-server-agent.activated")

			if tc.activatedFileExist {
				f, err := os.Create(tempFilePath)
				if err != nil {
					t.Fatal(err)
				}
				f.Close()

			}

			s := NewV1()
			s.Status = tc.currentAgentStatus

			if got := s.IsAgentActive(tempFilePath); got != tc.want {
				t.Errorf("IsAgentActivated() = %v, want %v", got, tc.want)
			}
			if s.Status != tc.wantAgentStatus {
				t.Errorf("AgentStatus = %v, want %v", s.Status, tc.wantAgentStatus)
			}
		})
	}
}

func TestActivate(t *testing.T) {
	testcases := []struct {
		name            string
		want            bool
		wantErr         bool
		wantAgentStatus Status
		mockHTTPCode    int
		mockWLMError    bool
		createFileError bool
	}{
		{
			name:            "activate agent successfully",
			want:            true,
			wantAgentStatus: Activated,
			mockHTTPCode:    201,
		},
		{
			name:            "activate returns true and err if failed to persist",
			want:            true,
			wantErr:         true,
			wantAgentStatus: Activated,
			mockHTTPCode:    201,
			createFileError: true,
		},
		{
			name:            "activate fails and it returns false and err",
			want:            false,
			wantErr:         true,
			wantAgentStatus: Installed,
			mockHTTPCode:    202,
		},
		{
			name:            "unexpected error",
			want:            false,
			wantErr:         true,
			wantAgentStatus: Installed,
			mockHTTPCode:    404,
			mockWLMError:    true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tempFilePath := path.Join(t.TempDir(), "google-cloud-sql-server-agent.activated")

			if tc.createFileError {
				tempFilePath = ""
			}

			s := NewV1()
			svc := &wlm.MockWlmService{
				MockHTTPCode: tc.mockHTTPCode,
				MockError:    tc.mockWLMError,
			}

			got, err := s.Activate(svc, tempFilePath, "", "", "", "")
			if got != tc.want {
				t.Errorf("Activate() = %v, want %v", got, tc.want)
			}
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("Activate() = %v, want error presence = %v", err, tc.wantErr)
			}
			if s.Status != tc.wantAgentStatus {
				t.Errorf("AgentStatus = %v, want %v", s.Status, tc.wantAgentStatus)
			}
		})
	}
}

func TestFakeActivate(t *testing.T) {
	testcases := []struct {
		name               string
		want               bool
		wantErr            bool
		mockActivateError  bool
		mockActivateResult bool
	}{
		{
			name:               "success with nil error",
			want:               true,
			mockActivateResult: true,
		},
		{
			name:               "success with error",
			want:               true,
			wantErr:            true,
			mockActivateError:  true,
			mockActivateResult: true,
		},
		{
			name: "failure",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockAgentStatue := MockAgentStatus{
				MockActivateError:  tc.mockActivateError,
				MockActivateResult: tc.mockActivateResult,
			}

			got, err := mockAgentStatue.Activate(&wlm.MockWlmService{}, "", "", "", "", "")

			if got != tc.want {
				t.Errorf("Activate() = %v, want %v", got, tc.want)
			}

			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("Activate()= %v, want error presence = %v", err, tc.wantErr)
			}
		})
	}
}
