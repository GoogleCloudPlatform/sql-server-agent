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

// Package instanceinfo provides functionality for interfacing with the compute API.
package instanceinfo

import (
	"context"
	"strings"

	compute "google.golang.org/api/compute/v1"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
	"github.com/GoogleCloudPlatform/sql-server-agent/internal"
)

// Disks contains information about a device name and the disk type
type Disks struct {
	DeviceName string
	DiskType   string
	Mapping    string
}

type gceInterface interface {
	GetInstance(project, zone, instance string) (*compute.Instance, error)
}

// Reader handles the retrieval of instance properties from a compute client instance.
type Reader struct {
	gceService gceInterface
}

// NewReader instantiates a Reader with gceService
func NewReader(gceService gceInterface) *Reader {
	return &Reader{
		gceService: gceService,
	}
}

// New instantiates a Reader with default instance properties.
func New(gceService gceInterface) *Reader {
	return &Reader{
		gceService: gceService,
	}
}

// AllDisks returns all possible disks with data from compute instance call
func (r *Reader) AllDisks(ctx context.Context, projectID, zone, instanceID string) ([]*Disks, error) {
	instance, err := r.gceService.GetInstance(projectID, zone, instanceID)
	if err != nil {
		log.Logger.Errorw("Could not get instance info from compute API, Enable the Compute Viewer IAM role for the Service Account", "project",
			projectID, "zone", zone, "instanceid", instanceID)
		return nil, err
	}
	allDisks := make([]*Disks, 0)
	for _, disks := range instance.Disks {
		deviceName, diskType := disks.DeviceName, DeviceType(disks.Type)
		allDisks = append(allDisks, &Disks{deviceName, diskType, ""})
	}

	return allDisks, nil
}

// DeviceType returns a formatted device type for a given disk type and name.
// The returned device type will be formatted as: "LOCAL-SSD" or "PERSISTENT-SSD". "OTHER" if another disk type
func DeviceType(diskType string) string {
	if diskType == "SCRATCH" {
		return internal.LocalSSD.String()
	} else if strings.Contains(diskType, "PERSISTENT") {
		return internal.PersistentSSD.String()
	} else {
		return internal.Other.String()
	}
}
