<#
.SYNOPSIS
    Prints a SHA256 that identifies the *content* of a .NET assembly, ignoring
    the parts the compiler changes on every build.

.DESCRIPTION
    csc.exe is not deterministic: building the same source twice produces two
    different files. Exactly two regions differ -

      * the PE header's TimeDateStamp (when the build ran), and
      * the MVID, a fresh GUID the compiler stamps into the metadata.

    Zero both and identical source gives an identical hash, so CI can tell
    "dist\FoghornClient.exe was built from this source" from "someone changed
    the source and forgot to rebuild the exe".

    Two things still have to match for the hashes to agree, because the compiler
    records both in the assembly:

      * the output file name - compare FoghornClient.exe with FoghornClient.exe,
        in different folders, not with a build called something else, and
      * the compiler - the 32-bit and 64-bit csc.exe emit different assemblies
        from the same source, so build.cmd pins the 64-bit one.

    This is a staleness check, not a security check. Use SHA256SUMS.txt to
    verify a download has not been tampered with.
#>
[CmdletBinding()]
param([Parameter(Mandatory)][string]$Path)

$ErrorActionPreference = 'Stop'
$b = [System.IO.File]::ReadAllBytes((Resolve-Path $Path).Path)

function U32($o) { [BitConverter]::ToUInt32($b, $o) }
function U16($o) { [BitConverter]::ToUInt16($b, $o) }

if ($b.Length -lt 0x40 -or $b[0] -ne 0x4D -or $b[1] -ne 0x5A) { throw "$Path is not a PE file (no MZ header)." }
$pe = U32 0x3C
if ($b.Length -lt $pe + 24) { throw "$Path is truncated (bad e_lfanew)." }
if ((U32 $pe) -ne 0x00004550) { throw "$Path is not a PE file (no PE signature)." }

# --- COFF header: blank the build timestamp -------------------------------
for ($i = 0; $i -lt 4; $i++) { $b[$pe + 8 + $i] = 0 }

$numSections  = U16 ($pe + 6)
$sizeOptional = U16 ($pe + 20)
$opt          = $pe + 24
$magic        = U16 $opt
if ($magic -ne 0x10B -and $magic -ne 0x20B) { throw "$Path has an unrecognised optional header (magic 0x$('{0:X}' -f $magic))." }

# --- Optional header: blank the PE checksum (derived from the above) ------
for ($i = 0; $i -lt 4; $i++) { $b[$opt + 64 + $i] = 0 }

# --- Section table, so we can turn RVAs into file offsets -----------------
$sections = @()
$secStart = $opt + $sizeOptional
for ($i = 0; $i -lt $numSections; $i++) {
    $s = $secStart + ($i * 40)
    $sections += [pscustomobject]@{ VA = U32 ($s + 12); RawSize = U32 ($s + 16); RawPtr = U32 ($s + 20) }
}
function RvaToOffset($rva) {
    foreach ($s in $sections) {
        if ($rva -ge $s.VA -and $rva -lt ($s.VA + $s.RawSize)) { return $s.RawPtr + ($rva - $s.VA) }
    }
    throw "RVA 0x$('{0:X}' -f $rva) is not inside any section of $Path."
}

# --- CLI header (data directory 14) -> metadata root ----------------------
$ddStart = if ($magic -eq 0x20B) { $opt + 112 } else { $opt + 96 }
$cliRva  = U32 ($ddStart + (14 * 8))
if ($cliRva -eq 0) { throw "$Path is not a managed assembly (no CLI header)." }
$cli     = RvaToOffset $cliRva
$mdRoot  = RvaToOffset (U32 ($cli + 8))
if ((U32 $mdRoot) -ne 0x424A5342) { throw "$Path has no BSJB metadata signature." }

# --- Walk the stream headers to find #GUID, whose first GUID is the MVID --
$verLen   = U32 ($mdRoot + 12)
$p        = $mdRoot + 16 + $verLen + 4      # skip version string, then Flags(2)+Streams(2)
$nStreams = U16 ($mdRoot + 16 + $verLen + 2)
$mvid     = -1
for ($i = 0; $i -lt $nStreams; $i++) {
    $off = U32 $p; $p += 8
    $nameStart = $p
    while ($b[$p] -ne 0) { $p++ }
    $name = [Text.Encoding]::ASCII.GetString($b, $nameStart, $p - $nameStart)
    $p = $nameStart + ([Math]::Floor(($p - $nameStart) / 4) + 1) * 4   # name is padded to a 4-byte boundary
    if ($name -eq '#GUID') { $mvid = $mdRoot + $off; break }
}
if ($mvid -lt 0) { throw "$Path has no #GUID metadata stream." }
for ($i = 0; $i -lt 16; $i++) { $b[$mvid + $i] = 0 }

$sha = [System.Security.Cryptography.SHA256]::Create()
($sha.ComputeHash($b) | ForEach-Object { $_.ToString('x2') }) -join ''
