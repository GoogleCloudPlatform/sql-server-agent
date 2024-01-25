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
	"fmt"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
)

// MockAgentStatus mocks agentstatus for testing usage.
type MockAgentStatus struct {
	MockActivateError            bool
	MockIsAgentActiveReturnValue bool
	MockActivateResult           bool
}

// Activate mock function.
func (m *MockAgentStatus) Activate(s wlm.WorkloadManagerService, path, name, projectID, instance, instancID string) (bool, error) {
	var err error
	if m.MockActivateError {
		err = fmt.Errorf("any error")
	}
	return m.MockActivateResult, err
}

// IsAgentActive mock function.
func (m *MockAgentStatus) IsAgentActive(path string) bool { return m.MockIsAgentActiveReturnValue }
