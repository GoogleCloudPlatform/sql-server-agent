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

// Package sqlservermetrics run SQL and OS collections and sends metrics to workload manager.
package sqlservermetrics

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"go.uber.org/zap/zapcore"
	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/shared/gce"
	"github.com/GoogleCloudPlatform/sapagent/shared/gce/metadataserver"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/activation"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/configuration"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/flags"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/guestcollector"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/instanceinfo"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/remote"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/secretmanager"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/sqlcollector"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/wlm"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

const (
	// ServiceName .
	ServiceName = internal.ServiceName
	// ServiceDisplayName .
	ServiceDisplayName = "Google Cloud Agent for SQL Server"
	// Description .
	Description = "Google Cloud Agent for SQL Server."
	// ExperimentalMode .
	ExperimentalMode = internal.ExperimentalMode
	// AgentUsageLogPrefix .
	AgentUsageLogPrefix = internal.AgentUsageLogPrefix
	// AgentVersion .
	AgentVersion = internal.AgentVersion
	driver       = "sqlserver"
	commandFind  = `sudo find %s -type f -iname "%s" -print`
	commandDf    = "sudo df --output=target %s | tail -n 1"
	commandMount = "mount | grep sd"
)

// CollectionType represents the enums of collection types.
type CollectionType int

const (
	// OS collection type.
	OS CollectionType = iota
	// SQL collection type.
	SQL
)

// InstanceProperties represents properties of instance.
type InstanceProperties struct {
	Name          string
	Instance      string
	InstanceID    string
	ProjectID     string
	ProjectNumber string
	Zone          string
	Image         string
}

// UsageMetricsLogger logs usage metrics.
var UsageMetricsLogger agentstatus.AgentStatus = UsageMetricsLoggerInit(internal.ServiceName, internal.AgentVersion, internal.AgentUsageLogPrefix, true)

// SIP is the source instance properties.
var SIP InstanceProperties = sourceInstanceProperties()

// Init parses flags and execute if certain flags are enabled.
func Init() (*flags.AgentFlags, string, bool) {
	f := flags.NewAgentFlags(SIP.ProjectID, SIP.Zone, SIP.Instance, SIP.ProjectNumber, SIP.Image)
	output, proceed := f.Execute()
	return f, output, proceed
}

// LoggingSetup initialize the agent logging level.
func LoggingSetup(ctx context.Context, logPrefix string, cfg *configpb.Configuration) {
	lp := log.Parameters{
		LogFileName:        logPrefix + ".log",
		LogToCloud:         cfg.GetLogToCloud(),
		CloudLogName:       "google-cloud-sql-server-agent",
		CloudLoggingClient: log.CloudLoggingClient(ctx, SIP.ProjectID),
	}
	logLevel := map[string]zapcore.Level{
		"DEBUG":   zapcore.DebugLevel,
		"INFO":    zapcore.InfoLevel,
		"WARNING": zapcore.WarnLevel,
		"ERROR":   zapcore.ErrorLevel,
	}
	if _, ok := logLevel[cfg.GetLogLevel()]; !ok {
		lp.Level = zapcore.InfoLevel
	} else {
		lp.Level = logLevel[cfg.GetLogLevel()]
	}
	log.SetupLogging(lp)
}

// LoggingSetupDefault wraps LoggingSetupDefault function from agent_shared.go.
func LoggingSetupDefault(ctx context.Context, prefix string) {
	lp := log.Parameters{
		LogFileName:  prefix + ".log",
		Level:        zapcore.InfoLevel,
		CloudLogName: "google-cloud-sql-server-agent",
	}
	log.SetupLogging(lp)
}

// UsageMetricsLoggerInit initializes and returns usage metrics logger.
func UsageMetricsLoggerInit(logName, logVersion, logPrefix string, logUsage bool) agentstatus.AgentStatus {
	ap := agentstatus.NewAgentProperties(logName, logVersion, logPrefix, logUsage)
	cp := agentstatus.NewCloudProperties(SIP.ProjectID, SIP.Zone, SIP.Instance, SIP.ProjectNumber, SIP.Image)
	return agentstatus.NewUsageMetricsLogger(ap, cp, []string{})
}

