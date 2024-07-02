#Requires -Version 5
#Requires -RunAsAdministrator
#Requires -Modules ScheduledTasks
<#
.SYNOPSIS
  Google Cloud Agent for SQL uninstall script.
.DESCRIPTION
  This powershell script is used to uninstall the Google Cloud Agent for SQL
  on the system and remove a Task Scheduler entry: google-cloud-sql-agent-monitor,
  .
#>
$ErrorActionPreference = 'Stop'
$INSTALL_DIR = 'C:\Program Files\Google\google-cloud-sql-server-agent'
$SVC_NAME = 'google-cloud-sql-server-agent'
$SVC_NAME_EXE = 'google-cloud-sql-server-agent.exe'
$MONITOR_TASK = 'google-cloud-sql-server-agent-monitor'
$LOGS_DIR = "$INSTALL_DIR\logs"
$LOG_FILE ="$LOGS_DIR\google-cloud-sql-server-agent-install.log"


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
  $time_stamp = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
  $logFileSize = $(Get-Item $LOG_FILE -ErrorAction Ignore).Length/1kb
  if ($logFileSize -ge 1024) {
    Write-Host "Logfilesize: $logFileSize kb, rotating"
    Move-Item -Force $LOG_FILE "$LOG_FILE.1"
  }
  Add-Content -Value ("$time_stamp - $log_message") -path $LOG_FILE
}


try {
  # stop the service / tasks and remove them
  Log-Write "attempting to uninstall $SVC_NAME service"
  if ($(Get-ScheduledTask $MONITOR_TASK -ErrorAction Ignore).TaskName) {
    Disable-ScheduledTask $MONITOR_TASK
    Unregister-ScheduledTask -TaskName $MONITOR_TASK -Confirm:$false
  }
  if ($(Get-Service -Name $SVC_NAME -ErrorAction Ignore).Status) {
    Stop-Service $SVC_NAME
    if ($(Get-Service -Name $SVC_NAME -ErrorAction SilentlyContinue).Status) {
      & $INSTALL_DIR\$SVC_NAME_EXE --action=uninstall
    }
    else {
      Log-Write 'uninstall failed, unable to stop service before uninstalling'
    }
  }

  # remove the agent directory
  if (Test-Path $INSTALL_DIR) {
    Log-Write 'attempting to delete path for install directory $INSTALL_DIR'
    Remove-Item -Recurse -Force $INSTALL_DIR
  }
}
catch {
  Log-Write $_.Exception|Format-List -force | Out-String
  break
}
