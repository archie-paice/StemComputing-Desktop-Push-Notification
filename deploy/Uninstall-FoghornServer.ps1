<#
.SYNOPSIS
    Removes the Foghorn server. Your data is kept unless you add -RemoveData.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\Uninstall-FoghornServer.ps1
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\Uninstall-FoghornServer.ps1 -RemoveData
#>
[CmdletBinding()]
param(
    [string]$InstallDir = (Join-Path $env:ProgramFiles 'Foghorn Server'),
    [string]$DataDir = (Join-Path $env:ProgramData 'Foghorn'),
    [switch]$RemoveData
)
$ErrorActionPreference = 'Stop'
$me = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $me.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host 'Run this from an elevated (Run as administrator) PowerShell window.' -ForegroundColor Red; exit 1
}
$exe = Join-Path $InstallDir 'foghorn-server.exe'
if (Get-Service -Name 'FoghornServer' -ErrorAction SilentlyContinue) {
    if (Test-Path $exe) { & $exe service uninstall }
    else { Stop-Service FoghornServer -Force -ErrorAction SilentlyContinue; & sc.exe delete FoghornServer | Out-Null }
}
Get-NetFirewallRule -DisplayName 'Foghorn Alert Server' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Start-Sleep -Seconds 1
if (Test-Path $InstallDir) { Remove-Item $InstallDir -Recurse -Force }
if ($RemoveData -and (Test-Path $DataDir)) { Remove-Item $DataDir -Recurse -Force; Write-Host "Removed data folder $DataDir" }
elseif (Test-Path $DataDir) { Write-Host "Kept your data in $DataDir (add -RemoveData to delete it too)." }
Write-Host 'Foghorn server removed.' -ForegroundColor Green
