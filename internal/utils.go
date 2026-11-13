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

// Package internal provides data structures and functions for collecting SQL Server information.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/commandlineexecutor"
	"github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries/log"
)

const (
	// ExperimentalMode .
	ExperimentalMode = true

	

	// AgentVersion is the version of the agent.
	AgentVersion = `1.3`
	

	// ServiceName .
	ServiceName = "google-cloud-sql-server-agent"

	// AgentUsageLogPrefix .
	AgentUsageLogPrefix = "wlm-sqlserver-eng"
)

// DiskTypeEnum enum used for disktypes to keep linux and windows collection consistent .
type DiskTypeEnum int

const (
	// LocalSSD - local disk
	LocalSSD DiskTypeEnum = iota
	// PersistentSSD - persistent disk
	PersistentSSD
	// Other - not local or persistent disk but still a valid disk type
	Other
)

func (disk DiskTypeEnum) String() string {
	return []string{"LOCAL-SSD", "PERSISTENT-SSD", "OTHER"}[disk]
}

func convertHexStringToBoolean(value string) (bool, error) {
	value = strings.TrimSpace(strings.Replace(value, "0x", "", -1))
	output, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		return false, err
	}
	return output == 1, nil
}

// HandleNilString converts generic string to the desired string output,
// or returns 'unknown' if desired type if nil.
func HandleNilString(data any) string {
	if data == nil {
		return "unknown"
	}
	return fmt.Sprintf("%v", data.(string))
}

// HandleNilInt converts generic int64 to desired string output,
// or returns 'unknown' if desired type if nil.
func HandleNilInt(data any) string {
	if data == nil {
		return "unknown"
	}
	// The passed in data might not be int64 so we need to handle the conversion from
	// all possible integer types to string.
	res, err := integerToString(data)
	if err != nil {
		log.Logger.Error(err)
		return "unknown"
	}

	return res
}

// HandleNilFloat64 converts generic float64 to desired string output,
// or returns 'unknown' if desired type if nil.
func HandleNilFloat64(data any) string {
	if data == nil {
		return "unknown"
	}
	return fmt.Sprintf("%f", data.(float64))
}

// HandleNilBool converts generic bool to desired string output,
// or returns 'unknown' if desired type if nil.
func HandleNilBool(data any) string {
	if data == nil {
		return "unknown"
	}
	return fmt.Sprintf("%v", data.(bool))
}

// SaveToFile saves data to given path.
func SaveToFile(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// PrettyStruct converts the passed in struct into a pretty json format.
func PrettyStruct(data any) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// CommandLineExecutorWrapper executes a windows or linux command with arguments given
func CommandLineExecutorWrapper(ctx context.Context, executable string, argsToSplit string, exec commandlineexecutor.Execute) (string, error) {
	result := exec(ctx, commandlineexecutor.Params{
		Executable:  executable,
		ArgsToSplit: argsToSplit,
	})
	if result.Error != nil {
		return "", fmt.Errorf("Error when running CommandLineExecutor: %s", result.StdErr)
	}
	return strings.TrimSuffix(result.StdOut, "\n"), nil
}

// safeLinuxPathRegex is a strict allow-list for absolute Linux file paths. It intentionally excludes
// shell metacharacters (quotes, backticks, $, ;, |, &, <, >, whitespace, glob characters, etc.).
var safeLinuxPathRegex = regexp.MustCompile(`^/[A-Za-z0-9_./-]*$`)

// IsSafeLinuxPath reports whether path is an absolute Linux path consisting only of characters
// that are safe to pass to commands such as find and df, including over a remote shell.
func IsSafeLinuxPath(path string) bool {
	return safeLinuxPathRegex.MatchString(path)
}

// commandLineExecutorArgs executes the given executable with an explicit argument list
// (no shell interpretation).
func commandLineExecutorArgs(ctx context.Context, executable string, args []string, exec commandlineexecutor.Execute) (string, error) {
	result := exec(ctx, commandlineexecutor.Params{
		Executable: executable,
		Args:       args,
	})
	if result.Error != nil {
		return "", fmt.Errorf("Error when running CommandLineExecutor: %s", result.StdErr)
	}
	return strings.TrimSuffix(result.StdOut, "\n"), nil
}

// firstLine returns the first non-empty line of s with surrounding whitespace trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// lastLine returns the last non-empty line of s with surrounding whitespace trimmed.
func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// GetPhysicalDriveFromPath gets the physical drive associated with a file path for linux and windows env
func GetPhysicalDriveFromPath(ctx context.Context, path string, windows bool, exec commandlineexecutor.Execute) string {

	if path == "" {
		return "unknown"
	} else if windows {
		mapping := strings.Split(path, `:`)
		if len(mapping) <= 1 {
			log.Logger.Warn("Couldn't find windows drive associated with the physical path name.")
			return "unknown"
		}
		return mapping[0]
	}

	// The path originates from SQL Server (sys.master_files.physical_name) and must be treated as
	// untrusted input. Validate it against a strict allow-list and never pass it through a shell.
	if !IsSafeLinuxPath(path) {
		log.Logger.Warnw("Physical path contains disallowed characters; skipping physical drive lookup.", "path", path)
		return "unknown"
	}

	dir, filename := filepath.Split(path)
	filePathOut, filePathErr := commandLineExecutorArgs(ctx, "find", []string{dir, "-type", "f", "-iname", filename, "-print"}, exec)
	if filePathErr != nil {
		log.Logger.Warn(filePathErr)
		return "unknown"
	}
	filePath := firstLine(filePathOut)
	if !IsSafeLinuxPath(filePath) {
		log.Logger.Warnw("Resolved file path contains disallowed characters; skipping physical drive lookup.", "path", filePath)
		return "unknown"
	}

	dfOut, physicalPathErr := commandLineExecutorArgs(ctx, "df", []string{"--output=target", filePath}, exec)
	if physicalPathErr != nil {
		log.Logger.Warn(physicalPathErr)
		return "unknown"
	}
	physicalPathMount := lastLine(dfOut)

	resultMount, mountErr := CommandLineExecutorWrapper(ctx, "/bin/sh", " -c ' mount |grep sd'", exec)
	if mountErr != nil {
		log.Logger.Warn(mountErr)
		return "unknown"
	}

	allMounts := strings.TrimSuffix(resultMount, "\n")
	physicalDriveHelper := regexp.MustCompile(` `+regexp.QuoteMeta(physicalPathMount)+` `).Split(allMounts, -1)

	physicalDrives := []string{}
	for i := 0; i < len(physicalDriveHelper)-1; i++ {
		splitStr := regexp.MustCompile("\n| |/").Split(physicalDriveHelper[i], -1)
		physicalDrives = append(physicalDrives, splitStr[len(splitStr)-2])
	}
	physicalDrive := strings.Join(physicalDrives, ", ")

	if physicalDrive == "" {
		return "unknown"
	}
	return physicalDrive
}

// integerToString converts any valid integer type to a string representation.
func integerToString(num any) (string, error) {
	switch v := num.(type) {
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case []uint8:
		return string([]uint8(v)), nil
	default:
		return "", fmt.Errorf("unsupported number type: %T", num)
	}
}
