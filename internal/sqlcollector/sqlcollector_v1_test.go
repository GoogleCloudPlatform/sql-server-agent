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

package sqlcollector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/google/go-cmp/cmp"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal/agentstatus"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

var fakeCloudProperties = agentstatus.NewCloudProperties("testProjectID", "testZone", "testInstanceName", "testProjectNumber", "testImage")
var fakeAgentProperties = agentstatus.NewAgentProperties("testName", "testVersion", false)
var fakeUsageMetricsLogger = agentstatus.NewUsageMetricsLogger(fakeAgentProperties, fakeCloudProperties, clockwork.NewRealClock(), []string{})

func TestCollectMasterRules(t *testing.T) {
	testcases := []struct {
		name         string
		timeout      int32
		delay        int
		mockQueryRes []*sqlmock.Rows
		rule         []internal.MasterRuleStruct
		want         []internal.Details
		queryError   bool
	}{
		{
			name:    "success",
			timeout: 30,
			delay:   0,

			mockQueryRes: []*sqlmock.Rows{
				sqlmock.NewRows([]string{"col1", "col2"}).AddRow("row1", "val1"),
			},

			rule: []internal.MasterRuleStruct{
				{
					Name:  "testRule",
					Query: "testQuery",
					Fields: func(fields [][]any) []map[string]string {
						return []map[string]string{
							map[string]string{
								"col1": internal.HandleNilString(fields[0][0]),
								"col2": internal.HandleNilString(fields[0][1]),
							},
						}
					},
				},
			},
			want: []internal.Details{
				{
					Name: "testRule",
					Fields: []map[string]string{
						map[string]string{
							"col1": "row1",
							"col2": "val1",
						},
					},
				},
			},
		},
		{
			name:    "empty result returned when timeout",
			timeout: 3,
			delay:   4,
			mockQueryRes: []*sqlmock.Rows{
				sqlmock.NewRows([]string{"col1", "col2"}).AddRow("row1", "val1"),
			},
			rule: []internal.MasterRuleStruct{
				{
					Name:  "testRule",
					Query: "testQuery",
				},
			},
			want: []internal.Details{},
		},
		{
			name:    "empty result when rule.Fields() returned empty map",
			timeout: 3,
			mockQueryRes: []*sqlmock.Rows{
				sqlmock.NewRows([]string{"col1", "col2"}).AddRow("row1", "val1"),
			},
			rule: []internal.MasterRuleStruct{
				{
					Name:   "testRule",
					Query:  "testQuery",
					Fields: func(fields [][]any) []map[string]string { return []map[string]string{} },
				},
			},
			want: []internal.Details{
				{
					Name:   "testRule",
					Fields: []map[string]string{},
				},
			},
		},
		{
			name:    "error caught when sql query returns error",
			timeout: 3,
			delay:   0,
			rule: []internal.MasterRuleStruct{
				{
					Name:  "testRule",
					Query: "testQuery",
				},
			},
			want:       []internal.Details{},
			queryError: true,
		},
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	c := V1{
		dbConn:             db,
		usageMetricsLogger: fakeUsageMetricsLogger,
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			internal.MasterRules = test.rule

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(test.timeout)*time.Second)
			defer cancel()

			if test.queryError {
				mock.ExpectQuery(test.rule[0].Query).WillReturnError(errors.New("new error"))
			} else {
				for i := range test.mockQueryRes {
					mock.ExpectQuery(test.rule[0].Query).WillReturnRows(test.mockQueryRes[i]).WillDelayFor(time.Duration(test.delay) * time.Second)
				}
			}

			r := c.CollectMasterRules(ctx, time.Second)
			if diff := cmp.Diff(r, test.want); diff != "" {
				t.Errorf("CollectMasterRules returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestNewV1(t *testing.T) {
	testcases := []struct {
		name    string
		driver  string
		wantErr bool
	}{
		{
			name:   "success",
			driver: "sqlserver",
		},
		{
			name:    "error",
			driver:  "any",
			wantErr: true,
		},
	}

	for _, tc := range testcases {
		_, err := NewV1(tc.driver, "", true, fakeUsageMetricsLogger)
		if gotErr := err != nil; gotErr != tc.wantErr {
			t.Errorf("NewV1() = %v, want error presence = %v", err, tc.wantErr)
		}
	}
}

func TestClose(t *testing.T) {
	c, err := NewV1("sqlserver", "", true, fakeUsageMetricsLogger)
	if err != nil {
		t.Errorf("NewV1() = %v, want nil", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}
