$config_json=$args[0]
$repo_version=$args[1]
$g3_exe_file_path=$args[2]

if ($repo_version -eq 'google3') {
  gsutil cp $g3_exe_file_path 'C:\\Program Files\\Google\\google-cloud-sql-server-agent\\google-cloud-sql-server-agent.exe'
  Set-Content -Path 'C:\\Program Files\\Google\\google-cloud-sql-server-agent\\configuration.json' -Value @"
$config_json
"@ -Encoding ascii
  & 'C:\\Program Files\\Google\\google-cloud-sql-server-agent\\google-cloud-sql-server-agent.exe' --action=install
}
else {
  if ($repo_version -eq 'unstable') {
    googet addrepo google-cloud-sql-server-agent https://us-central1-googet.pkg.dev/projects/sql-server-solutions/repos/google-cloud-sql-server-agent-windows-x86-64-unstable
  }
  else {
    googet addrepo google-cloud-sql-server-agent https://packages.cloud.google.com/yuck/repos/google-cloud-sql-server-agent-windows
  }
  googet -noconfirm install google-cloud-sql-server-agent
  Set-Content -Path 'C:\\Program Files\\Google\\google-cloud-sql-server-agent\\configuration.json' -Value @"
$config_json
"@ -Encoding ascii
}

$status = $(Get-Service -Name 'google-cloud-sql-server-agent').Status
if ($status -eq 'Running') {
  Stop-Service -Name 'google-cloud-sql-server-agent'
  Remove-Item -Path 'C:\\ProgramData\\Google\\google-cloud-sql-server-agent\\logs\\google-cloud-sql-server-agent.log'
}
Start-Service -Name 'google-cloud-sql-server-agent'
