<#
.SYNOPSIS
    Installs (or upgrades) the Foghorn alert server as a Windows service.

.DESCRIPTION
    Run this once, on the Windows machine that will host Foghorn, from an
    elevated PowerShell window:

        powershell -ExecutionPolicy Bypass -File .\Install-FoghornServer.ps1

    It copies foghorn-server.exe into Program Files, locks the data folder down
    to Administrators, registers the "Foghorn Alert Server" service, opens the
    Windows Firewall for it and prints the address and first-time sign-in.

    Running it again later upgrades the program and keeps all your data.

.PARAMETER Port
    TCP port for the web console and the desktop clients. Default 8080.
    Only used on a first install; afterwards edit config.json in the data folder.

.PARAMETER InstallDir
    Where the program goes. Default: C:\Program Files\Foghorn Server

.PARAMETER DataDir
    Where settings, history and logs live. Default: C:\ProgramData\Foghorn

.PARAMETER NoFirewallRule
    Skip creating the Windows Firewall rule (if you manage the firewall another way).
#>
[CmdletBinding()]
param(
    [int]$Port = 8080,
    [string]$InstallDir = (Join-Path $env:ProgramFiles 'Foghorn Server'),
    [string]$DataDir = (Join-Path $env:ProgramData 'Foghorn'),
    [switch]$NoFirewallRule
)

$ErrorActionPreference = 'Stop'
$ServiceName = 'FoghornServer'
$RuleName = 'Foghorn Alert Server'

function Say($text, $colour = 'Gray') { Write-Host $text -ForegroundColor $colour }

# --- must be elevated --------------------------------------------------------
$me = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $me.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Say 'This needs an elevated window. Right-click PowerShell, choose "Run as administrator", and run it again.' Red
    exit 1
}

# --- find the program --------------------------------------------------------
$source = @(
    (Join-Path $PSScriptRoot 'foghorn-server.exe'),
    (Join-Path $PSScriptRoot '..\dist\foghorn-server.exe')
) | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $source) {
    Say "Cannot find foghorn-server.exe next to this script or in ..\dist." Red
    exit 1
}
$source = (Resolve-Path $source).Path
$exe = Join-Path $InstallDir 'foghorn-server.exe'

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
$upgrade = [bool]$existing

Say ''
Say ($(if ($upgrade) { 'Upgrading Foghorn server' } else { 'Installing Foghorn server' })) Cyan

# --- stop the old one if present --------------------------------------------
if ($upgrade -and $existing.Status -ne 'Stopped') {
    Say '  stopping the running service...'
    Stop-Service -Name $ServiceName -Force
    (Get-Service $ServiceName).WaitForStatus('Stopped', '00:00:30')
}

# --- copy the program --------------------------------------------------------
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path $source -Destination $exe -Force
Unblock-File -Path $exe -ErrorAction SilentlyContinue
Say "  program:      $exe"

# --- data folder, readable by Administrators and SYSTEM only ------------------
# It holds password hashes and the client key, and ProgramData is normally
# readable by every user. SIDs are used so this works on non-English Windows.
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
& icacls.exe $DataDir /inheritance:r /grant:r '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-18:(OI)(CI)F' | Out-Null
Say "  data folder:  $DataDir  (Administrators only)"

# --- first install: choose the port ------------------------------------------
$configPath = Join-Path $DataDir 'config.json'
if (-not (Test-Path $configPath)) {
    $cfg = "{`r`n  `"listen`": `":$Port`",`r`n  `"tls_cert`": `"`",`r`n  `"tls_key`": `"`",`r`n  `"trust_proxy_headers`": false`r`n}`r`n"
    Set-Content -Path $configPath -Value $cfg -Encoding Ascii
} else {
    try {
        $listen = (Get-Content $configPath -Raw | ConvertFrom-Json).listen
        if ($listen -match ':(\d+)$') { $Port = [int]$Matches[1] }
    } catch { }
}

# --- service -----------------------------------------------------------------
if ($upgrade) {
    Start-Service -Name $ServiceName
} else {
    & $exe service install -data $DataDir
    if ($LASTEXITCODE -ne 0) { Say 'The service could not be installed. See the message above.' Red; exit 1 }
}

# --- firewall ----------------------------------------------------------------
if (-not $NoFirewallRule) {
    Get-NetFirewallRule -DisplayName $RuleName -ErrorAction SilentlyContinue | Remove-NetFirewallRule
    New-NetFirewallRule -DisplayName $RuleName -Direction Inbound -Action Allow -Program $exe `
        -Profile Domain, Private -Description 'Lets desktop clients and the web console reach Foghorn.' | Out-Null
    Say '  firewall:     inbound rule added (Domain and Private networks)'
}

# --- check it is answering ----------------------------------------------------
$url = "http://localhost:$Port/healthz"
$ok = $false
foreach ($i in 1..20) {
    try { $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 2; if ($r.StatusCode -eq 200) { $ok = $true; break } } catch { }
    Start-Sleep -Milliseconds 500
}
# If HTTPS is configured the plain-HTTP check fails even though all is well.
$running = (Get-Service $ServiceName).Status -eq 'Running'

Say ''
if ($ok -or $running) {
    $hostName = [System.Net.Dns]::GetHostName()
    Say 'Foghorn is running.' Green
    Say ''
    Say "  Web console:  http://${hostName}:$Port/" White
    $firstRun = Join-Path $DataDir 'FIRST-RUN.txt'
    if (Test-Path $firstRun) {
        Say ''
        Get-Content $firstRun | ForEach-Object { Say "  $_" White }
        Say ''
        Say '  Next: open the web console, sign in, choose your own password, then' Gray
        Say '  follow docs\DEPLOY-CLIENTS.md to put the client on your PCs.' Gray
    }
} else {
    Say "The service was installed but is not answering on port $Port." Red
    Say "Look at the end of $DataDir\foghorn.log - the usual cause is another program already using the port." Red
    Say "To use a different port: edit `"listen`" in $configPath, then run  Restart-Service $ServiceName" Red
    exit 1
}
