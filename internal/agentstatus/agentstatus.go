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

// Package agentstatus provides functions that log SQL Server Agent status.
package agentstatus

import (
	"github.com/jonboulle/clockwork"
	"github.com/GoogleCloudPlatform/sapagent/shared/usagemetrics"
)

// AgentStatus interface.
type AgentStatus interface {
	// Installed logs the agent status Installed.
	Installed()
	// Started logs the agent status Started.
	Started()
	// Configured logs the agent status Configured.
	Configured()
	// Misconfigured logs the agent status Misconfigured.
	Misconfigured()
	// Updated logs the agent status Updated.
	Updated(version string)
	// Running logs the agent status Running.
	Running()
	// Stopped logs the agent status Stopped.
	Stopped()
	// Action logs the agent status Action.
	Action(id int)
	// Error logs the agent status Error.
	Error(id int)
	// Uninstalled logs the agent status Uninstalled.
	Uninstalled()
	// LogStatus logs the agent status.
	LogStatus(status usagemetrics.Status, v string)
}

// Agent wide error code mappings.
// We need to maintain the error code list at go/sqlserver-agent-error-codes.
const (
	UnknownError = iota
	SQLCollectionFailure
	GuestCollectionFailure
	ReadConfigurationsFileError
	InvalidConfigurationsError
	SecretValueError
	SQLQueryExecutionError
	WMIQueryExecutionError
	MissingComputeViewerIAMRoleError
	InvalidJSONFormatError
	ProtoJSONUnmarshalError
	ParseKnownHostsError
	SetupSSHKeysError
	SSHDialError
	CommandExecutionError
	RemoteCommandExecutionError
	DataTypeConversionError
	WorkloadManagerConnectionError
	WinGuestCollectionTimeout
	LinuxGuestCollectionTimeout
	MappingLocalLinuxDiskTypeTimeout
)

// NewUsageMetricsLogger wraps NewLogger function from usagemetrics package.
func NewUsageMetricsLogger(agentProps *usagemetrics.AgentProperties, cloudProps *usagemetrics.CloudProperties, projectExclusions []string) *usagemetrics.Logger {
	return usagemetrics.NewLogger(agentProps, cloudProps, clockwork.NewRealClock(), projectExclusions)
}

// NewAgentProperties returns the pointer of the new instance usagemetrics.AgentProperties.
func NewAgentProperties(name, version, prefix string, logUsageMetrics bool) *usagemetrics.AgentProperties {
	return &usagemetrics.AgentProperties{
		Name:            name,
		Version:         version,
		LogUsagePrefix:  prefix,
		LogUsageMetrics: logUsageMetrics,
	}
}

// NewCloudProperties returns the pointer of the new instance usagemetrics.CloudProperties.
func NewCloudProperties(projectID, zone, instanceName, projectNumber, image string) *usagemetrics.CloudProperties {
	return &usagemetrics.CloudProperties{
		ProjectID:     projectID,
		Zone:          zone,
		InstanceName:  instanceName,
		ProjectNumber: projectNumber,
		Image:         image,
	}
}

// Status returns the usagemetrics.Status.
func Status(status string) usagemetrics.Status {
	return usagemetrics.Status(status)
}