// LoadConfiguration loads configuration from given path.
func LoadConfiguration(path string) (*configpb.Configuration, error) {
	return configuration.LoadConfiguration(path)
}

// CollectionService runs the passed in collection as a service.
func CollectionService(p string, collection func(cfg *configpb.Configuration, onetime bool) error, collectionType CollectionType) {
	for {
		cfg, err := LoadConfiguration(p)
		if cfg == nil {
			log.Logger.Errorw("Failed to load configuration", "error", err)
			UsageMetricsLogger.Error(agentstatus.ProtoJSONUnmarshalError)
			time.Sleep(time.Duration(time.Hour))
			continue
		}
		// Init UsageMetricsLogger for each collection cycle.
		UsageMetricsLogger = UsageMetricsLoggerInit(internal.ServiceName, internal.AgentVersion, internal.AgentUsageLogPrefix, !cfg.GetDisableLogUsage())
		// Set onetime to false for running collection as service
		if err := collection(cfg, false); err != nil {
			log.Logger.Errorw("Failed to run collection", "collection type", collectionType, "error", err)
			if collectionType == OS {
				UsageMetricsLogger.Error(agentstatus.GuestCollectionFailure)
			} else {
				UsageMetricsLogger.Error(agentstatus.SQLCollectionFailure)
			}
			time.Sleep(time.Duration(time.Hour))
			continue
		}
		// Sleep for collection interval.
		if collectionType == OS {
			time.Sleep(time.Duration(cfg.GetCollectionConfiguration().GetGuestOsMetricsCollectionIntervalInSeconds()) * time.Second)
		} else if collectionType == SQL {
			time.Sleep(time.Duration(cfg.GetCollectionConfiguration().GetSqlMetricsCollectionIntervalInSeconds()) * time.Second)
		}
	}
}

// sourceInstanceProperties returns properties of the instance the agent is running on.
func sourceInstanceProperties() InstanceProperties {
	properties := metadataserver.CloudPropertiesWithRetry(backoff.NewConstantBackOff(30 * time.Second))
	location := string(properties.GetZone()[0:strings.LastIndex(properties.GetZone(), "-")])
	name := fmt.Sprintf("projects/%s/locations/%s", properties.GetProjectId(), location)
	return InstanceProperties{
		Name:          name,
		ProjectID:     properties.GetProjectId(),
		ProjectNumber: properties.GetNumericProjectId(),
		InstanceID:    properties.GetInstanceId(),
		Instance:      properties.GetInstanceName(),
		Zone:          properties.GetZone(),
		Image:         properties.GetImage(),
	}
}

// initCollection executes steps for initializing a collection.
// The func is called at the beginning of every guest and sql collection.
func initCollection(ctx context.Context) (*wlm.WLM, error) {
	wlm, err := wlm.NewWorkloadManager(ctx)
	if err != nil {
		return nil, err
	}
	return wlm, nil
}

