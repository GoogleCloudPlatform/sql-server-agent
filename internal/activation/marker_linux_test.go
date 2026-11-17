/*
Copyright 2026 Google LLC

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

package activation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMarker(t *testing.T) {
	p := filepath.Join(t.TempDir(), "google-cloud-sql-server-agent.activated")
	if err := writeMarker(p); err != nil {
		t.Fatalf("writeMarker(%q) returned unexpected error: %v", p, err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatalf("os.Lstat(%q) returned unexpected error: %v", p, err)
	}
	if !fi.Mode().IsRegular() {
		t.Errorf("writeMarker(%q) created mode %v, want a regular file", p, fi.Mode())
	}
	if got, want := fi.Mode().Perm(), os.FileMode(0600); got != want {
		t.Errorf("writeMarker(%q) created file with perm %v, want %v", p, got, want)
	}
}

func TestWriteMarkerRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	content := []byte("do not truncate")
	if err := os.WriteFile(target, content, 0600); err != nil {
		t.Fatalf("os.WriteFile(%q) returned unexpected error: %v", target, err)
	}
	link := filepath.Join(dir, "google-cloud-sql-server-agent.activated")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("os.Symlink(%q, %q) returned unexpected error: %v", target, link, err)
	}

	if err := writeMarker(link); err == nil {
		t.Errorf("writeMarker(%q) succeeded through a symlink, want error", link)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) returned unexpected error: %v", target, err)
	}
	if string(got) != string(content) {
		t.Errorf("symlink target content = %q, want %q (unchanged)", got, content)
	}
}
