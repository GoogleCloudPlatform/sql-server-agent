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

// Package configuration contains functionalities for agent configuration operations.
package configuration

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

// SQLConfig .
type SQLConfig struct {
	Host       string
	Username   string
	SecretName string
	PortNumber int32
}

// GuestConfig .
type GuestConfig struct {
	ServerName             string
	GuestUserName          string
	GuestSecretName        string
	GuestPortNumber        int32
	LinuxRemote            bool
	LinuxSSHPrivateKeyPath string
}

// LoadConfiguration loads configuration from config file.
// Returns default configurations with error if reading configuration file has an error.
// Returns nil with error if the configuration file is in invalid format.
func LoadConfiguration(p string) (*configpb.Configuration, error) {
	// Read config file from file system.
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p), "configuration.json"))
	if err != nil {
		return &configpb.Configuration{
			CollectionConfiguration: &configpb.CollectionConfiguration{
				CollectGuestOsMetrics:                     true,
				CollectSqlMetrics:                         true,
				GuestOsMetricsCollectionIntervalInSeconds: 3600,
				SqlMetricsCollectionIntervalInSeconds:     3600,
			},
			CredentialConfiguration: []*configpb.CredentialConfiguration{
				&configpb.CredentialConfiguration{
					SqlConfigurations: []*configpb.CredentialConfiguration_SqlCredentials{
						&configpb.CredentialConfiguration_SqlCredentials{
							Host:       "localhost",
							UserName:   "",
							SecretName: "",
							PortNumber: 1433,
						},
					},
				},
			},
			LogLevel:                 "INFO",
			LogToCloud:               true,
			CollectionTimeoutSeconds: 10,
			MaxRetries:               5,
			RetryIntervalInSeconds:   3600,
		}, fmt.Errorf("failed to load the configuration file. filepath: %v, error: %v", p, err)
	}
	cfg := configpb.Configuration{}
	if err := protojson.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return validateConfigValues(&cfg), nil
}

// SQLConfigFromCredential returns config for SQL collection.
func SQLConfigFromCredential(creCfg *configpb.CredentialConfiguration) []*SQLConfig {
	var sqlConfigs []*SQLConfig
	for _, sqlCfg := range creCfg.GetSqlConfigurations() {
		sqlConfigs = append(sqlConfigs, &SQLConfig{
			Host:       sqlCfg.GetHost(),
			Username:   sqlCfg.GetUserName(),
			SecretName: sqlCfg.GetSecretName(),
			PortNumber: sqlCfg.GetPortNumber(),
		})
	}
	return sqlConfigs
}

// GuestConfigFromCredential returns config for guest OS collection.
func GuestConfigFromCredential(creCfg *configpb.CredentialConfiguration) *GuestConfig {
	switch creCfg.GuestConfigurations.(type) {
	case *configpb.CredentialConfiguration_RemoteWin:
		return &GuestConfig{
			ServerName:      creCfg.GetRemoteWin().GetServerName(),
			GuestUserName:   creCfg.GetRemoteWin().GetGuestUserName(),
			GuestSecretName: creCfg.GetRemoteWin().GetGuestSecretName(),
		}
	case *configpb.CredentialConfiguration_RemoteLinux:
		return &GuestConfig{
			ServerName:             creCfg.GetRemoteLinux().GetServerName(),
			GuestUserName:          creCfg.GetRemoteLinux().GetGuestUserName(),
			GuestPortNumber:        creCfg.GetRemoteLinux().GetGuestPortNumber(),
			LinuxRemote:            true,
			LinuxSSHPrivateKeyPath: creCfg.GetRemoteLinux().GetLinuxSshPrivateKeyPath(),
		}
	}
	return &GuestConfig{}
}

