$date = Get-Date -Format 'yyyyMMdd'
$logPath = $env:ProgramData + '\Google\google-cloud-sql-server-agent\logs\google-cloud-sql-server-agent.log'
$instance_name = (Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/name')
$continue = $true
while ($continue) {
    gsutil -h 'Content-Type:text/x-log' cp $logPath gs://sql-server-agent-soak-test/$date/$instance_name.log
    Start-Sleep 3600
}
