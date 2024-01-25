#Requires -Version 5
#Requires -RunAsAdministrator
#Requires -Modules ScheduledTasks
<#
.SYNOPSIS
  Google Cloud Agent for SQL installation script.
.DESCRIPTION
  This powershell script is used to install the Google Cloud Agent for SQL
  on the system and a Task Scheduler entry: google-cloud-sql-agent-monitor (runs every min),
  .
#>
$ErrorActionPreference = 'Stop'
$INSTALL_DIR = "$Env:Programfiles\Google\google-cloud-sql-server-agent"
$SVC_NAME = 'google-cloud-sql-server-agent'
# The google-cloud-sql-agent-service.exe is a Windows Service Wrapper for google-cloud-sql-server-agent.exe
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

function CreateItem-IfNotExists {
  param (
    [string] $PathToCreate,
    [string] $TypeToCreate
  )
  if (-not (Test-Path $PathToCreate)) {
    Log-Write "Creating folder/contents: $PathToCreate"
    New-Item -ItemType $TypeToCreate -Path $PathToCreate
  }
}

function RenameFile-IfNotExists {
  param (
    [string] $SourceFile,
    [string] $TargetFile
  )
  if (-not (Test-Path $TargetFile)) {
    Log-Write "Renaming [$SourceFile] to [$TargetFile]"
    Rename-Item -Path $SourceFile -NewName $TargetFile
  }
  else {
    Log-Write "Skipping file copy as the file [$TargetFile] already exists."
  }
}

function RemoveItem-IfExists {
  param (
    [string] $PathToRemove
  )
  if (Test-Path $PathToRemove) {
    Log-Write "Cleaning up prior folder/contents: $PathToRemove"
    # Left Overs, cleanup
    Remove-Item -Recurse -Force $PathToRemove
  }
}

function CreateInstall-Artifacts {
  # Using -Force flag will not complain if the folder already exists.
  CreateItem-IfNotExists $INSTALL_DIR 'Directory'
  CreateItem-IfNotExists $LOGS_DIR 'Directory'

  if (-not (Test-Path "$INSTALL_DIR\configuration.json") -And
    (Test-Path "$INSTALL_DIR\config.json")
  ) {
    RenameFile-IfNotExists "$INSTALL_DIR\config.json" "$INSTALL_DIR\configuration.json"
  }
  else {
    RenameFile-IfNotExists "$INSTALL_DIR\config-default.json" "$INSTALL_DIR\configuration.json"
  }

  RemoveItem-IfExists "$INSTALL_DIR\config-default.json"
}

function Update-Configuration {
  $date = Get-Date -Format 'MMddyyyy'
  $backup = "$INSTALL_DIR\config-$date.bak"
  Copy-Item "$INSTALL_DIR\configuration.json" $backup
  $save = $false

  $config = Get-Content $backup | ConvertFrom-Json
  foreach ($cred in $config.credential_configuration) {
    if ($cred.windows_authentication -ne $null) {
      $save = $true
      $cred.PSObject.Properties.Remove('windows_authentication')
    }

    if (![string]::IsNullOrEmpty($cred.domain)) {
      $save = $true
      $cred.user_name = '{0}\{1}' -f $cred.domain,$cred.user_name
      if ($cred.guest_user_name -ne $null) {
          $cred.guest_user_name = '{0}\{1}' -f $cred.domain,$cred.guest_user_name
      }
      $cred.PSObject.Properties.Remove('domain')
    }
    if ($cred.sql_configurations -eq $null) {
      $save = $true
      Add-Member -InputObject $cred -MemberType NoteProperty -Name 'sql_configurations' -Value @(
          $(New-Object PSObject -Property $([ordered]@{
              'host'        = $cred.host
              'user_name'   = $cred.user_name
              'secret_name' = $cred.secret_name
              'port_number' = $cred.port_number
          }))
      ) -Force
      if ([string]::isNullOrEmpty($config.remote_collection) -or ($config.remote_collection -eq $false)) {
        Add-Member -InputObject $cred -MemberType NoteProperty -Name 'local_collections' -Value $true -Force
      }
      elseif ([string]::isNullOrEmpty($cred.linux_remote) -or ($cred.linux_remote -eq $false)) {
        Add-Member -InputObject $cred -MemberType NoteProperty -Name 'remote_win' -Value $(New-Object PSObject -Property $([ordered]@{
                'server_name'       = $cred.server_name
                'guest_user_name'   = $cred.guest_user_name
                'guest_secret_name' = $cred.guest_secret_name
                })
        ) -Force
      }
      else {
        Add-Member -InputObject $cred -MemberType NoteProperty -Name 'remote_linux' -Value $(New-Object PSObject -Property $([ordered]@{
                    'server_name'                = $cred.server_name
                    'guest_user_name'            = $cred.guest_user_name
                    'guest_port_number'          = $cred.guest_port_number
                    'linux_ssh_private_key_path' = $cred.linux_ssh_private_key_path
                })
        ) -Force
      }
      $cred.PSObject.Properties.Remove('host')
      $cred.PSObject.Properties.Remove('user_name')
      $cred.PSObject.Properties.Remove('secret_name')
      $cred.PSObject.Properties.Remove('port_number')
      $cred.PSObject.Properties.Remove('server_name')
      $cred.PSObject.Properties.Remove('guest_user_name')
      $cred.PSObject.Properties.Remove('guest_secret_name')
      $cred.PSObject.Properties.Remove('guest_port_number')
      $cred.PSObject.Properties.Remove('linux_remote')
      $cred.PSObject.Properties.Remove('linux_ssh_private_key_path')
    }
  }

  if ($save) {
    $config | ConvertTo-Json -Depth 4 | Out-File "$INSTALL_DIR\configuration.json" -Encoding ASCII
  }
  else {
    RemoveItem-IfExists $backup
  }
}

