# Changelog

All notable changes to Foghorn are listed here, newest first.

## Unreleased

### Fixed
- **The shipped client was still the broken one.** `dist\FoghornClient.exe` was
  never rebuilt after the 1.0.1 source fix, so the binary in the release, on the
  deployment share and inside the MSI still threw `Method not found` on every
  poll. Rebuilt from `client\src` with the compiler in Windows, and checked
  against a running server: it connects and the server records it.
- **`dist\FoghornClient.msi` installed that broken client.** The MSI keeps its
  own copy in an embedded cabinet, so replacing the exe in `dist\` did not
  change what it deployed. The cabinet has been rebuilt around the working
  client, and the MSI now unpacks to byte-for-byte what is in `dist\`.
- **MSI transforms were empty in effect.** `New-FoghornTransform.ps1` called
  `GenerateTransform` on the original database instead of the modified one, so
  the `.mst` described removing the settings rather than adding them. PCs
  deployed with MSI + MST got no server address or client key. Argument order
  now matches the Windows SDK's `WiGenXfm.vbs`.
- **`dist\FoghornClient.msi` was a stale build.** It reported version 1.0.0 and
  was missing both post-build tweaks listed under 1.0.1 (the `SERVERURL`
  condition and `SecureCustomProperties`). Rebuilt as 1.0.1 with them.
- **`-UseSystemProxy` on the transform did nothing** — the MSI never wrote
  `UseSystemProxy`. It now does, from the `USESYSTEMPROXY` property.
- **`BUILDING.md` told you to build the shipped client with Mono**, writing
  `mcs … -out:dist/FoghornClient.exe`. That is how the 1.0.1 client came to call
  a method .NET Framework does not have: `mcs` compiles against Mono's class
  library, so the build succeeds and the exe only fails on a real PC. The
  section now says to build on Windows with `client\build.cmd`, and keeps the
  Mono command only as a syntax check that writes to a throwaway path.

### Added
- `deploy/msi/build-msi.sh` builds the MSI, applies the post-build tweaks and
  checks the result, so a bare `wixl` build can't be shipped by mistake.
- *MSI deploy test* GitHub Actions workflow: installs the MSI on a real Windows
  runner (plain, with transform, repair, uninstall, command-line properties).
- *Client build test* GitHub Actions workflow. It rebuilds the client from
  `client\src` on a Windows runner, fails if `dist\FoghornClient.exe` is not
  that build, then starts the shipped exe against a real Foghorn server and
  fails unless the server records it as connected. 1.0.1 shipped a client whose
  source was fixed but whose binary was not rebuilt, and no check noticed; a
  compile-only check would not have either, because the source compiled fine.
- `client\Get-AssemblyContentHash.ps1`, which hashes an assembly with the build
  timestamp and MVID blanked out. `csc.exe` is not deterministic, so two builds
  of identical source never have the same SHA256; this gives a value that only
  changes when the code does.
- `deploy\msi\Update-MsiClient.ps1` puts the current `dist\FoghornClient.exe`
  into the MSI's embedded cabinet, checks the result — including unpacking the
  finished MSI and comparing what it would install — and refreshes
  `dist\SHA256SUMS.txt`. It uses `makecab` and the Windows Installer COM API,
  both part of Windows, so the MSI can be kept in step without WiX. Rebuilding
  the MSI from `FoghornClient.wxs` still needs `build-msi.sh` on Linux.

### Changed
- `client\build.cmd` writes `dist\FoghornClient.exe` directly instead of leaving
  the exe in `client\` to be copied by hand — the copy is what got forgotten in
  1.0.1. Pass a path to build somewhere else: `build.cmd C:\tmp\FoghornClient.exe`.
  It is also pinned to the 64-bit compiler, because the 32-bit one emits a
  different assembly from the same source.
- Linux server documentation: a section in the README and a full
  `INSTALL-SERVER.md` Appendix A (firewall, HTTPS, low ports, reverse proxy,
  upgrade, password reset, moving between Windows and Linux, removal).

## 1.0.1

### Fixed
- **Client could not connect on any Windows PC.** The client called
  `String.TrimEnd(char)`. That single-character overload is not part of .NET
  Framework at any version — it only exists on .NET Core 2.0 and later — so the
  call threw `Method not found: 'System.String System.String.TrimEnd(Char)'` on
  every poll, on every PC, no matter how up to date it was. The source was
  switched to the params-array overload, which has existed since .NET 2.0.

  An earlier version of this entry said only PCs with .NET Framework 4.0 were
  affected. That was wrong: every Windows PC was.

  Note that the binary shipped in 1.0.1 still had the fault — see Unreleased.

### Added
- **MSI + transform (`.mst`) deployment.** The MSI is now generic — no server
  address or client key baked in — and pairs with a per-site transform that
  supplies those. Standard Windows administration, works with Group Policy
  Software Installation, Intune, SCCM and PDQ.
- `deploy\msi\New-FoghornTransform.ps1` generates a transform on any Windows
  PC using the built-in `WindowsInstaller.Installer` COM API. No WiX or SDK
  needed.
- `DEPLOY-CLIENTS.md` now leads with the MSI + MST workflow. The start-up
  script method is Appendix B.

### Changed
- The MSI's registry-writing component is now conditional on `SERVERURL` being
  set. Installing without a transform (or without command-line properties) no
  longer writes empty values into `HKLM\SOFTWARE\Foghorn` that would mask
  Group Policy settings.
- `SERVERURL`, `CLIENTKEY` and `USESYSTEMPROXY` are declared in
  `SecureCustomProperties` so they survive a managed install.

## 1.0.0

First public release.

### Server
- Single self-contained Windows service; embedded web console; JSON-file
  storage; HTTP or HTTPS.
- PBKDF2-SHA256 passwords, forced first-run password change, login lock-out,
  CSRF protection, 12-hour sessions.
- API tokens.

### Alerts
- Three levels (Notice, Warning, Urgent), three styles (Corner, Centre, Full
  screen).
- Acknowledgement tracking, auto-close with countdown bar, sound, link
  button, late-arrival window, recall.

### Targeting
- Everyone, saved groups, or rules on computer name, username, OU, AD group
  and IP subnet. Live reach count while composing.

### Accounts
- Administrators and Senders. Senders can be limited to particular groups.

### Client
- .NET Framework 4.x WinForms; always-on-top without stealing focus;
  multi-monitor full-screen; DPI-aware.
- Settings from Group Policy (ADMX), local registry, ini file or command
  line. `--test` and `--status`.

### Deployment
- PowerShell installers for server and client. Watchdog scheduled task. ADMX
  Group Policy template. MSI. systemd unit for Linux.

### Docs
- README, install, deploy, user guide, security, troubleshooting, API,
  building.
