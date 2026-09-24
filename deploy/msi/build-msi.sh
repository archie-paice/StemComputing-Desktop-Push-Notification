#!/usr/bin/env bash
# Builds dist/FoghornClient.msi from deploy/msi/FoghornClient.wxs on Linux.
#
#   sudo apt install wixl msitools     # Debian/Ubuntu
#   deploy/msi/build-msi.sh            # run from anywhere
#
# wixl cannot express two things the MSI needs for managed deployment, so this
# script applies them after the build and then checks the result. Always build
# the MSI with this script - a plain `wixl` build is missing both tweaks.
#
#   1. Condition "SERVERURL" on the Settings component, so installing without a
#      transform writes nothing to HKLM\SOFTWARE\Foghorn (which would otherwise
#      mask the Group Policy settings).
#   2. SERVERURL, CLIENTKEY and USESYSTEMPROXY in SecureCustomProperties, so
#      they survive a managed (elevated / per-machine) install.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
wxs="$here/FoghornClient.wxs"
msi="$root/dist/FoghornClient.msi"

for tool in wixl msibuild msiinfo; do
  command -v "$tool" >/dev/null || { echo "Missing $tool - install the wixl and msitools packages." >&2; exit 1; }
done
[ -f "$root/dist/FoghornClient.exe" ] || { echo "dist/FoghornClient.exe not found - build the client first." >&2; exit 1; }

version="$(grep -o 'Version="[0-9.]*"' "$wxs" | head -1 | cut -d'"' -f2)"
echo "Building Foghorn Client MSI $version"

# wixl resolves Source= paths relative to the current directory.
( cd "$here" && wixl -a x64 -o "$msi" "$wxs" )

msibuild "$msi" -q "UPDATE Component SET Condition='SERVERURL' WHERE Component='Settings'"
msibuild "$msi" -q "UPDATE Property SET Value='WIX_DOWNGRADE_DETECTED;WIX_UPGRADE_DETECTED;SERVERURL;CLIENTKEY;USESYSTEMPROXY' WHERE Property='SecureCustomProperties'"

# ---- self-checks: fail the build rather than ship a broken MSI -------------
fail() { echo "CHECK FAILED: $*" >&2; exit 1; }
props="$(msiinfo export "$msi" Property)"
comps="$(msiinfo export "$msi" Component)"
regs="$(msiinfo export "$msi" Registry)"

grep -q "^ProductVersion	$version" <<<"$props"              || fail "ProductVersion is not $version"
grep -q "SecureCustomProperties	.*SERVERURL;CLIENTKEY;USESYSTEMPROXY" <<<"$props" || fail "SecureCustomProperties tweak missing"
grep -q "^Settings	.*	SERVERURL	" <<<"$comps"                   || fail "Settings component is not conditioned on SERVERURL"
for v in ServerUrl ClientKey UseSystemProxy; do
  grep -q "	$v	" <<<"$regs" || fail "registry value $v missing"
done

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
( cd "$tmp" && msiextract "$msi" >/dev/null )
cmp -s "$tmp/Foghorn/FoghornClient.exe" "$root/dist/FoghornClient.exe" || fail "embedded FoghornClient.exe differs from dist/"

# Keep the checksums file in step.
( cd "$root/dist" && sha256sum FoghornClient.exe FoghornClient.msi foghorn-server.exe foghorn-server-linux-amd64 > SHA256SUMS.txt )

echo "OK: $msi ($version) - checks passed, dist/SHA256SUMS.txt updated."
