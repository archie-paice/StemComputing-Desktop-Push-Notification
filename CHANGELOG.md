# Changelog

All notable changes to Foghorn are listed here, newest first.

## Unreleased

### Fixed
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

### Added
- `deploy/msi/build-msi.sh` builds the MSI, applies the post-build tweaks and
  checks the result, so a bare `wixl` build can't be shipped by mistake.
- *MSI deploy test* GitHub Actions workflow: installs the MSI on a real Windows
  runner (plain, with transform, repair, uninstall, command-line properties).
- Linux server documentation: a section in the README and a full
  `INSTALL-SERVER.md` Appendix A (firewall, HTTPS, low ports, reverse proxy,
  upgrade, password reset, moving between Windows and Linux, removal).

## 1.0.1

### Fixed
- **Client failed to connect on PCs with only .NET Framework 4.0.** The client
  used `String.TrimEnd(char)`, a single-char overload that was only added in
  .NET Framework 4.5, so older PCs threw `Method not found` on every request.
  Switched to the params-array overload that has existed since .NET 2.0.

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
