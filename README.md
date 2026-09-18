# Foghorn

**Send pop-up alerts to Windows desktops from a web page.**

A member of staff types a message in their browser, picks who should get it
(everyone, a room, an AD group, one PC) and presses **Send alert**. A second or
two later it is on top of whatever those people are doing.

![The Foghorn web console](docs/images/console-send.png)

No cloud, no licences, no database. One server program, one small client
program, and clear documentation. Built for schools, colleges and offices on
Active Directory.

---

## Features

- **Three styles**, so alerts fit their purpose:
  - *Corner* — a card at the top-right, above other windows, without stealing the keyboard.
  - *Centre* — the same card, larger, in the middle of the screen.
  - *Full screen* — covers every monitor. For emergencies and "eyes to the front".
- **Targeting** by computer name, logged-on user, AD organisational unit, AD group, or IP subnet. Saved groups (typically rooms) for one-click reuse.
- **Acknowledgement tracking.** Ask people to confirm they read it, and see who did.
- **Recall** — take an alert off every screen still showing it.
- **Live delivery view.** Watch, per computer, who has displayed and confirmed each alert.
- **Templates** for wording you reuse. Wording only — audience is always a deliberate choice.
- **Roles.** *Administrators* manage everything. *Senders* can send alerts, optionally limited to their own groups (a tutor to their own classroom, say). Every alert is signed with the sender's name.
- **API tokens** for scripts and monitoring systems.
- **Small.** About 40 MB of memory on the server with 1,500 PCs connected. The client is a 64 KB `.exe` and needs nothing installed on Windows 10 or 11.
- **Standard deployment.** Roll the client out with an MSI + transform through Group Policy Software Installation, Intune, SCCM, PDQ, or a start-up script.
- **No internet needed.** Console uses system fonts only. No CDN, no telemetry, no external calls.
- **HTTPS** with your own certificate. Cross-site request forgery and login lock-out are built in. Passwords are PBKDF2-SHA256 (210,000 rounds).

## Quick start (about 10 minutes)

You need a Windows machine that stays on (a server or a spare PC — Foghorn uses
very little of it) and one test PC.

**1. Install the server.** Copy this whole folder to the server. Open
PowerShell **as administrator**, go to the `deploy` folder and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-FoghornServer.ps1
```

It prints the console address, a temporary password and the **client key** —
copy them for the next few minutes.

**2. Sign in.** On any PC, open `http://YOUR-SERVER:8080/`, sign in as `admin`
with the temporary password, choose your own. Under **Settings** set your
organisation name.

**3. Try it on one PC by hand.** Copy `dist\FoghornClient.exe`,
`deploy\Install-FoghornClient.cmd` and `deploy\Install-FoghornClient.ps1` into
one folder on a test PC. From an **administrator** command prompt in that
folder:

```bat
Install-FoghornClient.cmd -ServerUrl http://YOUR-SERVER:8080 -ClientKey PASTE-THE-KEY
```

Log on as a normal user. The PC appears in **Computers** with a green dot
within a few seconds. Go to **Send an alert** and send yourself one.

**4. Roll it out to every PC.** Follow
[docs/DEPLOY-CLIENTS.md](docs/DEPLOY-CLIENTS.md) — the recommended path is an
**MSI + transform** deployed with Group Policy Software Installation.

> Want to see what alerts look like before installing anything? Open a command
> prompt in the `dist` folder on any Windows PC and run
> `FoghornClient.exe --test`. It shows one of each style without needing a
> server.

## How it fits together

```
   Staff browser                     Foghorn server                   Windows PCs
 ┌───────────────┐   http(s)     ┌────────────────────┐  http(s)  ┌──────────────────┐
 │  Web console  │ ───────────▶  │  foghorn-server    │ ◀──────── │ FoghornClient.exe│
 │  (any device) │               │  one Windows       │  each PC  │ runs as the      │
 └───────────────┘               │  service, port 8080│  keeps a  │ logged-on user,  │
                                 │  data: one folder  │  request  │ no window until  │
                                 └────────────────────┘  open     │ an alert arrives │
                                                                  └──────────────────┘
```

- **Clients connect out to the server.** Nothing listens on the PCs, so there are no firewall changes on them.
- Each client tells the server its computer name, logged-on user, the computer's OU and the user's AD groups. That is how targeting works — the server never talks to a domain controller directly.
- All data is JSON files in `C:\ProgramData\Foghorn`. Backing up is copying that folder.

## What is in this folder

| Folder / file | What it is |
|---|---|
| `dist\foghorn-server.exe` | The server, ready to run (Windows x64). Includes the web console. |
| `dist\FoghornClient.exe` | The desktop client, ready to deploy. Needs nothing installed on Windows 10 or 11. |
| `dist\FoghornClient.msi` | The client as an MSI. Ship generic; pair with a transform (below) for deployment. |
| `dist\foghorn-server-linux-amd64` | The server for Linux. |
| `dist\SHA256SUMS.txt` | Checksums of the four binaries. |
| `deploy\` | Install and uninstall scripts, the MSI transform generator, the ADMX Group Policy template, the systemd unit. |
| `docs\` | Full documentation — the guides below. |
| `server\`, `client\` | Source code (Go for the server, C# for the client). |

## Documentation

| Read this | When |
|---|---|
| **[docs/INSTALL-SERVER.md](docs/INSTALL-SERVER.md)** | Installing, moving, upgrading, backing up, removing the server. HTTPS. |
| **[docs/DEPLOY-CLIENTS.md](docs/DEPLOY-CLIENTS.md)** | Getting the client onto every PC — MSI + transform through Group Policy, or a start-up script. |
| **[docs/USER-GUIDE.md](docs/USER-GUIDE.md)** | For the staff who will send alerts. Safe to hand out as it is. |
| **[docs/SECURITY.md](docs/SECURITY.md)** | What is protected, what is not, and how to tighten it. Read before going live. |
| **[docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)** | Something is not working. Organised by symptom. |
| **[docs/API.md](docs/API.md)** | Sending alerts from scripts, monitoring systems, or other tools. |
| **[docs/BUILDING.md](docs/BUILDING.md)** | Rebuilding from source. Renaming the product. |
| **[CHANGELOG.md](CHANGELOG.md)** | What changed and when. |

## Building from source

```bash
# Server (needs Go 1.22+)
cd server && go test ./... && go build -mod=vendor -trimpath -ldflags "-s -w" -o ../dist/foghorn-server.exe .

# Client (on Windows, uses the C# compiler that ships with Windows itself)
cd client && build.cmd
```

Full details, including cross-compiling and rebuilding the MSI, in
[docs/BUILDING.md](docs/BUILDING.md).

## Contributing

Contributions and bug reports are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md) for how to file issues, open pull requests,
and the coding conventions the project follows.

## Licence

MIT — see [LICENSE](LICENSE). The server bundles `golang.org/x/sys`
(BSD-3-Clause; licence in `server/vendor/golang.org/x/sys/LICENSE`).