// ValidateConfigValues verifies if the numeric values from the config file are valid.
// If not, the default value will be set to the field.
func validateConfigValues(config *configpb.Configuration) *configpb.Configuration {
	fields := []struct {
		name            string
		defaultValue    int32
		minValue        int32
		valueFromConfig int32
		setDefaultValue func(int32)
	}{
		{
			name:            "collection_timeout_seconds",
			defaultValue:    10,
			minValue:        1,
			valueFromConfig: config.GetCollectionTimeoutSeconds(),
			setDefaultValue: func(defaultValue int32) {
				config.CollectionTimeoutSeconds = defaultValue
			},
		},
		{
			name:            "max_retries",
			defaultValue:    3,
			minValue:        -1,
			valueFromConfig: config.GetMaxRetries(),
			setDefaultValue: func(defaultValue int32) {
				config.MaxRetries = defaultValue
			},
		},
		{
			name:            "retry_interval_in_seconds",
			defaultValue:    3600,
			minValue:        1,
			valueFromConfig: config.GetRetryIntervalInSeconds(),
			setDefaultValue: func(defaultValue int32) {
				config.RetryIntervalInSeconds = defaultValue
			},
		},
		{
			name:            "guest_os_metrics_collection_interval_in_seconds",
			defaultValue:    3600,
			minValue:        1,
			valueFromConfig: config.GetCollectionConfiguration().GetGuestOsMetricsCollectionIntervalInSeconds(),
			setDefaultValue: func(defaultValue int32) {
				config.GetCollectionConfiguration().GuestOsMetricsCollectionIntervalInSeconds = defaultValue
			},
		},
		{
			name:            "sql_metrics_collection_interval_in_seconds",
			defaultValue:    3600,
			minValue:        1,
			valueFromConfig: config.GetCollectionConfiguration().GetSqlMetricsCollectionIntervalInSeconds(),
			setDefaultValue: func(defaultValue int32) {
				config.GetCollectionConfiguration().SqlMetricsCollectionIntervalInSeconds = defaultValue
			},
		},
	}

	for _, f := range fields {
		if f.valueFromConfig < f.minValue {
			log.Logger.Warnf("Invalid value for field %v. Using the default value %v", f.name, f.defaultValue)
			f.setDefaultValue(f.defaultValue)
		}
	}

	return config
}

// ValidateCredCfgSQL validates if the configuration file is valid for SQL collection.
// Each CredentialConfiguration must provide valid "user_name", "secret_name" and "port_number".
// If remote collection is enabled, the following fields must be provided:
//
//	"host", "instance_id", "instance_name"
func ValidateCredCfgSQL(remote, windows bool, sqlCfg *SQLConfig, guestCfg *GuestConfig, instanceID, instanceName string) error {
	errMsg := "invalid value for"
	hasError := false

	if sqlCfg.Username == "" {
		errMsg = errMsg + ` "user_name"`
		hasError = true
	}
	if sqlCfg.SecretName == "" {
		errMsg = errMsg + ` "secret_name"`
		hasError = true
	}
	if sqlCfg.PortNumber == 0 {
		errMsg = errMsg + ` "port_number"`
		hasError = true
	}

	if remote {
		if sqlCfg.Host == "" {
			errMsg = errMsg + ` "host"`
			hasError = true
		}
		if guestCfg.ServerName == "" {
			errMsg = errMsg + ` "server_name"`
			hasError = true
		}
		if guestCfg.GuestUserName == "" {
			errMsg = errMsg + ` "guest_user_name"`
			hasError = true
		}
		if windows && guestCfg.GuestSecretName == "" {
			errMsg = errMsg + ` "guest_secret_name"`
			hasError = true
		}
		if instanceID == "" {
			errMsg = errMsg + ` "instance_id"`
			hasError = true
		}
		if instanceName == "" {
			errMsg = errMsg + ` "instance_name"`
			hasError = true
		}
		if !windows {
			if guestCfg.LinuxSSHPrivateKeyPath == "" {
				errMsg = errMsg + ` "linux_ssh_private_key_path"`
				hasError = true
			}
			if guestCfg.GuestPortNumber == 0 {
				errMsg = errMsg + ` "guest_port_number"`
				hasError = true
			}
		}
	}

	if hasError {
		return fmt.Errorf(errMsg)
	}
	return nil
}

// ValidateCredCfgGuest validates if the configuration file is valid for guest collection.
// If remote collection is enabled, the following fields must be provided:
// "server_name", "guest_user_name", "guest_secret_name", "instance_id", "instance_name"
func ValidateCredCfgGuest(remote, windows bool, guestCfg *GuestConfig, instanceID, instanceName string) error {
	errMsg := "invalid value for"
	hasError := false

	if remote {
		if guestCfg.ServerName == "" {
			errMsg = errMsg + ` "server_name"`
			hasError = true
		}
		if guestCfg.GuestUserName == "" {
			errMsg = errMsg + ` "guest_user_name"`
			hasError = true
		}
		if windows && guestCfg.GuestSecretName == "" {
			errMsg = errMsg + ` "guest_secret_name"`
			hasError = true
		}
		if instanceID == "" {
			errMsg = errMsg + ` "instance_id"`
			hasError = true
		}
		if instanceName == "" {
			errMsg = errMsg + ` "instance_name"`
			hasError = true
		}
		if !windows {
			if guestCfg.LinuxSSHPrivateKeyPath == "" {
				errMsg = errMsg + ` "linux_ssh_private_key_path"`
				hasError = true
			}
			if guestCfg.GuestPortNumber == 0 {
				errMsg = errMsg + ` "guest_port_number"`
				hasError = true
			}
		}
	}

	if hasError {
		return fmt.Errorf(errMsg)
	}
	return nil
}
