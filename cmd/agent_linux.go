/*
Copyright 2022 Google LLC

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
	"path/filepath"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/guestcollector"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

func logPrefix() string {
	return "/var/log/google-cloud-sql-server-agent"
}

func configPath() string {
	return "/etc/google-cloud-sql-server-agent/"
}

func agentFilePath() string {
	return "/tmp/"
}

func osCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectGuestOsMetrics() {
		return nil
	}

	if cfg.GetRemoteCollection() {
		return fmt.Errorf("remote collection from a linux vm is not supported; please use a windows vm to collect on other remote machines or turn off the remote collection flag")
	}

	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}

	wlm, err := agent.InitCollection(ctx)
	if err != nil {
		return err
	}

	if !onetime {
		if err := agent.CheckAgentStatus(wlm, path); err != nil {
			return err
		}
	}
	log.Logger.Info("Guest os rules collection starts.")
	// only local collection is supported for linux binary.
	// therefore we only get the first credential from cred list and ignore the followings.
	credentialCfg := cfg.GetCredentialConfiguration()[0]
	guestCfg := agent.GuestConfigFromCredential(credentialCfg)
	if err := agent.ValidateCredCfgGuest(false, !guestCfg.LinuxRemote, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
		return err
	}

	sourceInstanceProps := agent.SIP
	targetInstanceProps := sourceInstanceProps
	disks, err := agent.AllDisks(ctx, targetInstanceProps)
	if err != nil {
		return fmt.Errorf("failed to collect disk info: %w", err)
	}

	c := guestcollector.NewLinuxCollector(disks, "", "", "", false, 22, agent.UsageMetricsLogger)
	timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
	details := agent.RunOSCollection(ctx, c, timeout)
	agent.UpdateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, details)

	if onetime {
		target := "localhost"
		agent.PersistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", target, "guest")))
	} else {
		log.Logger.Debugf("Source vm %s is sending os collected data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
		interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second
		agent.SendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
	}
	log.Logger.Info("Guest os rules collection ends.")
	return nil
}

func sqlCollection(ctx context.Context, path, logPrefix string, cfg *configpb.Configuration, onetime bool) error {
	if !cfg.GetCollectionConfiguration().GetCollectSqlMetrics() {
		return nil
	}
	if cfg.GetRemoteCollection() {
		return fmt.Errorf("remote collection from a linux vm is not supported; please use a windows vm to collect on other remote machines or turn off the remote collection flag")
	}
	if cfg.GetCredentialConfiguration() == nil || len(cfg.GetCredentialConfiguration()) == 0 {
		return fmt.Errorf("empty credentials")
	}

	wlm, err := agent.InitCollection(ctx)
	if err != nil {
		return err
	}

	if !onetime {
		if err := agent.CheckAgentStatus(wlm, path); err != nil {
			return err
		}
	}

	log.Logger.Info("Sql rules collection starts.")
	for _, credentialCfg := range cfg.GetCredentialConfiguration() {
		validationDetails := agent.InitDetails()
		sourceInstanceProps := agent.SIP
		guestCfg := agent.GuestConfigFromCredential(credentialCfg)
		for _, sqlCfg := range agent.SQLConfigFromCredential(credentialCfg) {
			if err := agent.ValidateCredCfgSQL(false, !guestCfg.LinuxRemote, sqlCfg, guestCfg, credentialCfg.GetInstanceId(), credentialCfg.GetInstanceName()); err != nil {
				log.Logger.Errorw("Invalid credential configuration", "error", err)
				agent.UsageMetricsLogger.Error(agentstatus.InvalidConfigurationsError)
				continue
			}
			pswd, err := agent.SecretValue(ctx, sourceInstanceProps.ProjectID, sqlCfg.SecretName)
			if err != nil {
				log.Logger.Errorw("Failed to get secret value", "error", err)
				agent.UsageMetricsLogger.Error(agentstatus.SecretValueError)
				continue
			}
			conn := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;", sqlCfg.Host, sqlCfg.Username, pswd, sqlCfg.PortNumber)
			timeout := time.Duration(cfg.GetCollectionTimeoutSeconds()) * time.Second
			details, err := agent.RunSQLCollection(ctx, conn, timeout, false)
			if err != nil {
				log.Logger.Errorw("Failed to run sql collection", "error", err)
				agent.UsageMetricsLogger.Error(agentstatus.SQLCollectionFailure)
				continue
			}
			for _, detail := range details {
				for _, field := range detail.Fields {
					field["host_name"] = sqlCfg.Host
					field["port_number"] = fmt.Sprintf("%d", sqlCfg.PortNumber)
				}
			}
			agent.AddPhysicalDriveLocal(ctx, details, false)

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
		agent.UpdateCollectedData(wlm, sourceInstanceProps, targetInstanceProps, validationDetails)

		if onetime {
			agent.PersistCollectedData(wlm, filepath.Join(filepath.Dir(logPrefix), fmt.Sprintf("%s-%s.json", targetInstanceProps.Instance, "sql")))
		} else {
			log.Logger.Debugf("Source vm %s is sending collected sql data on target machine, %s, to workload manager.", sourceInstanceProps.Instance, targetInstanceProps.Instance)
			interval := time.Duration(cfg.GetRetryIntervalInSeconds()) * time.Second
			agent.SendRequestToWLM(wlm, sourceInstanceProps.Name, cfg.GetMaxRetries(), interval)
		}
	}
	log.Logger.Info("Sql rules collection ends.")
	return nil
}
