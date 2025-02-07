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

// Package main serves as the Main entry point for sql server agent.
package main

import (
	"context"
	"fmt"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/daemon"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/sqlservermetrics"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

func main() {
	flags, output, proceed := sqlservermetrics.Init()
	if output != "" {
		fmt.Println(output)
	}
	if !proceed {
		return
	}

	ctx := context.Background()
	// Load default logging configuration.
	sqlservermetrics.LoggingSetupDefault(ctx, sqlservermetrics.LogPrefix())
	// Load configuration.
	cfg, err := sqlservermetrics.LoadConfiguration(sqlservermetrics.ConfigPath())
	if cfg == nil {
		log.Logger.Fatalw("Failed to load configuration", "error", err)
	}
	if err != nil {
		log.Logger.Errorw("Failed to load configuration. Using default configurations", "error", err)
	}
	// Load logging configuration based on the configuration file.
	sqlservermetrics.LoggingSetup(ctx, sqlservermetrics.LogPrefix(), cfg)

	// onetime collection
	if flags.Onetime {
		if err := sqlservermetrics.OSCollection(ctx, sqlservermetrics.AgentFilePath(), sqlservermetrics.LogPrefix(), cfg, true); err != nil {
			log.Logger.Errorw("Failed to complete os collection", "error", err)
		}
		if err := sqlservermetrics.SQLCollection(ctx, sqlservermetrics.AgentFilePath(), sqlservermetrics.LogPrefix(), cfg, true); err != nil {
			log.Logger.Errorw("Failed to complete sql collection", "error", err)
		}
		return
	}
	// Init UsageMetricsLogger by reading "disable_log_usage" from the configuration file.
	sqlservermetrics.UsageMetricsLogger = sqlservermetrics.UsageMetricsLoggerInit(sqlservermetrics.ServiceName, sqlservermetrics.AgentVersion, sqlservermetrics.AgentUsageLogPrefix, !cfg.GetDisableLogUsage())
	osCollectionFunc := func(cfg *configpb.Configuration, onetime bool) error {
		return sqlservermetrics.OSCollection(ctx, sqlservermetrics.AgentFilePath(), sqlservermetrics.LogPrefix(), cfg, onetime)
	}
	sqlCollectionFunc := func(cfg *configpb.Configuration, onetime bool) error {
		return sqlservermetrics.SQLCollection(ctx, sqlservermetrics.AgentFilePath(), sqlservermetrics.LogPrefix(), cfg, onetime)
	}

	s, err := daemon.CreateService(
		func() {
			sqlservermetrics.CollectionService(sqlservermetrics.ConfigPath(), osCollectionFunc, sqlservermetrics.OS)
		},
		func() {
			sqlservermetrics.CollectionService(sqlservermetrics.ConfigPath(), sqlCollectionFunc, sqlservermetrics.SQL)
		},
		daemon.CreateConfig(sqlservermetrics.ServiceName, sqlservermetrics.ServiceDisplayName, sqlservermetrics.Description),
		sqlservermetrics.UsageMetricsLogger)

	if err != nil {
		log.Logger.Fatalw("Failed to create the service", "error", err)
	}

	if err = daemon.Control(s, flags.Action, sqlservermetrics.UsageMetricsLogger); err != nil {
		log.Logger.Fatal(err)
	}
}
