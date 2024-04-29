#!/bin/bash

# build the binary for windows
blaze build third_party/sqlserveragent/cmd/win:main --config=lexan
# copy the windows build to the bucket
gsutil cp blaze-bin/third_party/sqlserveragent/cmd/win/main gs://sql-server-agent-integration/$USER/google-cloud-sql-server-agent.exe
# build the binary for linux
blaze build third_party/sqlserveragent/cmd/linux:google_cloud_sql_server_agent
# copy the linux build to the bucket
gsutil cp blaze-bin/third_party/sqlserveragent/cmd/linux/google_cloud_sql_server_agent gs://sql-server-agent-integration/$USER/google-cloud-sql-server-agent
# run the integration test
blaze test --test_strategy=local third_party/sqlserveragent/automation_tests/integration_test:integration_test_google3
