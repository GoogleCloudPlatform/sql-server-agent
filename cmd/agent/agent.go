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

// Package agent offers functions that is commonly used by both windows and linux agent.
package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"github.com/jonboulle/clockwork"
	"github.com/GoogleCloudPlatform/sapagent/shared/gce"
	"github.com/GoogleCloudPlatform/sapagent/shared/gce/metadataserver"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent/agentshared"
	"github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent/flags"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/activation"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/configuration"
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
	ServiceName = "google-cloud-sql-server-agent"
	// ServiceDisplayName .
	ServiceDisplayName = "Google Cloud Agent for SQL Server"
	// Description .
	Description = "Google Cloud Agent for SQL Server."
	// ExperimentalMode .
	ExperimentalMode = internal.ExperimentalMode
	driver           = "sqlserver"
	commandFind      = `sudo find %s -type f -iname "%s" -print`
	commandDf        = "sudo df --output=target %s | tail -n 1"
	commandMount     = "mount | grep sd"
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
var UsageMetricsLogger agentstatus.AgentStatus = UsageMetricsLoggerInit(false)

// UsageMetricsLoggerInit initializes and returns usage metrics logger.
func UsageMetricsLoggerInit(logUsage bool) agentstatus.AgentStatus {
	ap := agentstatus.NewAgentProperties(ServiceName, internal.AgentVersion, logUsage)
	sip := SourceInstanceProperties()
	cp := agentstatus.NewCloudProperties(sip.ProjectID, sip.Zone, sip.Instance, sip.ProjectNumber, sip.Image)
	return agentstatus.NewUsageMetricsLogger(ap, cp, clockwork.NewRealClock(), []string{})
}

