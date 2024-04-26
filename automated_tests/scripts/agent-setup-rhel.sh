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
    tee /etc/yum.repos.d/google-cloud-sql-server-agent.repo << EOM
[google-cloud-sql-server-agent]
name=Google Cloud Agent for SQL Server
baseurl=https://us-central1-yum.pkg.dev/projects/sql-server-solutions/google-cloud-sql-server-agent-el8-x86-64-unstable
enabled=1
gpgcheck=0
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
  else
    tee /etc/yum.repos.d/google-cloud-sql-server-agent.repo << EOM
[google-cloud-sql-server-agent]
name=Google Cloud Agent for SQL Server
baseurl=https://us-central1-yum.pkg.dev/projects/sql-server-solutions/google-cloud-sql-server-agent-el8-x86-64
enabled=1
gpgcheck=0
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
  fi
  yum -y install google-cloud-sql-server-agent
fi
echo "
$config
" > configuration.json
mv configuration.json /etc/google-cloud-sql-server-agent/configuration.json
if [ -e /var/log/google-cloud-sql-server-agent.log ]; then
  rm /var/log/google-cloud-sql-server-agent.log
fi
systemctl restart google-cloud-sql-server-agent
