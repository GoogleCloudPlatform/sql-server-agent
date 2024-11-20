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

// Package flags defines the flags in the command line.
package flags

import (
	"fmt"

	"flag"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

// AgentFlags .
type AgentFlags struct {
	Action        string
	Onetime       bool
	Address       string
	Protocol      string
	errorLogFile  string
	logName       string
	logStatus     string
	logVersion    string
	version       bool
	help          bool
	h             bool
	projectID     string
	zone          string
	instance      string
	projectNumber string
	image         string
}

// NewAgentFlags initialize flags and return the reference of struct agentFlags.
func NewAgentFlags(projectID, zone, instance, projectNumber, image string) *AgentFlags {
	action := flag.String("action", "", "Action for running the agent.")
	onetime := flag.Bool("onetime", false, "Onetime mode for the agent.")
	version := flag.Bool("agent_version", false, "Display the version of the agent.")
	help := flag.Bool("help", false, "Display the usage of each flag.")
	h := flag.Bool("h", false, "Display the usage of each flag.")
	// protocol, address and errorlogfile are used by guest agent.
	protocol := flag.String("protocol", "", "protocol to use uds/tcp")
	address := flag.String("address", "", "address to start server listening on")
	errorLogfile := flag.String("errorlogfile", "", "file to write error logs to")
	logStatus := flag.String("logstatus", "", "log usage status")
	logVersion := flag.String("logversion", "", "log usage version")
	logName := flag.String("logname", "", "name of the log type")

	if !flag.Parsed() {
		flag.Parse()
	}

	return &AgentFlags{
		Action:        *action,
		Onetime:       *onetime,
		Address:       *address,
		Protocol:      *protocol,
		errorLogFile:  *errorLogfile,
		version:       *version,
		help:          *help,
		h:             *h,
		logStatus:     *logStatus,
		logVersion:    *logVersion,
		logName:       *logName,
		projectID:     projectID,
		zone:          zone,
		instance:      instance,
		projectNumber: projectNumber,
		image:         image,
	}
}

// Execute based on the flag values.
// Return false if the caller needs to stop running.
// Otherwise return true.
func (af *AgentFlags) Execute() (string, bool) {
	if af.help || af.h {
		return af.usage(), false
	}
	if af.version {
		return fmt.Sprintf("Google Cloud SQL Server Agent version: %v.", internal.AgentVersion), false
	}
	if af.logStatus != "" {
		return af.status()
	}
	if af.Onetime {
		return "", true
	}
	// TODO - LCM integration.
	if af.Action == "" {
		return af.usage(), false
	}
	return "", true
}

func (af *AgentFlags) usage() string {
	return `Usage: google-cloud-sql-server-agent -(h|agent_version|onetime)`
}

func (af *AgentFlags) status() (string, bool) {
	if af.logName == "" {
		return "Please specify the name of the log -logname.", false
	}
	if af.logVersion == "" {
		return "Please specify the version of the log -logversion.", false
	}
	ap := agentstatus.NewAgentProperties(af.logName, af.logVersion, internal.AgentUsageLogPrefix, true)
	cp := agentstatus.NewCloudProperties(af.projectID, af.zone, af.instance, af.projectNumber, af.image)
	agentstatus.NewUsageMetricsLogger(ap, cp, []string{}).LogStatus(agentstatus.Status(af.logStatus), "")
	return "", false
}
