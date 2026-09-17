<#
.SYNOPSIS
    Removes the Foghorn desktop client from this PC. Run as an administrator or SYSTEM.
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\Uninstall-FoghornClient.ps1
#>
[CmdletBinding()]
param([string]$InstallDir = (Join-Path $env:ProgramFiles 'Foghorn'))
$ErrorActionPreference = 'SilentlyContinue'
Unregister-ScheduledTask -TaskName 'Foghorn Client Watchdog' -Confirm:$false
Get-Process -Name 'FoghornClient' | Stop-Process -Force
Remove-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run' -Name 'FoghornClient'
Remove-Item -Path 'HKLM:\SOFTWARE\Foghorn' -Recurse -Force
Start-Sleep -Seconds 1
Remove-Item -Path $InstallDir -Recurse -Force
Write-Host 'Foghorn client removed.'
exit 0
