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

package sqlservermetrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/guestcollector"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

// LogPrefix is the log prefix for windows.
func LogPrefix() string {
	return filepath.Join(
		os.Getenv("ProgramData"),
		"Google",
		"google-cloud-sql-server-agent",
		"logs",
		"google-cloud-sql-server-agent")
}

// ConfigPath is the config path for windows.
func ConfigPath() string {
	p, err := os.Executable()
	if err != nil {
		log.Logger.Fatalw("Failed to get the path of executable", "error", err)
	}
	return p
}

// AgentFilePath is the agent file path for windows.
func AgentFilePath() string {
	p, err := os.Executable()
	if err != nil {
		log.Logger.Fatalw("Failed to get the path of executable", "error", err)
	}
	return p
}

// OSCollection is the windows implementation of OSCollection.
func OSCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectGuestOsMetrics() {
		return nil
	}
	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}
	wlm, err := initCollection(ctx)
	if err != nil {
		return err
	}
	if !onetime {
		if err := checkAgentStatus(wlm, path); err != nil {
			return err
		}
	}

	sourceInstanceProps := SIP
	timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
	interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second

	log.Logger.Info("Guest rules collection starts.")
	for _, credentialCfg := range cfg.GetCredentialConfiguration() {
		guestCfg := guestConfigFromCredential(credentialCfg)
		if err := validateCredCfgGuest(cfg.GetRemoteCollection(), !guestCfg.LinuxRemote, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
			log.Logger.Errorw("Invalid credential configuration", "error", err)
			UsageMetricsLogger.Error(agentstatus.InvalidConfigurationsError)
			if !cfg.GetRemoteCollection() {
				break
			}
			continue
		}

		targetInstanceProps := sourceInstanceProps
		var c guestcollector.GuestCollector
		if cfg.GetRemoteCollection() {
			// remote collection
			targetInstanceProps = InstanceProperties{
				InstanceID: credentialCfg.GetInstanceId(),
				Instance:   credentialCfg.GetInstanceName(),
			}
			host := guestCfg.ServerName
			username := guestCfg.GuestUserName
			if !guestCfg.LinuxRemote {
				log.Logger.Debug("Starting remote win guest collection for ip " + host)
				pswd, err := secretValue(ctx, sourceInstanceProps.ProjectID, guestCfg.GuestSecretName)
				if err != nil {
					log.Logger.Errorw("Collection failed", "target", guestCfg.ServerName, "error", fmt.Errorf("failed to get secret value: %v", err))
					UsageMetricsLogger.Error(agentstatus.SecretValueError)
					if !cfg.GetRemoteCollection() {
						break
					}
					continue
				}
				c = guestcollector.NewWindowsCollector(host, username, pswd, UsageMetricsLogger)
			} else {
				// on local windows vm collecting on remote linux vm's, we use ssh, otherwise we use wmi
				log.Logger.Debug("Starting remote linux guest collection for ip " + host)
				// disks only used for local linux collection
				c = guestcollector.NewLinuxCollector(nil, host, username, guestCfg.LinuxSSHPrivateKeyPath, true, guestCfg.GuestPortNumber, UsageMetricsLogger)
			}
		} else {
			// local win collection
			log.Logger.Debug("Starting local win guest collection")
			c = guestcollector.NewWindowsCollector(nil, nil, nil, UsageMetricsLogger)
		}

		details := runOSCollection(ctx, c, timeout)
		updateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, details)
		log.Logger.Debug("Finished guest collection")

		if onetime {
			target := "localhost"
			if cfg.GetRemoteCollection() {
				target = credentialCfg.GetInstanceName()
			}
			persistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", target, "guest")))
		} else {
			log.Logger.Debugf("Source vm %s is sending os collected data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
			sendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
		}
		// Local collection.
		// Exit the loop. Only take the first credential in the credentialconfiguration array.
		if !cfg.GetRemoteCollection() {
			break
		}
	}
	log.Logger.Info("Guest rules collection ends.")

	return nil
}

// SQLCollection is the windows implementation of SQLCollection.
func SQLCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectSqlMetrics() {
		return nil
	}
	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}

	wlm, err := initCollection(ctx)
	if err != nil {
		return err
	}
	if !onetime {
		if err := checkAgentStatus(wlm, path); err != nil {
			return err
		}
	}

	sourceInstanceProps := SIP
	timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
	interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second

	log.Logger.Info("SQL rules collection starts.")
	for _, credentialCfg := range cfg.GetCredentialConfiguration() {
		validationDetails := initDetails()
		guestCfg := guestConfigFromCredential(credentialCfg)
		for _, sqlCfg := range sqlConfigFromCredential(credentialCfg) {
			if err := validateCredCfgSQL(cfg.GetRemoteCollection(), !guestCfg.LinuxRemote, sqlCfg, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
				log.Logger.Errorw("Invalid credential configuration", "error", err)
				UsageMetricsLogger.Error(agentstatus.InvalidConfigurationsError)
				continue
			}
			pswd, err := secretValue(ctx, sourceInstanceProps.ProjectID, sqlCfg.SecretName)
			if err != nil {
				log.Logger.Errorw("Failed to get secret value", "error", err)
				UsageMetricsLogger.Error(agentstatus.SecretValueError)
				continue
			}
			conn := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;", sqlCfg.Host, sqlCfg.Username, pswd, sqlCfg.PortNumber)
			details, err := runSQLCollection(ctx, conn, timeout, !guestCfg.LinuxRemote)
			if err != nil {
				log.Logger.Errorw("Failed to run sql collection", "error", err)
				UsageMetricsLogger.Error(agentstatus.SQLCollectionFailure)
				continue
			}

			for _, detail := range details {
				for _, field := range detail.Fields {
					field["host_name"] = sqlCfg.Host
					field["port_number"] = fmt.Sprintf("%d", sqlCfg.PortNumber)
				}
			}

			// getting physical drive if on local windows collecting sql on linux remote
			if cfg.GetRemoteCollection() && guestCfg.LinuxRemote {
				addPhysicalDriveRemoteLinux(details, guestCfg)
			} else {
				addPhysicalDriveLocal(ctx, details, true)
			}

			for i, detail := range details {
				for _, vd := range validationDetails {
					if detail.Name == vd.Name {
						detail.Fields = append(vd.Fields, detail.Fields...)
						details[i] = detail
						break
					}
				}
			}
			validationDetails = details
		}

		targetInstanceProps := sourceInstanceProps
		// update targetInstanceProps value for remote collections.
		if cfg.GetRemoteCollection() {
			// remote collection
			targetInstanceProps = InstanceProperties{
				InstanceID: credentialCfg.GetInstanceId(),
				Instance:   credentialCfg.GetInstanceName(),
			}
		}
		updateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, validationDetails)
		if onetime {
			target := "localhost"
			if cfg.GetRemoteCollection() {
				target = targetInstanceProps.Instance
			}
			persistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", target, "sql")))
		} else {
			log.Logger.Debugf("Source vm %s is sending collected sql data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
			sendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
		}
	}
	log.Logger.Info("SQL rules collection ends.")
	return nil
}
