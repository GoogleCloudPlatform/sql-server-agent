module github.com/GoogleCloudPlatform/sql-server-agent

go 1.20

replace github.com/GoogleCloudPlatform/sql-server-agent/internal => ./internal

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd => ./cmd

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent => ./cmd/agent

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent/agentshared => ./cmd/agent/agentshared

replace github.com/GoogleCloudPlatform/sql-server-agent/protos => ./protos

replace github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig => ./protos/sqlserveragentconfig

require (
  cloud.google.com/go/secretmanager v1.11.1
  github.com/DATA-DOG/go-sqlmock v1.5.0
  github.com/GoogleCloudPlatform/sapagent v0.0.0-20240304141225-7c9b90912309
  github.com/StackExchange/wmi v1.2.1
  github.com/cenkalti/backoff/v4 v4.2.1
  github.com/google/go-cmp v0.6.0
  github.com/kardianos/service v1.2.2
  github.com/microsoft/go-mssqldb v1.4.0
  go.uber.org/zap v1.25.0
  golang.org/x/crypto v0.14.0
  google.golang.org/api v0.144.0
  google.golang.org/protobuf v1.31.0
)

require (
  cloud.google.com/go v0.110.7 // indirect
  cloud.google.com/go/compute v1.23.0 // indirect
  cloud.google.com/go/compute/metadata v0.2.3 // indirect
  cloud.google.com/go/iam v1.1.1 // indirect
  cloud.google.com/go/logging v1.7.0 // indirect
  cloud.google.com/go/longrunning v0.5.1 // indirect
  github.com/go-ole/go-ole v1.2.6 // indirect
  github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
  github.com/golang-sql/sqlexp v0.1.0 // indirect
  github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
  github.com/golang/protobuf v1.5.3 // indirect
  github.com/google/s2a-go v0.1.7 // indirect
  github.com/google/uuid v1.3.1 // indirect
  github.com/googleapis/enterprise-certificate-proxy v0.3.1 // indirect
  github.com/googleapis/gax-go/v2 v2.12.0 // indirect
  github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
  github.com/pkg/errors v0.9.1 // indirect
  go.opencensus.io v0.24.0 // indirect
  go.uber.org/multierr v1.10.0 // indirect
  golang.org/x/net v0.17.0 // indirect
  golang.org/x/oauth2 v0.12.0 // indirect
  golang.org/x/sync v0.3.0 // indirect
  golang.org/x/sys v0.13.0 // indirect
  golang.org/x/text v0.13.0 // indirect
  google.golang.org/appengine v1.6.7 // indirect
  google.golang.org/genproto v0.0.0-20230913181813-007df8e322eb // indirect
  google.golang.org/genproto/googleapis/api v0.0.0-20230913181813-007df8e322eb // indirect
  google.golang.org/genproto/googleapis/rpc v0.0.0-20230920204549-e6e6cdab5c13 // indirect
  google.golang.org/grpc v1.58.3 // indirect
  gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
