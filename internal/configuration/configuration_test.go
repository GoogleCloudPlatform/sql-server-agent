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

package configuration

import (
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	configpb "github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig"
)

func TestLoadConfiguration(t *testing.T) {
	testcases := []struct {
		name          string
		unmarshallErr bool
		readFileErr   bool
		want          *configpb.Configuration
		wantErr       bool
	}{
		{
			name: "success",
			want: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{
					CollectGuestOsMetrics:                     true,
					GuestOsMetricsCollectionIntervalInSeconds: 30,
					CollectSqlMetrics:                         true,
					SqlMetricsCollectionIntervalInSeconds:     30,
				},
				CredentialConfiguration: []*configpb.CredentialConfiguration{
					&configpb.CredentialConfiguration{
						SqlConfigurations: []*configpb.CredentialConfiguration_SqlCredentials{
							&configpb.CredentialConfiguration_SqlCredentials{
								Host:       ".",
								UserName:   "test-user-name",
								SecretName: "test-secret-name",
								PortNumber: 1433,
							},
						},
						GuestConfigurations: &configpb.CredentialConfiguration_LocalCollection{
							LocalCollection: true,
						},
					},
				},
				LogLevel:                 "DEBUG",
				CollectionTimeoutSeconds: 30,
				RetryIntervalInSeconds:   3600,
			},
		},
		{
			name:          "config file invalid",
			unmarshallErr: true,
			want:          nil,
			wantErr:       true,
		},
		{
			name:        "read file error-return default configuration",
			readFileErr: true,
			want: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{
					CollectGuestOsMetrics:                     true,
					GuestOsMetricsCollectionIntervalInSeconds: 3600,
					CollectSqlMetrics:                         true,
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
				CollectionTimeoutSeconds: 10,
				MaxRetries:               5,
				RetryIntervalInSeconds:   3600,
			},
			wantErr: true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var content string
			if !tc.unmarshallErr {
				content = `
{
	"collection_configuration": {
		"collect_guest_os_metrics": true,
		"guest_os_metrics_collection_interval_in_seconds": 30,
		"collect_sql_metrics": true,
		"sql_metrics_collection_interval_in_seconds": 30
	},
	"credential_configuration": [
		{
			"sql_configurations": [
				{
						"host": ".",
						"user_name": "test-user-name",
						"secret_name": "test-secret-name",
						"port_number": 1433
				}
			],
			"local_collection":true
		}
	],
	"log_level": "DEBUG",
	"collection_timeout_seconds": 30
}`
			} else {
				content = `{
	"anyfield": "anyvalue"
}`
			}

			tempFilePath := path.Join(t.TempDir(), "configuration.json")

			if !tc.readFileErr {
				if err := os.WriteFile(tempFilePath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := LoadConfiguration(tempFilePath)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("loadConfiguration() = %v, want error presence = %v", got, err)
			}

			if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
				t.Errorf("loadConfiguration() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestSQLConfigFromCredential(t *testing.T) {
	tests := []struct {
		name  string
		input *configpb.CredentialConfiguration
		want  []*SQLConfig
	}{
		{
			name: "SQLConfig with new configuration format-local",
			input: &configpb.CredentialConfiguration{
				SqlConfigurations: []*configpb.CredentialConfiguration_SqlCredentials{
					&configpb.CredentialConfiguration_SqlCredentials{
						Host:       "test-host",
						UserName:   "test-user-name",
						SecretName: "test-secret-name",
						PortNumber: 1433,
					},
				},
			},
			want: []*SQLConfig{
				&SQLConfig{
					Host:       "test-host",
					Username:   "test-user-name",
					SecretName: "test-secret-name",
					PortNumber: 1433,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SQLConfigFromCredential(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SQLConfigFromCredential(%v) returned an unexpected diff (-want +got): %v", tc.input, diff)
			}
		})
	}
}

func TestGuestConfigFromCredential(t *testing.T) {
	tests := []struct {
		name  string
		input *configpb.CredentialConfiguration
		want  *GuestConfig
	}{
		{
			name: "GuestConfig with new configuration format-local",
			input: &configpb.CredentialConfiguration{
				GuestConfigurations: &configpb.CredentialConfiguration_LocalCollection{
					LocalCollection: true,
				},
			},
			want: &GuestConfig{},
		},
		{
			name: "GuestConfig with new configuration format-remote_win",
			input: &configpb.CredentialConfiguration{
				GuestConfigurations: &configpb.CredentialConfiguration_RemoteWin{
					RemoteWin: &configpb.CredentialConfiguration_GuestCredentialsRemoteWin{
						ServerName:      "test-server-name",
						GuestUserName:   "test-guest-user-name",
						GuestSecretName: "test-guest-secret-name",
					},
				},
			},
			want: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
		},
		{
			name: "GuestConfig with new configuration format-remote_linux",
			input: &configpb.CredentialConfiguration{
				GuestConfigurations: &configpb.CredentialConfiguration_RemoteLinux{
					RemoteLinux: &configpb.CredentialConfiguration_GuestCredentialsRemoteLinux{
						ServerName:             "test-server-name",
						GuestUserName:          "test-guest-user-name",
						GuestPortNumber:        22,
						LinuxSshPrivateKeyPath: "test-linux-ssh-private-key-path",
					},
				},
			},
			want: &GuestConfig{
				ServerName:             "test-server-name",
				GuestUserName:          "test-guest-user-name",
				GuestPortNumber:        22,
				LinuxRemote:            true,
				LinuxSSHPrivateKeyPath: "test-linux-ssh-private-key-path",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GuestConfigFromCredential(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GuestConfigFromCredential(%v) returned an unexpected diff (-want +got): %v", tc.input, diff)
			}
		})
	}
}

func TestValidateConfigValues(t *testing.T) {
	testcases := []struct {
		name  string
		input *configpb.Configuration
		want  *configpb.Configuration
	}{
		{
			name: "values are all invalid",
			input: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{},
				MaxRetries:              -2,
			},
			want: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{
					GuestOsMetricsCollectionIntervalInSeconds: 3600,
					SqlMetricsCollectionIntervalInSeconds:     3600,
				},
				CollectionTimeoutSeconds: 10,
				MaxRetries:               3,
				RetryIntervalInSeconds:   3600,
			},
		},
		{
			name: "values are all valid",
			input: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{
					GuestOsMetricsCollectionIntervalInSeconds: 1,
					SqlMetricsCollectionIntervalInSeconds:     1,
				},
				CollectionTimeoutSeconds: 1,
				MaxRetries:               1,
				RetryIntervalInSeconds:   1,
			},
			want: &configpb.Configuration{
				CollectionConfiguration: &configpb.CollectionConfiguration{
					GuestOsMetricsCollectionIntervalInSeconds: 1,
					SqlMetricsCollectionIntervalInSeconds:     1,
				},
				CollectionTimeoutSeconds: 1,
				MaxRetries:               1,
				RetryIntervalInSeconds:   1,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateConfigValues(tc.input)
			if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
				t.Errorf("ValidateConfigValues() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestValidateCredCfgSQL(t *testing.T) {
	testcases := []struct {
		name             string
		inputSQLConfig   *SQLConfig
		inputGuestConfig *GuestConfig
		remote           bool
		windows          bool
		instanceID       string
		instanceName     string
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name: "success-local",
			inputSQLConfig: &SQLConfig{
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
		},
		{
			name: "success-remote",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			remote:       true,
			windows:      true,
		},
		{
			name: "failure-local-missing-user_name",
			inputSQLConfig: &SQLConfig{
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			wantErr:    true,
			wantErrMsg: `invalid value for "user_name"`,
		},
		{
			name: "failure-local-missing-secret_name",
			inputSQLConfig: &SQLConfig{
				Username:   "test-user-name",
				PortNumber: 1433,
			},
			wantErr:    true,
			wantErrMsg: `invalid value for "secret_name"`,
		},
		{
			name: "failure-local-missing-port_number",
			inputSQLConfig: &SQLConfig{
				Username:   "test-user-name",
				SecretName: "test-secret-name",
			},
			wantErr:    true,
			wantErrMsg: `invalid value for "port_number"`,
		},
		{
			name:             "failure-remote-win",
			inputSQLConfig:   &SQLConfig{},
			inputGuestConfig: &GuestConfig{},
			windows:          true,
			remote:           true,
			wantErr:          true,
			wantErrMsg:       `invalid value for "user_name" "secret_name" "port_number" "host" "server_name" "guest_user_name" "guest_secret_name" "instance_id" "instance_name"`,
		},
		{
			name: "failure-remote-missing host",
			inputSQLConfig: &SQLConfig{
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			remote:       true,
			windows:      true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			wantErr:      true,
			wantErrMsg:   `invalid value for "host"`,
		},
		{
			name: "failure-remote-linux-missing linux_ssh_private_key_path",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestPortNumber: 22,
			},
			remote:       true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			wantErr:      true,
			wantErrMsg:   `invalid value for "linux_ssh_private_key_path"`,
		},
		{
			name: "failure-remote-linux-missing guest_port_number",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:             "test-server-name",
				GuestUserName:          "test-guest-user-name",
				LinuxSSHPrivateKeyPath: "test-ssh-private-key-path",
			},
			remote:       true,
			wantErr:      true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			wantErrMsg:   `invalid value for "guest_port_number"`,
		},
		{
			name: "failure-remote-win-missing-instance_id",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			instanceName: "test-instance-name",
			remote:       true,
			windows:      true,
			wantErr:      true,
			wantErrMsg:   `invalid value for "instance_id"`,
		},
		{
			name: "failure-remote-missing-instance_name",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			remote:     true,
			windows:    true,
			instanceID: "test-instance-id",
			wantErr:    true,
			wantErrMsg: `invalid value for "instance_name"`,
		},
		{
			name: "success-remote-linux",
			inputSQLConfig: &SQLConfig{
				Host:       "test-host",
				Username:   "test-user-name",
				SecretName: "test-secret-name",
				PortNumber: 1433,
			},
			inputGuestConfig: &GuestConfig{
				ServerName:             "test-server-name",
				GuestUserName:          "test-guest-user-name",
				LinuxSSHPrivateKeyPath: "test-ssh-private-key-path",
				GuestPortNumber:        22,
			},
			remote:       true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCredCfgSQL(tc.remote, tc.windows, tc.inputSQLConfig, tc.inputGuestConfig, tc.instanceID, tc.instanceName)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("validateCredentialConfiguration() = %v, want error presence = %v", err, tc.wantErr)
			}
			if err != nil && err.Error() != tc.wantErrMsg {
				t.Errorf("validateCredentialConfiguration() = %v, want error message = %v", err, tc.wantErrMsg)
			}
		})
	}
}

func TestValidateCredCfgGuest(t *testing.T) {
	testcases := []struct {
		name             string
		inputGuestConfig *GuestConfig
		remote           bool
		windows          bool
		instanceID       string
		instanceName     string
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name:             "success-local",
			inputGuestConfig: &GuestConfig{},
		},
		{
			name: "success-remote-win",
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			remote:       true,
			windows:      true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
		},
		{
			name: "success-remote-linux",
			inputGuestConfig: &GuestConfig{
				ServerName:             "test-server-name",
				GuestUserName:          "test-guest-user-name",
				LinuxSSHPrivateKeyPath: "test-ssh-private-key-path",
				GuestPortNumber:        22,
			},
			remote:       true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
		},
		{
			name:             "failure-remote-win",
			inputGuestConfig: &GuestConfig{},
			windows:          true,
			remote:           true,
			wantErr:          true,
			wantErrMsg:       `invalid value for "server_name" "guest_user_name" "guest_secret_name" "instance_id" "instance_name"`,
		},
		{
			name: "failure-remote-linux-missing-linux_ssh_private_key_path",
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestPortNumber: 22,
			},
			remote:       true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			wantErr:      true,
			wantErrMsg:   `invalid value for "linux_ssh_private_key_path"`,
		},
		{
			name: "failure-remote-linux-missing-guest_port_number",
			inputGuestConfig: &GuestConfig{
				ServerName:             "test-server-name",
				GuestUserName:          "test-guest-user-name",
				LinuxSSHPrivateKeyPath: "test-ssh-private-key-path",
			},
			remote:       true,
			wantErr:      true,
			instanceID:   "test-instance-id",
			instanceName: "test-instance-name",
			wantErrMsg:   `invalid value for "guest_port_number"`,
		},
		{
			name: "failure-remote-missing-instance_id",
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			remote:       true,
			windows:      true,
			instanceName: "test-instance-name",
			wantErr:      true,
			wantErrMsg:   `invalid value for "instance_id"`,
		},
		{
			name: "failure-remote-missing-instance_name",
			inputGuestConfig: &GuestConfig{
				ServerName:      "test-server-name",
				GuestUserName:   "test-guest-user-name",
				GuestSecretName: "test-guest-secret-name",
			},
			remote:     true,
			windows:    true,
			instanceID: "test-instance-id",
			wantErr:    true,
			wantErrMsg: `invalid value for "instance_name"`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCredCfgGuest(tc.remote, tc.windows, tc.inputGuestConfig, tc.instanceID, tc.instanceName)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("validateCredentialConfiguration() = %v, want error presence = %v", err, tc.wantErr)
			}
			if err != nil && err.Error() != tc.wantErrMsg {
				t.Errorf("validateCredentialConfiguration() = %v, want error message = %v", err, tc.wantErrMsg)
			}
		})
	}
}
