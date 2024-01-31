#!/bin/bash

#
# Build script that will get the module dependencies and build a linux binary.
# The google_cloud_sql_server_agent binary will be built into the buildoutput/ dir.
#

set -exu

echo "Starting the build process for the SQL Server Agent..."

# LINT.IfChange
echo "**************  Getting go 1.20"
wget -q https://go.dev/dl/go1.20.3.linux-amd64.tar.gz
# LINT.ThenChange(
#  //depot/google3/third_party/sqlserveragent/kokoro/build.sh,
#  //depot/google3/third_party/sqlserveragent/.github/workflows/go.yml
# )
mkdir -p /tmp/sqlserveragent
tar -C /tmp/sqlserveragent -xzf go1.20.3.linux-amd64.tar.gz
export GOROOT=/tmp/sqlserveragent/go
mkdir -p $GOROOT/.cache
mkdir -p $GOROOT/pkg/mod
export GOMODCACHE=$GOROOT/pkg/mod
export GOCACHE=$GOROOT/.cache
PATH=/tmp/sqlserveragent/go/bin:$PATH
go clean -modcache

echo "**************  Getting module dependencies"
go get -d ./...

echo "**************  Running all tests"
go test ./...

echo "**************  Building Linux binary"
mkdir -p buildoutput
env GOOS=linux GOARCH=amd64 go build -o buildoutput/google_cloud_sql_server_agent cmd/linux/main.go

echo "**************  Cleaning up"
rm -f go1.20.3.linux-amd64.tar.gz*
go clean -modcache
rm -fr /tmp/sqlserveragent

echo "**************  Finished building the SQL Server Agent, the google_cloud_sql_server_agent binary is available in the buildoutput directory"
