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

package guestcollector

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

// GuestCollector interface.
type GuestCollector interface {
	CollectGuestRules(context.Context, time.Duration) internal.Details
}

// allOSFields are all expected fields in OS collection in collection order.
// LocalSSDRule needs to be collected before DataDiskAllocatinUnitsRule for linux.
var allOSFields = []string{
	internal.PowerProfileSettingRule,
	internal.LocalSSDRule,
	internal.DataDiskAllocationUnitsRule,
	internal.GCBDRAgentRunning,
}

// CollectionOSFields returns all expected fields in OS collection
func CollectionOSFields() []string { return append([]string(nil), allOSFields...) }

// MarkUnknownOsFields checks the collected os fields; if nil or missing, then the data is marked as unknown
func MarkUnknownOsFields(details *[]internal.Details) error {
	if len(*details) != 1 {
		return fmt.Errorf("CheckOSCollectedMetrics details should have only 1 field for OS collection, got %d", len(*details))
	}
	detail := (*details)[0]
	if detail.Name != "OS" {
		return fmt.Errorf("CheckOSCollectedMetrics details.name should be collecting for OS, got %s", detail.Name)
	}
	if len(detail.Fields) > 1 {
		return fmt.Errorf("CheckOSCollectedMetrics details.fields should have 1 field in OS collection, got %d", len(detail.Fields))
	}

	if len(detail.Fields) == 0 {
		fields := map[string]string{
			internal.PowerProfileSettingRule:     "unknown",
			internal.LocalSSDRule:                "unknown",
			internal.DataDiskAllocationUnitsRule: "unknown",
			internal.GCBDRAgentRunning:           "unknown",
		}
		(*details)[0].Fields = append((*details)[0].Fields, fields)
		return nil
	}

	// for os collection, details only has one element and details.Fields only has one element
	// sql collections is different as there can be multiple details and multiple details.Fields
	for _, field := range CollectionOSFields() {
		_, ok := detail.Fields[0][field]
		if !ok {
			(*details)[0].Fields[0][field] = "unknown"
		}
	}
	return nil
}
