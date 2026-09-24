<#
.SYNOPSIS
    Creates a Windows Installer transform (.mst) that carries the Foghorn
    server address and client key.

.DESCRIPTION
    An MSI Transform is the standard way to feed settings into an MSI when
    deploying it through Group Policy Software Installation, Intune, SCCM or
    PDQ. You keep one generic FoghornClient.msi and one MST per site (or per
    key rotation); the MSI never contains the key, so it is safe to store on a
    share alongside the transform.

    This script uses the Windows Installer COM API (WindowsInstaller.Installer)
    which is part of Windows itself - no WiX, SDK or extra tools needed.

    Must run on Windows, in a normal PowerShell window (no elevation required).

.PARAMETER MsiPath
    Path to the base FoghornClient.msi. Default: .\FoghornClient.msi

.PARAMETER MstPath
    Path to write the .mst to. Default: .\FoghornClient.mst

.PARAMETER ServerUrl
    Your Foghorn server address, e.g. http://foghorn01:8080

.PARAMETER ClientKey
    Client key from Settings in the Foghorn web console.

.PARAMETER UseSystemProxy
    Set if PCs can only reach the server through the Windows web proxy.

.EXAMPLE
    .\New-FoghornTransform.ps1 -ServerUrl http://foghorn01:8080 -ClientKey 3f9a1c...

.EXAMPLE
    .\New-FoghornTransform.ps1 -MsiPath \\srv\Deploy\Foghorn\FoghornClient.msi `
                               -MstPath \\srv\Deploy\Foghorn\NorthbrookCollege.mst `
                               -ServerUrl https://foghorn.college.local:8443 `
                               -ClientKey 3f9a1c...
#>
[CmdletBinding()]
param(
    [string]$MsiPath  = (Join-Path $PSScriptRoot 'FoghornClient.msi'),
    [string]$MstPath  = (Join-Path $PSScriptRoot 'FoghornClient.mst'),
    [Parameter(Mandatory)][string]$ServerUrl,
    [Parameter(Mandatory)][string]$ClientKey,
    [switch]$UseSystemProxy
)

$ErrorActionPreference = 'Stop'

if ($ServerUrl -notmatch '^https?://') { throw "ServerUrl must start with http:// or https:// (got '$ServerUrl')." }
if (-not (Test-Path $MsiPath))         { throw "Cannot find $MsiPath. Copy FoghornClient.msi next to this script or pass -MsiPath." }
$MsiPath = (Resolve-Path $MsiPath).Path
$MstPath = [System.IO.Path]::GetFullPath($MstPath)
if (Test-Path $MstPath) { Remove-Item $MstPath -Force }

$ServerUrl = $ServerUrl.TrimEnd('/')
$ClientKey = $ClientKey.Trim()

Write-Host "Base MSI:   $MsiPath"
Write-Host "Transform:  $MstPath"
Write-Host "Server:     $ServerUrl"
Write-Host "Client key: $($ClientKey.Substring(0,[Math]::Min(6,$ClientKey.Length)))..."

# We build a transform by opening the MSI twice: one as the "reference" state,
# one as the modified state, then asking the reference to write the delta as an
# MST. This is exactly what WiX's MakeMST does.
$installer = New-Object -ComObject WindowsInstaller.Installer

# 1 = msiOpenDatabaseModeReadOnly ; 2 = msiOpenDatabaseModeTransact
$refDb = $installer.OpenDatabase($MsiPath, 1)
# Copy the MSI so we don't touch the original
$workingCopy = [System.IO.Path]::GetTempFileName() + '.msi'
Copy-Item -Path $MsiPath -Destination $workingCopy -Force
$modDb = $installer.OpenDatabase($workingCopy, 2)

function Set-Property($db, $name, $value) {
    $view = $db.OpenView("SELECT * FROM Property WHERE Property = '$name'")
    $view.Execute($null)
    $rec = $view.Fetch()
    if ($rec) {
        # UPDATE
        $view2 = $db.OpenView("UPDATE Property SET Value = ? WHERE Property = '$name'")
        $u = $installer.CreateRecord(1); $u.StringData(1) = $value
        $view2.Execute($u); $view2.Close()
    } else {
        # INSERT
        $view2 = $db.OpenView("INSERT INTO Property (Property, Value) VALUES (?, ?)")
        $r = $installer.CreateRecord(2); $r.StringData(1) = $name; $r.StringData(2) = $value
        $view2.Execute($r); $view2.Close()
    }
    $view.Close()
}

Set-Property $modDb 'SERVERURL' $ServerUrl
Set-Property $modDb 'CLIENTKEY' $ClientKey
if ($UseSystemProxy) { Set-Property $modDb 'USESYSTEMPROXY' '1' }
$modDb.Commit()

# Generate the transform. GenerateTransform is called on the MODIFIED database
# and given the ORIGINAL as its reference: "the differences that turn the
# reference into me". (Called the other way round it writes a transform that
# deletes the rows we just added: it either fails to apply or does nothing, and the
# PCs install with no server address or key.) Same order as the Windows
# SDK sample WiGenXfm.vbs.
$modDb.GenerateTransform($refDb, $MstPath) | Out-Null
# 0, 0 = no error suppression and no validation checks: the transform is only
# Property rows, so it applies to this MSI and to later versions of it.
$modDb.CreateTransformSummaryInfo($refDb, $MstPath, 0, 0)

# Release the COM objects and delete the working copy
[void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($modDb)
[void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($refDb)
[void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($installer)
Remove-Item $workingCopy -Force -ErrorAction SilentlyContinue

if (-not (Test-Path $MstPath)) { throw "The transform was not created. This usually means the properties were already set to these values in the MSI." }
$size = (Get-Item $MstPath).Length
Write-Host ""
Write-Host "Wrote transform ($size bytes)." -ForegroundColor Green
Write-Host ""
Write-Host "Try it locally on a test PC (as administrator):"
Write-Host "  msiexec /i `"$MsiPath`" TRANSFORMS=`"$MstPath`" /qb"
Write-Host ""
Write-Host "Then check:"
Write-Host "  reg query HKLM\SOFTWARE\Foghorn"
Write-Host "  `"C:\Program Files\Foghorn\FoghornClient.exe`" --status"
