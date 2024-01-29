$ErrorActionPreference = 'Stop'
$INSTALL_DIR = "$Env:Programfiles\Google\google-cloud-sql-server-agent"
$LOGS_DIR = "$INSTALL_DIR\logs"
$LOG_FILE ="$LOGS_DIR\google-cloud-sql-server-agent-monitor.log"
$SVC_NAME = 'google-cloud-sql-server-agent'

function Log-Write {
  #.DESCRIPTION
  #  Writes to log file.
  param (
    [string] $log_message
  )
  Write-Host $log_message
  if (-not (Test-Path $LOGS_DIR)) {
    return
  }
  #(Write-EventLog -EntryType Info -Source $EVENT_LOG_NAME -LogName Application `
  # -Message $log_message -EventId 1111) | Out-Null
  $time_stamp = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
  $logFileSize = $(Get-Item $LOG_FILE -ErrorAction Ignore).Length/1kb
  if ($logFileSize -ge 1024) {
    Write-Host "Logfilesize: $logFileSize kb, rotating"
    Move-Item -Force $LOG_FILE "$LOG_FILE.1"
  }
  Add-Content -Value ("$time_stamp - $log_message") -path $LOG_FILE
}

try {
  Log-Write 'Sql server agent monitor job started'
  $status = $(Get-Service -Name $SVC_NAME -ErrorAction Ignore).Status
  if ($status -ne 'Running') {
    Log-Write "SQL Server Agent service is not running, status: $status"
    Restart-Service -Force $SVC_NAME
    Log-Write 'SQL Server Agent service restarted'
    Start-Sleep -Seconds 1.5
    $restart_status = $(Get-Service -Name $SVC_NAME -ErrorAction Ignore).Status
    if ($restart_status -ne 'Running') {
      Log-Write 'Restarting SQL Server Agent service was unsuccessful'
    }
  }
}
catch {
  Log-Write $_.Exception|Format-List -force | Out-String
  break
}
finally {
}
