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

// Package sqlcollector contains modules that collects rules from Sql server.
package sqlcollector

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

// SQLCollector is the interface of Collector
// which declares all funcs that needs to be implemented.
type SQLCollector interface {
	CollectMasterRules(context.Context, time.Duration) []internal.Details
}
