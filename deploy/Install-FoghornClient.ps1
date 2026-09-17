<#
.SYNOPSIS
    Installs (or upgrades) the Foghorn desktop client on this PC.

.DESCRIPTION
    Safe to run again and again: it only changes what needs changing, so it
    works as a Group Policy computer start-up script, from Intune/PDQ/SCCM, or
    by hand from an elevated window. Must run as an administrator or SYSTEM.

    What it does:
      1. Copies FoghornClient.exe to C:\Program Files\Foghorn
      2. Saves the server address and client key (if you pass them)
      3. Makes the client start whenever anyone logs on
      4. Adds a watchdog that brings the client back within 5 minutes if it is closed

    The client itself always runs as the logged-on user, never as an administrator.

.PARAMETER ServerUrl
    Address of your Foghorn server, e.g. http://foghorn01:8080
    Leave out if you set it with the Foghorn Group Policy template instead.

.PARAMETER ClientKey
    The client key from Settings in the web console.

.PARAMETER NoWatchdog
    Do not create the scheduled task that restarts the client if it is closed.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\Install-FoghornClient.ps1 -ServerUrl http://foghorn01:8080 -ClientKey 0123abcd...
#>
[CmdletBinding()]
param(
    [string]$ServerUrl,
    [string]$ClientKey,
    [string]$InstallDir = (Join-Path $env:ProgramFiles 'Foghorn'),
    [switch]$NoWatchdog
)

$ErrorActionPreference = 'Stop'
$TaskName = 'Foghorn Client Watchdog'
$LogFile = Join-Path $env:WINDIR 'Temp\FoghornClient-install.log'

function Note($text) {
    $line = '{0}  {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $text
    Write-Host $text
    try { Add-Content -Path $LogFile -Value $line } catch { }
}

try {
    $me = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $me.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'This must run as an administrator (or as SYSTEM, which is what a Group Policy start-up script uses).'
    }

    $source = Join-Path $PSScriptRoot 'FoghornClient.exe'
    if (-not (Test-Path $source)) { $source = Join-Path $PSScriptRoot '..\dist\FoghornClient.exe' }
    if (-not (Test-Path $source)) { throw "FoghornClient.exe is not next to this script ($PSScriptRoot)." }
    $target = Join-Path $InstallDir 'FoghornClient.exe'

    # --- 1. program files -----------------------------------------------------
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Get-ChildItem -Path $InstallDir -Filter '*.old' -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue

    $needsCopy = $true
    if (Test-Path $target) {
        $needsCopy = (Get-FileHash $source -Algorithm SHA256).Hash -ne (Get-FileHash $target -Algorithm SHA256).Hash
    }
    if ($needsCopy) {
        if (Test-Path $target) {
            # A running program cannot be overwritten, but it can be renamed out of the way.
            $aside = "$target.$([DateTime]::Now.Ticks).old"
            try { Rename-Item -Path $target -NewName (Split-Path $aside -Leaf) } catch { }
        }
        Copy-Item -Path $source -Destination $target -Force
        Unblock-File -Path $target -ErrorAction SilentlyContinue
        Note "Installed FoghornClient.exe to $InstallDir"
    }

    # --- 2. settings ------------------------------------------------------------
    if ($ServerUrl) {
        if ($ServerUrl -notmatch '^https?://') { throw "ServerUrl must start with http:// or https:// (got '$ServerUrl')." }
        $key = 'HKLM:\SOFTWARE\Foghorn'
        if (-not (Test-Path $key)) { New-Item -Path $key -Force | Out-Null }
        Set-ItemProperty -Path $key -Name 'ServerUrl' -Value $ServerUrl.TrimEnd('/')
        if ($ClientKey) { Set-ItemProperty -Path $key -Name 'ClientKey' -Value $ClientKey.Trim() }
    }

    # --- 3. start at every log-on ------------------------------------------------
    $run = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run'
    $wanted = '"{0}"' -f $target
    if ((Get-ItemProperty -Path $run -Name 'FoghornClient' -ErrorAction SilentlyContinue).FoghornClient -ne $wanted) {
        Set-ItemProperty -Path $run -Name 'FoghornClient' -Value $wanted
        Note 'Set to start at log-on'
    }

    # --- 4. watchdog ---------------------------------------------------------------
    # A task that runs as "whoever is logged on" (the built-in Users group), at
    # log-on and every 5 minutes after. While the client is running the task is
    # still "running", so the repeats are ignored; if a student ends the process
    # the next repeat starts it again.
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($NoWatchdog) {
        if ($task) { Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false; Note 'Removed watchdog task' }
    } elseif (-not $task -or $needsCopy) {
        $exeXml = [System.Security.SecurityElement]::Escape($target)
        $xml = @"
<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Starts the Foghorn desktop alert client for the logged-on user, and restarts it if it is closed.</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <Repetition>
        <Interval>PT5M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <GroupId>S-1-5-32-545</GroupId>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>$exeXml</Command>
    </Exec>
  </Actions>
</Task>
"@
        Register-ScheduledTask -TaskName $TaskName -Xml $xml -Force | Out-Null
        Note 'Registered watchdog task'
    }

    # --- start it now for anyone already logged on ---------------------------------
    if (-not $NoWatchdog) {
        try { Start-ScheduledTask -TaskName $TaskName } catch { }
    }

    Note 'Foghorn client is installed.'
    exit 0
}
catch {
    Note "FAILED: $($_.Exception.Message)"
    exit 1
}