function ConfigureAgentWindows-Service {
  if ($(Get-Service -Name $SVC_NAME -ErrorAction SilentlyContinue).Status) {
    & "$INSTALL_DIR\$SVC_NAME_EXE" --action=uninstall
  }
  & "$INSTALL_DIR\$SVC_NAME_EXE" --action=install
}

function AddMonitor-Task {
  if ($(Get-ScheduledTask $MONITOR_TASK -ErrorAction Ignore).TaskName) {
     Log-Write "Scheduled task exists: $MONITOR_TASK"
     Unregister-ScheduledTask -TaskName $MONITOR_TASK -Confirm:$false
  }
  Log-Write "Adding scheduled task: $MONITOR_TASK"

  $action = New-ScheduledTaskAction `
    -Execute 'Powershell.exe' `
    -Argument "-File `"$INSTALL_DIR\google-cloud-sql-server-agent-monitor.ps1`" -WindowStyle Hidden" `
    -WorkingDirectory $INSTALL_DIR
  $trigger = New-ScheduledTaskTrigger `
      -Once `
      -At (Get-Date) `
      -RepetitionInterval (New-TimeSpan -Minutes 10) `
      -RepetitionDuration (New-TimeSpan -Days (365 * 20))
  Register-ScheduledTask -Action $action -Trigger $trigger `
    -TaskName $MONITOR_TASK `
    -Description $MONITOR_TASK -User 'System'
  Log-Write "Added scheduled task: $MONITOR_TASK"
}

function  StopService-AndTasks {
  if ($(Get-ScheduledTask $MONITOR_TASK -ErrorAction Ignore).TaskName) {
    Disable-ScheduledTask $MONITOR_TASK
  }

  $service = Get-Service -Name $SVC_NAME -ErrorAction Ignore

  if ($service.Status -and $service.Status -ne 'Stopped') {
    Stop-Service $SVC_NAME
    $service.WaitForStatus('Stopped', (New-TimeSpan -Minutes 5))
  }
}

function  StartService-AndTasks {
  Start-Service $SVC_NAME  -ErrorAction Ignore
  Enable-ScheduledTask $MONITOR_TASK
}

$Success = $false
$Processing=$false
try {
  Log-Write 'Installing the Google Cloud Agent for SQL'
  CreateInstall-Artifacts
  $Processing = $true;

  Log-Write 'Updating the configuration'
  Update-Configuration

  Log-Write 'Stopping agent services...'
  StopService-AndTasks
  Log-Write 'Stopped agent services'

  Log-Write 'Configuring Windows service...'
  ConfigureAgentWindows-Service
  Log-Write 'Windows service configured'

  Log-Write 'Adding monitor task...'
  AddMonitor-Task
  Log-Write 'Monitor task added'

  Log-Write 'Starting agent services...'
  StartService-AndTasks
  Log-Write 'Started agent services'

  $Success = $true
  Log-Write 'Successuflly installed the Google Cloud Agent for SQL'
}
catch {
  Log-Write $_.Exception|Format-List -force | Out-String
  break
}
Finally {
  # Try to start service and tasks again to make sure we are not leaving things inconsistent.
  try {
    if ($Processing -and !$Success) {
      StartService-AndTasks
    }
  }
  Finally {
  }
}
