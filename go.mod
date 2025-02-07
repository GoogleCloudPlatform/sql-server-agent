module github.com/GoogleCloudPlatform/sql-server-agent

go 1.23

toolchain go1.23.0

replace github.com/GoogleCloudPlatform/sql-server-agent/internal => ./internal

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd => ./cmd

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent => ./cmd/agent

replace github.com/GoogleCloudPlatform/sql-server-agent/cmd/agent/agentshared => ./cmd/agent/agentshared

replace github.com/GoogleCloudPlatform/sql-server-agent/protos => ./protos

replace github.com/GoogleCloudPlatform/sql-server-agent/protos/sqlserveragentconfig => ./protos/sqlserveragentconfig

require (
  cloud.google.com/go/secretmanager v1.14.4
  github.com/DATA-DOG/go-sqlmock v1.5.0
  github.com/GoogleCloudPlatform/workloadagentplatform/sharedlibraries v0.0.0-20250206170227-f24f60073850
  github.com/StackExchange/wmi v1.2.1
  github.com/cenkalti/backoff/v4 v4.3.0
  github.com/google/go-cmp v0.6.0
  github.com/jonboulle/clockwork v0.5.0
  github.com/kardianos/service v1.2.2
  github.com/microsoft/go-mssqldb v1.4.0
  go.uber.org/zap v1.27.0
  golang.org/x/crypto v0.32.0
  google.golang.org/api v0.220.0
  google.golang.org/protobuf v1.36.5
)

require (
  cloud.google.com/go v0.118.0 // indirect
  cloud.google.com/go/auth v0.14.1 // indirect
  cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
  cloud.google.com/go/compute/metadata v0.6.0 // indirect
  cloud.google.com/go/iam v1.3.1 // indirect
  cloud.google.com/go/logging v1.13.0 // indirect
  cloud.google.com/go/longrunning v0.6.4 // indirect
  github.com/BurntSushi/toml v0.3.1 // indirect
  github.com/GoogleCloudPlatform/workloadagentplatform/sharedprotos v0.0.0-20250206170227-f24f60073850 // indirect
  github.com/felixge/httpsnoop v1.0.4 // indirect
  github.com/go-logr/logr v1.4.2 // indirect
  github.com/go-logr/stdr v1.2.2 // indirect
  github.com/go-ole/go-ole v1.2.6 // indirect
  github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
  github.com/golang-sql/sqlexp v0.1.0 // indirect
  github.com/google/s2a-go v0.1.9 // indirect
  github.com/google/uuid v1.6.0 // indirect
  github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
  github.com/googleapis/gax-go/v2 v2.14.1 // indirect
  github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
  github.com/pkg/errors v0.9.1 // indirect
  go.opentelemetry.io/auto/sdk v1.1.0 // indirect
  go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.58.0 // indirect
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0 // indirect
  go.opentelemetry.io/otel v1.34.0 // indirect
  go.opentelemetry.io/otel/metric v1.34.0 // indirect
  go.opentelemetry.io/otel/trace v1.34.0 // indirect
  go.uber.org/multierr v1.10.0 // indirect
  golang.org/x/net v0.34.0 // indirect
  golang.org/x/oauth2 v0.26.0 // indirect
  golang.org/x/sync v0.10.0 // indirect
  golang.org/x/sys v0.29.0 // indirect
  golang.org/x/text v0.21.0 // indirect
  golang.org/x/time v0.9.0 // indirect
  google.golang.org/genproto v0.0.0-20250204164813-702378808489 // indirect
  google.golang.org/genproto/googleapis/api v0.0.0-20250204164813-702378808489 // indirect
  google.golang.org/genproto/googleapis/rpc v0.0.0-20250127172529-29210b9bc287 // indirect
  google.golang.org/grpc v1.70.0 // indirect
)
