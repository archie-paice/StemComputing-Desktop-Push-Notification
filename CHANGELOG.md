# Changelog

## 1.0.0

First version.

- Server: single executable with embedded web console; Windows service; JSON-file storage; HTTP or HTTPS.
- Alerts: three levels, three display styles, acknowledgement tracking, auto-close, sound, link button, late-arrival window, recall.
- Targeting: everyone, saved groups, or rules on computer name, username, OU, AD group and IP/subnet.
- Accounts: administrators and senders (optionally limited to groups); API tokens.
- Client: .NET Framework 4.x WinForms; always-on-top without stealing focus; multi-monitor full-screen; Group Policy (ADMX), registry, ini or command-line settings; `--test` and `--status`.
- Deployment: PowerShell installers, watchdog task, ADMX template, MSI, systemd unit.
