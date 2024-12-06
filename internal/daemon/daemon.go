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

// Package daemon uses kardianos service to make the app run as service / daemon on windows / linux.
package daemon

import (
	"time"

	"github.com/kardianos/service"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/workloadagentplatform/integration/common/shared/log"
)

type program struct {
	statusLogger  agentstatus.AgentStatus
	osCollection  func()
	sqlCollection func()
}

func (p *program) Start(s service.Service) error {
	log.Logger.Info("Service starts.")

	if p.osCollection != nil {
		go p.osCollection()
	}
	if p.sqlCollection != nil {
		go p.sqlCollection()
	}

	go func() {
		// Wait for 5 minutes in case the service was killed after it starts.
		// The agent logs the status as Running after the first 5 mins wait. Then it logs the status
		// in every hour.
		time.Sleep(5 * time.Minute)
		for {
			p.statusLogger.Running()
			time.Sleep(time.Hour)
		}
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	log.Logger.Info("Service ends.")
	p.statusLogger.Stopped()
	return nil
}

func install(s service.Service, statusLogger agentstatus.AgentStatus) error {
	if err := s.Install(); err != nil {
		return err
	}
	log.Logger.Info("Service installs.")
	statusLogger.Installed()
	return nil
}

func uninstall(s service.Service, statusLogger agentstatus.AgentStatus) error {
	if err := s.Uninstall(); err != nil {
		return err
	}
	log.Logger.Info("Service uninstalls.")
	statusLogger.Uninstalled()
	return nil
}

// CreateConfig creates and returns Config pointer for the service.
func CreateConfig(name, displayName, description string) *service.Config {
	serviceArg := []string{"--action=run"}

	return &service.Config{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Arguments:   serviceArg,
	}
}

// CreateService initializes and returns service, or error if any.
func CreateService(osCollection func(), sqlCollection func(), sc *service.Config, statusLogger agentstatus.AgentStatus) (service.Service, error) {
	prg := &program{
		statusLogger:  statusLogger,
		osCollection:  osCollection,
		sqlCollection: sqlCollection,
	}
	s, err := service.New(prg, sc)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Control wraps the function from service package and adds the supported action "run" to the service.
func Control(s service.Service, action string, statusLogger agentstatus.AgentStatus) error {
	var err error
	switch action {
	case "run":
		err = s.Run()
	case "install":
		err = install(s, statusLogger)
	case "uninstall":
		err = uninstall(s, statusLogger)
	default:
		err = service.Control(s, action)
	}
	return err
}
