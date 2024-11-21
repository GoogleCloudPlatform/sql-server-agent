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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/guestcollector"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/sqlservermetrics"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

func logPrefix() string {
	return filepath.Join(
		os.Getenv("ProgramData"),
		"Google",
		"google-cloud-sql-server-agent",
		"logs",
		"google-cloud-sql-server-agent")
}

func configPath() string {
	p, err := os.Executable()
	if err != nil {
		log.Logger.Fatalw("Failed to get the path of executable", "error", err)
	}
	return p
}

func agentFilePath() string {
	p, err := os.Executable()
	if err != nil {
		log.Logger.Fatalw("Failed to get the path of executable", "error", err)
	}
	return p
}

func osCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectGuestOsMetrics() {
		return nil
	}
	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}
	wlm, err := sqlservermetrics.InitCollection(ctx)
	if err != nil {
		return err
	}
	if !onetime {
		if err := sqlservermetrics.CheckAgentStatus(wlm, path); err != nil {
			return err
		}
	}

	sourceInstanceProps := sqlservermetrics.SIP
	timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
	interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second

	log.Logger.Info("Guest rules collection starts.")
	for _, credentialCfg := range cfg.GetCredentialConfiguration() {
		guestCfg := sqlservermetrics.GuestConfigFromCredential(credentialCfg)
		if err := sqlservermetrics.ValidateCredCfgGuest(cfg.GetRemoteCollection(), !guestCfg.LinuxRemote, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
			log.Logger.Errorw("Invalid credential configuration", "error", err)
			sqlservermetrics.UsageMetricsLogger.Error(agentstatus.InvalidConfigurationsError)
			if !cfg.GetRemoteCollection() {
				break
			}
			continue
		}

		targetInstanceProps := sourceInstanceProps
		var c guestcollector.GuestCollector
		if cfg.GetRemoteCollection() {
			// remote collection
			targetInstanceProps = sqlservermetrics.InstanceProperties{
				InstanceID: credentialCfg.GetInstanceId(),
				Instance:   credentialCfg.GetInstanceName(),
			}
			host := guestCfg.ServerName
			username := guestCfg.GuestUserName
			if !guestCfg.LinuxRemote {
				log.Logger.Debug("Starting remote win guest collection for ip " + host)
				pswd, err := sqlservermetrics.SecretValue(ctx, sourceInstanceProps.ProjectID, guestCfg.GuestSecretName)
				if err != nil {
					log.Logger.Errorw("Collection failed", "target", guestCfg.ServerName, "error", fmt.Errorf("failed to get secret value: %v", err))
					sqlservermetrics.UsageMetricsLogger.Error(agentstatus.SecretValueError)
					if !cfg.GetRemoteCollection() {
						break
					}
					continue
				}
				c = guestcollector.NewWindowsCollector(host, username, pswd, sqlservermetrics.UsageMetricsLogger)
			} else {
				// on local windows vm collecting on remote linux vm's, we use ssh, otherwise we use wmi
				log.Logger.Debug("Starting remote linux guest collection for ip " + host)
				// disks only used for local linux collection
				c = guestcollector.NewLinuxCollector(nil, host, username, guestCfg.LinuxSSHPrivateKeyPath, true, guestCfg.GuestPortNumber, sqlservermetrics.UsageMetricsLogger)
			}
		} else {
			// local win collection
			log.Logger.Debug("Starting local win guest collection")
			c = guestcollector.NewWindowsCollector(nil, nil, nil, sqlservermetrics.UsageMetricsLogger)
		}

		details := sqlservermetrics.RunOSCollection(ctx, c, timeout)
		sqlservermetrics.UpdateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, details)
		log.Logger.Debug("Finished guest collection")

		if onetime {
			target := "localhost"
			if cfg.GetRemoteCollection() {
				target = credentialCfg.GetInstanceName()
			}
			sqlservermetrics.PersistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", target, "guest")))
		} else {
			log.Logger.Debugf("Source vm %s is sending os collected data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
			sqlservermetrics.SendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
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

func sqlCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectSqlMetrics() {
		return nil
	}
	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}

	wlm, err := sqlservermetrics.InitCollection(ctx)
	if err != nil {
		return err
	}
	if !onetime {
		if err := sqlservermetrics.CheckAgentStatus(wlm, path); err != nil {
			return err
		}
	}

	sourceInstanceProps := sqlservermetrics.SIP
	timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
	interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second

	log.Logger.Info("SQL rules collection starts.")
	for _, credentialCfg := range cfg.GetCredentialConfiguration() {
		validationDetails := sqlservermetrics.InitDetails()
		guestCfg := sqlservermetrics.GuestConfigFromCredential(credentialCfg)
		for _, sqlCfg := range sqlservermetrics.SQLConfigFromCredential(credentialCfg) {
			if err := sqlservermetrics.ValidateCredCfgSQL(cfg.GetRemoteCollection(), !guestCfg.LinuxRemote, sqlCfg, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
				log.Logger.Errorw("Invalid credential configuration", "error", err)
				sqlservermetrics.UsageMetricsLogger.Error(agentstatus.InvalidConfigurationsError)
				continue
			}
			pswd, err := sqlservermetrics.SecretValue(ctx, sourceInstanceProps.ProjectID, sqlCfg.SecretName)
			if err != nil {
				log.Logger.Errorw("Failed to get secret value", "error", err)
				sqlservermetrics.UsageMetricsLogger.Error(agentstatus.SecretValueError)
				continue
			}
			conn := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;", sqlCfg.Host, sqlCfg.Username, pswd, sqlCfg.PortNumber)
			details, err := sqlservermetrics.RunSQLCollection(ctx, conn, timeout, !guestCfg.LinuxRemote)
			if err != nil {
				log.Logger.Errorw("Failed to run sql collection", "error", err)
				sqlservermetrics.UsageMetricsLogger.Error(agentstatus.SQLCollectionFailure)
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
				sqlservermetrics.AddPhysicalDriveRemoteLinux(details, guestCfg)
			} else {
				sqlservermetrics.AddPhysicalDriveLocal(ctx, details, true)
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
			targetInstanceProps = sqlservermetrics.InstanceProperties{
				InstanceID: credentialCfg.GetInstanceId(),
				Instance:   credentialCfg.GetInstanceName(),
			}
		}
		sqlservermetrics.UpdateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, validationDetails)
		if onetime {
			target := "localhost"
			if cfg.GetRemoteCollection() {
				target = targetInstanceProps.Instance
			}
			sqlservermetrics.PersistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", target, "sql")))
		} else {
			log.Logger.Debugf("Source vm %s is sending collected sql data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
			sqlservermetrics.SendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
		}
	}
	log.Logger.Info("SQL rules collection ends.")
	return nil
}