// checkAgentStatus checks agent status. Return error if it failed to activate.
func checkAgentStatus(wlm wlm.WorkloadManagerService, path string) error {
	agentStatus := activation.NewV1()
	fp := filepath.Join(filepath.Dir(path), "google-cloud-sql-server-agent.activated")
	if !agentStatus.IsAgentActive(fp) {
		log.Logger.Info("Agent is not active. Activating the agent.")
		isActive, err := agentStatus.Activate(wlm, fp, SIP.Name, SIP.ProjectID, SIP.Instance, SIP.InstanceID)
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

// validateCredCfgSQL wraps ValidateCredCfgSQL from configuration package.
func validateCredCfgSQL(remote, windows bool, sqlCfg *configuration.SQLConfig, guestCfg *configuration.GuestConfig, instanceID, instanceName string) error {
	return configuration.ValidateCredCfgSQL(remote, windows, sqlCfg, guestCfg, instanceID, instanceName)
}

// validateCredCfgGuest wraps ValidateCredCfgGuest from configuration package.
func validateCredCfgGuest(remote, windows bool, guestCfg *configuration.GuestConfig, instanceID, instanceName string) error {
	return configuration.ValidateCredCfgGuest(remote, windows, guestCfg, instanceID, instanceName)
}

// runSQLCollection starts running sql collection based on given connection string.
func runSQLCollection(ctx context.Context, conn string, timeout time.Duration, windows bool) ([]internal.Details, error) {
	c, err := sqlcollector.NewV1(driver, conn, windows, UsageMetricsLogger)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	// Start db collection.
	log.Logger.Debug("Collecting SQL Server rules.")
	details := c.CollectMasterRules(ctx, timeout)
	log.Logger.Debug("Collecting SQL Server rules completes.")
	return details, nil
}

// runOSCollection starts running os collection.
func runOSCollection(ctx context.Context, c guestcollector.GuestCollector, timeout time.Duration) []internal.Details {
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

// secretValue gets secret value from Secret Manager.
func secretValue(ctx context.Context, projectID string, secretName string) (string, error) {
	log.Logger.Debug("Getting secret.")
	smClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer smClient.Close()
	pswd, err := smClient.GetSecretValue(ctx, projectID, secretName)
	if err != nil {
		return "", err
	}
	log.Logger.Debug("Getting secret completes.")
	return pswd, nil
}

// allDisks attempts to call compute api to return all possible disks.
func allDisks(ctx context.Context, ip InstanceProperties) ([]*instanceinfo.Disks, error) {
	tempGCE, err := gce.NewGCEClient(ctx)
	if err != nil {
		return nil, err
	}

	r := instanceinfo.New(tempGCE)
	return r.AllDisks(ctx, ip.ProjectID, ip.Zone, ip.InstanceID)
}

// updateCollectedData constructs writeinsightrequest from given collected details.
// The func will be called by both guest and sql collections.
func updateCollectedData(wlmService wlm.WorkloadManagerService, sourceProps, targetProps InstanceProperties, details []internal.Details) {
	sqlservervalidation := wlm.InitializeSQLServerValidation(sourceProps.ProjectID, targetProps.Instance)
	sqlservervalidation = wlm.UpdateValidationDetails(sqlservervalidation, details)
	writeInsightRequest := wlm.InitializeWriteInsightRequest(sqlservervalidation, targetProps.InstanceID)
	writeInsightRequest.Insight.SentTime = time.Now().Format(time.RFC3339)
	// update wlmService.Request to writeInsightRequest
	wlmService.UpdateRequest(writeInsightRequest)
}

// sendRequestToWLM sends request to workloadmanager.
func sendRequestToWLM(wlmService wlm.WorkloadManagerService, location string, retries int32, interval time.Duration) {
	sendRequest := func() bool {
		_, err := wlmService.SendRequest(location)
		if err != nil {
			log.Logger.Errorw("Failed to send request to workload manager", "error", err)
			UsageMetricsLogger.Error(agentstatus.WorkloadManagerConnectionError)
			return false
		}
		return true
	}

	if err := retry(sendRequest, retries, interval); err != nil {
		log.Logger.Errorw("Failed to retry sending request to workload manager", "error", err)
		UsageMetricsLogger.Error(agentstatus.WorkloadManagerConnectionError)
	}
}

// persistCollectedData persists collected data in the file system.
// The file name follows the format "[target]-[collectionType].json"
// e.g. "localhost-guest.json"
// The file is saved in the same location as log file.
func persistCollectedData(wlm *wlm.WLM, path string) error {
	log.Logger.Debug("Saving collected result locally.")
	requestJSON, err := internal.PrettyStruct(wlm.Request)
	if err != nil {
		return err
	}
	return internal.SaveToFile(path, []byte(requestJSON))
}

// retry returns error if it exceeds max retries limits.
func retry(run func() bool, maxRetries int32, interval time.Duration) error {
	if maxRetries == -1 {
		for {
			if !run() {
				time.Sleep(interval)
				continue
			}
			return nil
		}
	}

	for retry := int32(0); retry < maxRetries; retry++ {
		if !run() {
			time.Sleep(interval)
			continue
		}
		return nil
	}
	return fmt.Errorf("reached max retries")
}

// addPhysicalDriveRemoteLinux adds physical drive to sql collection based off details for windows to remote linux instances
func addPhysicalDriveRemoteLinux(details []internal.Details, cred *configuration.GuestConfig) {
	user := cred.GuestUserName
	port := cred.GuestPortNumber
	ip := cred.ServerName
	// We need to call NewRemote, SetupKeys and CreateClient respectively to set up the remote correctly.
	r := remote.NewRemote(ip, user, port, UsageMetricsLogger)
	if err := r.SetupKeys(cred.LinuxSSHPrivateKeyPath); err != nil {
		log.Logger.Errorw("Failed to setup keys.", "error", err)
		UsageMetricsLogger.Error(agentstatus.SetupSSHKeysError)
		return
	}
	if err := r.CreateClient(); err != nil {
		log.Logger.Errorw("Failed to create client.", "error", err)
		UsageMetricsLogger.Error(agentstatus.SSHDialError)
		return
	}
	defer r.Close()
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
			dir, filePath := filepath.Split(physicalPath)
			findCommand := fmt.Sprintf(commandFind, dir, filePath)

			filePath, filePathErr := remote.RunCommandWithPipes(findCommand, r)
			if filePathErr != nil {
				log.Logger.Warnf("Failed to run cmd %v. error: %v", findCommand, filePathErr)
				continue
			}
			filePath = strings.TrimSuffix(filePath, "\n")

			command := fmt.Sprintf(commandDf, filePath)
			physicalPathMount, physicalPathErr := remote.RunCommandWithPipes(command, r)
			if physicalPathErr != nil {
				log.Logger.Warnf("Failed to run cmd %v. error: %v", command, physicalPathErr)
				continue
			}
			physicalPathMount = strings.TrimSuffix(physicalPathMount, "\n")

			resultMount, mountErr := remote.RunCommandWithPipes(commandMount, r)
			if mountErr != nil {
				log.Logger.Warnf("Failed to run cmd %v. error: %v", commandMount, mountErr)
				continue
			}

			allMounts := strings.TrimSuffix(resultMount, "\n")
			physicalDriveHelper := regexp.MustCompile(` `+physicalPathMount+` `).Split(allMounts, -1)

			physicalDrives := []string{}
			for i := 0; i < len(physicalDriveHelper)-1; i++ {
				splitStr := regexp.MustCompile("\n| |/").Split(physicalDriveHelper[i], -1)
				if len(splitStr) < 2 {
					log.Logger.Warn("regex for linux error. Unable to find physical drive associated with mount.")
					continue
				}
				physicalDrives = append(physicalDrives, splitStr[len(splitStr)-2])
			}
			physicalDrive := strings.Join(physicalDrives, ", ")
			field["physical_drive"] = physicalDrive
		}
	}
}

// addPhysicalDriveLocal starts physical drive to physical path mapping
func addPhysicalDriveLocal(ctx context.Context, details []internal.Details, windows bool) {
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

// initDetails returns empty array of internal.Details
func initDetails() []internal.Details {
	return []internal.Details{}
}

// sqlConfigFromCredential wraps the function SQLConfigFromCredential in configuration package.
func sqlConfigFromCredential(cred *configpb.CredentialConfiguration) []*configuration.SQLConfig {
	return configuration.SQLConfigFromCredential(cred)
}

// guestConfigFromCredential wraps the function GuestConfigFromCredential in configuration package.
func guestConfigFromCredential(cred *configpb.CredentialConfiguration) *configuration.GuestConfig {
	return configuration.GuestConfigFromCredential(cred)
}
