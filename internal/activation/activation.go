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

// Package activation contains functionalities for activating sql server client.
package activation

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
)

// Status defines new type indicating agent status.
type Status int

// AgentStatus interface.
type AgentStatus interface {
	Activate(s wlm.WorkloadManagerService, path, name, projectID, instance, instancID string) (bool, error)
	IsAgentActive(path string) bool
}

const (
	// Installed indicates the agent is in Installed status.
	Installed Status = iota
	// Activated indicates the agent is in Activated status.
	Activated
)

// V1 is the agent current status. Default status is Installed.
type V1 struct {
	Status Status
}

// NewV1 returns AgentStatus with default status "Installed".
func NewV1() *V1 {
	return &V1{
		Status: Installed,
	}
}

// Activate the agent.
// Return true if the activation succeed. Also returns true with error if file persistence failed.
// Otherwise return false.
func (a *V1) Activate(s wlm.WorkloadManagerService, path, name, projectID, instance, instancID string) (bool, error) {
	// Server returns either 201 or 202 for a valid request.
	// 201: Agent is activated.
	// 202: Agent activation failed.
	// Other http code will result in an non-nil error returned.
	request := wlm.InitializeWriteInsightRequest(wlm.InitializeSQLServerValidation(projectID, instance), instancID)
	s.UpdateRequest(request)
	response, err := s.SendRequest(name)
	if err != nil {
		return false, fmt.Errorf("Activate() failed due to SendRequest(%s) failure: %w", name, err)
	}

	if response.HTTPStatusCode == 201 {
		a.Status = Activated
		return true, internal.SaveToFile(path, []byte(""))
	}
	return false, fmt.Errorf("activating agent failed with result code %v", response.HTTPStatusCode)
}

// IsAgentActive returns the agent activation status.
// Return true if the agent is activated. False if the agent is inactive.
func (a *V1) IsAgentActive(path string) bool {
	if a.Status == Activated {
		return true
	}

	if _, err := os.Stat(path); err == nil {
		a.Status = Activated
		return true
	}

	return false
}
