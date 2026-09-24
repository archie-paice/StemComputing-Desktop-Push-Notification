# Changelog

All notable changes to Foghorn are listed here, newest first.

## 1.0.3

Credit where it is due. No functional changes — if 1.0.2 is working on your
estate, this only changes what Foghorn says about who wrote it.

### Added
- **Foghorn is credited to Archie Paice**, who designed and built it, with
  `hello@archiepaice.com` as the contact. It now appears in the places an
  administrator actually looks:
  - the client exe's file properties — *Company* and *Copyright* on the Details
    tab of `FoghornClient.exe`;
  - *Publisher* in Apps and Features, from the MSI's `Manufacturer`, together
    with a contact and a help link on the entry;
  - the client's `--status` and `--help` windows, so whoever is troubleshooting
    a PC can see it without going and finding the repository;
  - the Group Policy template, in the description shown by the Group Policy
    Management Editor;
  - the `LICENSE` copyright line and the README.

### Changed
- The client's `AssemblyVersion` and `AssemblyFileVersion` were still
  `1.0.0.0`, so the Details tab reported 1.0.0.0 even on the 1.0.2 build. They
  now read 1.0.3.0, matching `Program.Version`. This was going to wait for 2.0,
  but the credit above appears on that same properties dialog and it would have
  sat next to a version that was two releases out of date.

### Server
- Both server binaries have been **rebuilt**, so the web console credit is
  actually in what you deploy. The console is compiled into the binary with
  `go:embed`, so editing `server/web` changes nothing anyone runs until the
  binary is rebuilt — signing in now shows the credit.
- `dist\foghorn-server.exe` and `dist\foghorn-server-linux-amd64` are larger
  (5.9 MB → 7.8 MB, 5.7 MB → 7.5 MB). They were previously built with an older
  Go; these are built with Go 1.27.1. No functional change, and `go test ./...`
  passes.
- **The server build is now reproducible**, which it was not before. Go was
  stamping the git commit into the binary, so it changed with every commit and
  no check could confirm `dist\` matched the source — committing the binary was
  circular, since it recorded the revision before the one containing it. Builds
  now pass `-buildvcs=false` and set `CGO_ENABLED=0` explicitly (Go records that
  setting, so leaving it unset makes the output depend on whether the machine
  has a C compiler). Same source and same Go version now give a byte-identical
  binary anywhere.
- *Server build test* workflow: rebuilds both binaries on a pinned Go, fails if
  `dist\` is not that build, then runs the committed Linux server and checks the
  console it serves is byte-for-byte the files in `server/web`.

### Still to do
- `foghorn-server version` still prints 1.0.0, and `reset-password` still tells
  you to run the Windows-only `foghorn-server service start` even on Linux.
  Both were parked for 2.0; the toolchain to fix them is now in place.

## 1.0.2

This release exists because 1.0.1 did not work. The fix was in the source but
never in the binary, so every Windows PC still failed. If you deployed 1.0.1,
replace it with this.

### Fixed
- **The client now reports its own version.** `Program.Version` was still
  `1.0.0`, so even a correct client showed up as 1.0.0 in the web console and
  there was no way to tell a fixed PC from a broken one. The version is now
  checked against `FoghornClient.wxs` and the MSI on every pull request.
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

### Upgrading
- The MSI is 1.0.2 with a new ProductCode and the same UpgradeCode, so it
  replaces 1.0.0 *and* the broken 1.0.1 by itself: add it to the Group Policy
  package, and each PC swaps the old client for this one at its next restart.
  Keeping it at 1.0.1 would not have worked — Windows Installer sees a matching
  ProductCode and version and decides the product is already installed.
- The Foghorn **server** still reports 1.0.0. Its version lives in `server`,
  which needs the Go toolchain to rebuild; changing the source without
  rebuilding `dist\foghorn-server.exe` would repeat the mistake this release is
  about. Server and client versions are independent — a 1.0.0 server works with
  a 1.0.2 client.

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
