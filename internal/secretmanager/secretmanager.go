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

// Package secretmanager is the wrapper of google cloud secretmanager api.
package secretmanager

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretMgrInterface defines functions in the interface of secret manager.
type SecretMgrInterface interface {
	GetSecretValue(ctx context.Context, projectID, secretName string) (string, error)
	Close()
}

// Client struct.
type Client struct {
	client *secretmanager.Client
}

// NewClient create and return an instance of SecretManagerClient.
// Returns nil if there is an error during the NewClient.
func NewClient(ctx context.Context) (*Client, error) {
	// Create the client.
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

// GetSecretValue returns the latest version of given secret name from Secret Manager.
func (s *Client) GetSecretValue(ctx context.Context, projectID, secretName string) (string, error) {
	result, err := s.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", projectID, secretName, "latest"),
	})
	if err != nil {
		return "", err
	}

	payload := result.GetPayload()
	if payload == nil {
		return "", fmt.Errorf("empty secret value from secret manager")
	}

	return string(payload.GetData()), nil
}

// Close the secret manager client.
func (s *Client) Close() error {
	return s.client.Close()
}
