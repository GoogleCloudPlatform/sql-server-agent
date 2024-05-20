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

package instanceinfo

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	"github.com/GoogleCloudPlatform/sapagent/shared/gce/fake"
)

func TestGetDeviceTypeForLinux(t *testing.T) {
	testcases := []struct {
		name string
		want string
	}{
		{
			name: "testOther",
			want: "OTHER",
		},
		{
			name: "PERSISTENT",
			want: "PERSISTENT-SSD",
		},
		{
			name: "persistent",
			want: "OTHER",
		},
		{
			name: "SCRATCH",
			want: "LOCAL-SSD",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeviceType(tc.name)

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("DiskToDiskType() returned wrong result (-got +want):\n%s", diff)
			}
		})
	}
}

func TestAllDisks(t *testing.T) {
	tests := []struct {
		projectID  string
		zone       string
		instanceID string
		gceService *fake.TestGCE
		want       []*Disks
	}{
		{
			gceService: &fake.TestGCE{
				GetDiskResp: []*compute.Disk{{Type: "/some/path/device-type"}},
				GetDiskErr:  []error{nil},
				GetInstanceResp: []*compute.Instance{
					{
						MachineType:       "test-machine-type",
						CpuPlatform:       "test-cpu-platform",
						CreationTimestamp: "test-creation-timestamp",
						Disks: []*compute.AttachedDisk{
							{
								Source:     "/some/path/disk-name",
								DeviceName: "disk-device-name",
								Type:       "PERSISTENT",
							},
							{
								Source:     "",
								DeviceName: "disk-device-name",
								Type:       "SCRATCH",
							},
							{
								Source:     "",
								DeviceName: "disk-device-name",
								Type:       "TestOther",
							},
						},
						NetworkInterfaces: []*compute.NetworkInterface{
							{
								Name:      "network-name",
								Network:   "test-network",
								NetworkIP: "test-network-ip",
							},
						},
					},
				},
				GetInstanceErr: []error{nil},
				ListZoneOperationsResp: []*compute.OperationList{
					{
						Items: []*compute.Operation{
							{
								EndTime: "2022-08-23T12:00:01.000-04:00",
							},
							{
								EndTime: "2022-08-23T12:00:00.000-04:00",
							},
						},
					},
				},
				ListZoneOperationsErr: []error{nil},
			},
			want: []*Disks{
				&Disks{
					DeviceName: "disk-device-name",
					DiskType:   "PERSISTENT-SSD",
					Mapping:    "",
				},
				&Disks{
					DeviceName: "disk-device-name",
					DiskType:   "LOCAL-SSD",
					Mapping:    "",
				},
				&Disks{
					DeviceName: "disk-device-name",
					DiskType:   "OTHER",
					Mapping:    "",
				},
			},
		},
	}

	ctx := context.Background()
	for _, tc := range tests {
		r := NewReader(tc.gceService)
		r.gceService.GetInstance("", "", "")
		got, err := r.AllDisks(ctx, tc.projectID, tc.zone, tc.instanceID)
		if err != nil {
			t.Errorf("AllDisks(%v, %v, %v) returned an unexpected error: %v", tc.projectID, tc.zone, tc.instanceID, err)
			continue
		}

		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("AllDisks(%v, %v, %v) returned an unexpected diff (-want +got): %v", tc.projectID, tc.zone, tc.instanceID, diff)
		}
	}
}
