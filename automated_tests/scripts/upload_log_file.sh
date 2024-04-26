#!/bin/bash

date=$(date "+%Y%m%d")
log_path="/var/log/google-cloud-sql-server-agent.log"
instance_name=$(curl "http://metadata.google.internal/computeMetadata/v1/instance/name" -H "Metadata-Flavor: Google")
while true; do
    gsutil -h "Content-Type:text/x-log" cp $log_path gs://sql-server-agent-soak-test/$date/$instance_name.log
    sleep 3600
done

