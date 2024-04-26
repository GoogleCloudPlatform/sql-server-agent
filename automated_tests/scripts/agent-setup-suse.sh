#!/bin/bash

repo=$1
config=$2
gs_file_path=$3
if [ "$repo" = "google3" ]; then
  /usr/bin/gsutil cp $gs_file_path /usr/bin/google_cloud_sql_server_agent
  chmod 775 /usr/bin/google_cloud_sql_server_agent
  mkdir -p /etc/google-cloud-sql-server-agent
  /usr/bin/google_cloud_sql_server_agent --action=install
else
  if [ "$repo" = "unstable" ]; then
    zypper addrepo --refresh https://us-central1-yum.pkg.dev/projects/sql-server-solutions/google-cloud-sql-server-agent-sles15-x86-64-unstable google-cloud-sql-server-agent
  else
    zypper addrepo --refresh https://packages.cloud.google.com/yum/repos/google-cloud-sql-server-agent-sles15 google-cloud-sql-server-agent
  fi
  zypper --non-interactive install google-cloud-sql-server-agent
fi
echo "
$config
" > configuration.json
mv configuration.json /etc/google-cloud-sql-server-agent/configuration.json
if [ -e /var/log/google-cloud-sql-server-agent.log ]; then
  rm /var/log/google-cloud-sql-server-agent.log
fi
systemctl restart google-cloud-sql-server-agent
