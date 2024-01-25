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

// Package wlm contains types and functions to interact with WorkloadManager cloud APIs.
package wlm

import (
	"context"
	"fmt"

	"google.golang.org/api/option"
	workloadmanager "google.golang.org/api/workloadmanager/v1"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

const (
	basePath = "https://workloadmanager-datawarehouse.googleapis.com/"
)

// WorkloadManagerService the interface of WLM.
type WorkloadManagerService interface {
	SendRequest(string) (*workloadmanager.WriteInsightResponse, error)
	UpdateRequest(*workloadmanager.WriteInsightRequest)
}

// WLM struct which contains workloadmanager service.
type WLM struct {
	wlmService *workloadmanager.Service
	Request    *workloadmanager.WriteInsightRequest
}

// NewWorkloadManager creates new WLM and it return non-nil error if any error was caught.
func NewWorkloadManager(ctx context.Context) (*WLM, error) {
	wlm, err := workloadmanager.NewService(ctx, option.WithEndpoint(basePath))
	if err != nil {
		return nil, fmt.Errorf("%v error creating WLM client", err)
	}
	return &WLM{wlmService: wlm}, nil
}

// SendRequest sends request to workloadmanager.
func (wlm *WLM) SendRequest(location string) (*workloadmanager.WriteInsightResponse, error) {
	return wlm.wlmService.Projects.Locations.Insights.WriteInsight(location, wlm.Request).Do()
}

// UpdateRequest updates WLM request.
func (wlm *WLM) UpdateRequest(writeInsightRequest *workloadmanager.WriteInsightRequest) {
	wlm.Request = writeInsightRequest
}

// InitializeSQLServerValidation intializes and returns SqlserverValidation.
func InitializeSQLServerValidation(projectID, instance string) *workloadmanager.SqlserverValidation {
	return &workloadmanager.SqlserverValidation{
		AgentVersion: internal.AgentVersion,
		ProjectId:    projectID,
		Instance:     instance,
		ValidationDetails: []*workloadmanager.SqlserverValidationValidationDetail{
			&workloadmanager.SqlserverValidationValidationDetail{
				Type:    "SQLSERVER_VALIDATION_TYPE_UNSPECIFIED",
				Details: []*workloadmanager.SqlserverValidationDetails{},
			},
		},
	}
}

// InitializeWriteInsightRequest intializes and returns WriteInsightRequest.
func InitializeWriteInsightRequest(sqlservervalidation *workloadmanager.SqlserverValidation, instanceID string) *workloadmanager.WriteInsightRequest {
	return &workloadmanager.WriteInsightRequest{
		Insight: &workloadmanager.Insight{
			SqlserverValidation: sqlservervalidation,
			InstanceId:          instanceID,
		},
	}
}

// UpdateValidationDetails update ValidationDetails in SqlserverValidation.
func UpdateValidationDetails(sqlservervalidation *workloadmanager.SqlserverValidation, details []internal.Details) *workloadmanager.SqlserverValidation {
	sqlservervalidation.ValidationDetails = []*workloadmanager.SqlserverValidationValidationDetail{}
	for _, detail := range details {
		d := []*workloadmanager.SqlserverValidationDetails{}
		for _, f := range detail.Fields {
			d = append(d, &workloadmanager.SqlserverValidationDetails{
				Fields: f,
			})
		}
		sqlservervalidation.ValidationDetails = append(sqlservervalidation.ValidationDetails, &workloadmanager.SqlserverValidationValidationDetail{
			Type:    detail.Name,
			Details: d,
		})
	}
	return sqlservervalidation
}
