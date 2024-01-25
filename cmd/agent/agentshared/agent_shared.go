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

// Package agentshared contains functions that used by the package sqlserveragent.
package agentshared

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap/zapcore"
	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/activation"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/guestcollector"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/sqlcollector"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
)

// CheckAgentStatus checks agent status. Return error if it failed to activate.
func CheckAgentStatus(agentStatus activation.AgentStatus, wlmService wlm.WorkloadManagerService, fp string, name, projectID, instance, instanceID string) error {
	if !agentStatus.IsAgentActive(fp) {
		log.Logger.Info("Agent is not active. Activating the agent.")
		isActive, err := agentStatus.Activate(wlmService, fp, name, projectID, instance, instanceID)
		if isActive {
			log.Logger.Info("Agent is activated.")
			if err != nil {
				log.Logger.Warnw("An error occured during the agent activation", "error", err)
			}
		} else {
			return fmt.Errorf("Activation failed. Error: %v", err)
		}
	}
	return nil
}

// LoggingSetup sets the log parameters.
func LoggingSetup(ctx context.Context, prefix, level, projectID string, cloudLogging bool) {
	lp := log.Parameters{
		LogFileName:        prefix + ".log",
		LogToCloud:         cloudLogging,
		CloudLogName:       "google-cloud-sql-server-agent",
		CloudLoggingClient: log.CloudLoggingClient(ctx, projectID),
	}
	logLevel := map[string]zapcore.Level{
		"DEBUG":   zapcore.DebugLevel,
		"INFO":    zapcore.InfoLevel,
		"WARNING": zapcore.WarnLevel,
		"ERROR":   zapcore.ErrorLevel,
	}
	if _, ok := logLevel[level]; !ok {
		lp.Level = zapcore.InfoLevel
	} else {
		lp.Level = logLevel[level]
	}
	log.SetupLogging(lp)
}

// LoggingSetupDefault sets the logging with default parameters.
// Default level will be INFO.
func LoggingSetupDefault(ctx context.Context, prefix string) {
	lp := log.Parameters{
		LogFileName:  prefix + ".log",
		Level:        zapcore.InfoLevel,
		CloudLogName: "google-cloud-sql-server-agent",
	}
	log.SetupLogging(lp)
}

// RunSQLCollection runs sql collection based on given conn string.
func RunSQLCollection(ctx context.Context, c sqlcollector.SQLCollector, timeout time.Duration) []internal.Details {
	// Start db collection.
	log.Logger.Debug("Collecting SQL Server rules.")
	details := c.CollectMasterRules(ctx, timeout)
	log.Logger.Debug("Collecting SQL Server rules completes.")
	return details
}

// RunOSCollection runs guest collection based on given collector type.
// GuestCollector could be either for Linux or for Windows.
func RunOSCollection(ctx context.Context, c guestcollector.GuestCollector, timeout time.Duration) []internal.Details {
	details := []internal.Details{}
	log.Logger.Debug("Collecting guest rules")
	details = append(details, c.CollectGuestRules(ctx, timeout))
	err := guestcollector.MarkUnknownOsFields(&details)
	if err != nil {
		log.Logger.Warnf("RunOSCollection: Failed to mark unknown collected fields. error: %v", err)
	}

	log.Logger.Debug("Collecting guest rules completes")
	return details
}

// AddPhysicalDriveLocal adds physical drive to sql collection based off details for local instances
func AddPhysicalDriveLocal(ctx context.Context, details []internal.Details, windows bool) {
	for _, detail := range details {
		if detail.Name != "DB_LOG_DISK_SEPARATION" {
			continue
		}
		for _, field := range detail.Fields {
			physicalPath, pathExists := field["physical_name"]
			if !pathExists {
				log.Logger.Warn("physical_name field for DB_LOG_DISK_SEPERATION does not exist")
				continue
			}
			field["physical_drive"] = internal.GetPhysicalDriveFromPath(ctx, physicalPath, windows, commandlineexecutor.ExecuteCommand)
		}
	}
}