// SourceInstanceProperties returns properties of the instance the agent is running on.
func SourceInstanceProperties() InstanceProperties {
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

// Init parses flags and execute if certain flags are enabled.
func Init() (*flags.AgentFlags, string, bool) {
	f := flags.NewAgentFlags()
	output, proceed := f.Execute()
	return f, output, proceed
}

// LoggingSetup initialize the agent logging level.
func LoggingSetup(ctx context.Context, logPrefix string, cfg *configpb.Configuration) {
	agentshared.LoggingSetup(ctx, logPrefix, cfg.GetLogLevel(), SourceInstanceProperties().ProjectID, cfg.GetLogToCloud())
}

// LoggingSetupDefault wraps LoggingSetupDefault function from agent_shared.go.
func LoggingSetupDefault(ctx context.Context, prefix string) {
	agentshared.LoggingSetupDefault(ctx, prefix)
}

// InitCollection executes steps for initializing a collection.
// The func is called at the beginning of every guest and sql collection.
func InitCollection(ctx context.Context) (*wlm.WLM, error) {
	wlm, err := wlm.NewWorkloadManager(ctx)
	if err != nil {
		return nil, err
	}
	return wlm, nil
}

// CheckAgentStatus checks agent status. Return error if it failed to activate.
func CheckAgentStatus(wlm wlm.WorkloadManagerService, path string) error {
	ip := SourceInstanceProperties()
	return agentshared.CheckAgentStatus(activation.NewV1(), wlm, filepath.Join(filepath.Dir(path), "google-cloud-sql-server-agent.activated"), ip.Name, ip.ProjectID, ip.Instance, ip.InstanceID)
}

// LoadConfiguration loads configuration from given path.
func LoadConfiguration(path string) (*configpb.Configuration, error) {
	return configuration.LoadConfiguration(path)
}

// ValidateCredCfgSQL wraps ValidateCredCfgSQL from configuration package.
func ValidateCredCfgSQL(remote, windows bool, sqlCfg *configuration.SQLConfig, guestCfg *configuration.GuestConfig, instanceID, instanceName string) error {
	return configuration.ValidateCredCfgSQL(remote, windows, sqlCfg, guestCfg, instanceID, instanceName)
}

// ValidateCredCfgGuest wraps ValidateCredCfgGuest from configuration package.
func ValidateCredCfgGuest(remote, windows bool, guestCfg *configuration.GuestConfig, instanceID, instanceName string) error {
	return configuration.ValidateCredCfgGuest(remote, windows, guestCfg, instanceID, instanceName)
}

// RunSQLCollection starts running sql collection based on given connection string.
func RunSQLCollection(ctx context.Context, conn string, timeout time.Duration, windows bool) ([]internal.Details, error) {
	c, err := sqlcollector.NewV1(driver, conn, windows, UsageMetricsLogger)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return agentshared.RunSQLCollection(ctx, c, timeout), nil
}

// RunOSCollection starts running os collection.
func RunOSCollection(ctx context.Context, c guestcollector.GuestCollector, timeout time.Duration) []internal.Details {
	return agentshared.RunOSCollection(ctx, c, timeout)
}

// SecretValue gets secret value from Secret Manager.
func SecretValue(ctx context.Context, projectID string, secretName string) (string, error) {
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

// AllDisks attempts to call compute api to return all possible disks.
func AllDisks(ctx context.Context, ip InstanceProperties) ([]*instanceinfo.Disks, error) {
	tempGCE, err := gce.NewGCEClient(ctx)
	if err != nil {
		return nil, err
	}

	r := instanceinfo.New(tempGCE)
	return r.AllDisks(ctx, ip.ProjectID, ip.Zone, ip.InstanceID)
}

// UpdateCollectedData constructs writeinsightrequest from given collected details.
// The func will be called by both guest and sql collections.
func UpdateCollectedData(wlmService wlm.WorkloadManagerService, sourceProps, targetProps InstanceProperties, details []internal.Details) {
	sqlservervalidation := wlm.InitializeSQLServerValidation(sourceProps.ProjectID, targetProps.Instance)
	sqlservervalidation = wlm.UpdateValidationDetails(sqlservervalidation, details)
	writeInsightRequest := wlm.InitializeWriteInsightRequest(sqlservervalidation, targetProps.InstanceID)
	writeInsightRequest.Insight.SentTime = time.Now().Format(time.RFC3339)
	// update wlmService.Request to writeInsightRequest
	wlmService.UpdateRequest(writeInsightRequest)
}

// SendRequestToWLM sends request to workloadmanager.
func SendRequestToWLM(wlmService wlm.WorkloadManagerService, location string, retries int32, interval time.Duration) {
	sendRequest := func() bool {
		_, err := wlmService.SendRequest(location)
		if err != nil {
			log.Logger.Errorw("Failed to send request to workload manager", "error", err)
			UsageMetricsLogger.Error(agentstatus.WorkloadManagerConnectionError)
			return false
		}
		return true
	}

	if err := Retry(sendRequest, retries, interval); err != nil {
		log.Logger.Errorw("Failed to retry sending request to workload manager", "error", err)
		UsageMetricsLogger.Error(agentstatus.WorkloadManagerConnectionError)
	}
}

// PersistCollectedData persists collected data in the file system.
// The file name follows the format "[target]-[collectionType].json"
// e.g. "localhost-guest.json"
// The file is saved in the same location as log file.
func PersistCollectedData(wlm *wlm.WLM, path string) error {
	log.Logger.Debug("Saving collected result locally.")
	requestJSON, err := internal.PrettyStruct(wlm.Request)
	if err != nil {
		return err
	}
	return internal.SaveToFile(path, []byte(requestJSON))
}

// Retry returns error if it exceeds max retries limits.
func Retry(run func() bool, maxRetries int32, interval time.Duration) error {
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
		UsageMetricsLogger = UsageMetricsLoggerInit(cfg.GetLogUsage())
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

// AddPhysicalDriveRemoteLinux adds physical drive to sql collection based off details for windows to remote linux instances
func AddPhysicalDriveRemoteLinux(details []internal.Details, cred *configuration.GuestConfig) {
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

// AddPhysicalDriveLocal starts physical drive to physical path mapping
func AddPhysicalDriveLocal(ctx context.Context, details []internal.Details, windows bool) {
	agentshared.AddPhysicalDriveLocal(ctx, details, windows)
}

// InitDetails returns empty array of internal.Details
func InitDetails() []internal.Details {
	return []internal.Details{}
}

// SQLConfigFromCredential wraps the function SQLConfigFromCredential in configuration package.
func SQLConfigFromCredential(cred *configpb.CredentialConfiguration) []*configuration.SQLConfig {
	return configuration.SQLConfigFromCredential(cred)
}

// GuestConfigFromCredential wraps the function GuestConfigFromCredential in configuration package.
func GuestConfigFromCredential(cred *configpb.CredentialConfiguration) *configuration.GuestConfig {
	return configuration.GuestConfigFromCredential(cred)
}
