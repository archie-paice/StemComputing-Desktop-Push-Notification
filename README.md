# Foghorn

Send pop-up alerts to Windows desktops from a web page.

A member of staff types a message in their browser, picks who should get it
(everyone, a room, an AD group, one PC) and presses **Send alert**. A second or
two later it is on top of whatever those people are doing — as a card in the
corner, a box in the middle of the screen, or a full-screen takeover for
emergencies. You can see which computers showed it and who confirmed they read
it, and you can recall it.

Built for Windows PCs on Active Directory. No cloud, no licences, no database:
one server program, one small client program, and this documentation.

---

## Contents of this folder

| Folder / file | What it is |
|---|---|
| `dist\foghorn-server.exe` | The server, ready to run (Windows x64). Includes the web console. |
| `dist\FoghornClient.exe` | The desktop client, ready to deploy. Needs nothing installed on Windows 10/11. |
| `dist\FoghornClient.msi` | The same client as an MSI, for Group Policy *Software Installation*. |
| `dist\foghorn-server-linux-amd64` | The server for Linux, if you would rather host it there. |
| `deploy\` | Install and uninstall scripts, the Group Policy template, the systemd unit. |
| `docs\` | The guides listed below. |
| `server\`, `client\` | Full source code. See [docs/BUILDING.md](docs/BUILDING.md). |

## How it fits together

```
   Staff browser                     Foghorn server                   Student PCs
 ┌───────────────┐   http(s)     ┌────────────────────┐  http(s)  ┌──────────────────┐
 │  Web console  │ ───────────▶  │  foghorn-server    │ ◀──────── │ FoghornClient.exe│
 │  (any device) │               │  one Windows       │  each PC  │ runs as the      │
 └───────────────┘               │  service, port 8080│  keeps a  │ logged-on user,  │
                                 │  data: one folder  │  request  │ no window until  │
                                 └────────────────────┘  open     │ an alert arrives │
                                                                  └──────────────────┘
```

* The **clients connect out to the server**. Nothing listens on the PCs, so
  there are no firewall changes on them.
* Each client tells the server its computer name, logged-on user, the
  computer's OU and the user's AD groups. That is how targeting works — the
  server never needs to talk to a domain controller.
* All data is a handful of JSON files in `C:\ProgramData\Foghorn`. Backing up
  means copying that folder.

## Quick start (about 10 minutes)

You need: a Windows machine that stays on (a server or a spare PC — it uses
about 40 MB of memory), and one test PC.

**1. Install the server.** Copy this whole folder to the server machine. Open
PowerShell **as administrator**, go to the `deploy` folder and run:

```powershell
powershell -ExecutionPolicy Bypass -File .\Install-FoghornServer.ps1
```

It prints the console address, a temporary password and the **client key**.

**2. Sign in.** On any PC open `http://YOUR-SERVER:8080/`, sign in as `admin`
with the temporary password, and choose your own. Under **Settings** set your
organisation name.

**3. Put the client on one test PC.** Copy `deploy\Install-FoghornClient.ps1`,
`deploy\Install-FoghornClient.cmd` and `dist\FoghornClient.exe` into one folder
on the test PC. From an **administrator** prompt in that folder:

```bat
Install-FoghornClient.cmd -ServerUrl http://YOUR-SERVER:8080 -ClientKey PASTE-THE-KEY
```

**4. Check it.** Log on to the test PC as a normal user. Within a few seconds
it appears under **Computers** in the console with a green dot. Go to **Send an
alert**, type something, send it, and watch it arrive.

**5. Roll it out.** Follow [docs/DEPLOY-CLIENTS.md](docs/DEPLOY-CLIENTS.md) to
deploy to every PC with Group Policy.

> Want to see what alerts look like before installing anything? Open a command
> prompt in the `dist` folder on any Windows PC and run
> `FoghornClient.exe --test`. It shows one of each style without needing a server.

## The guides

| Read this | When |
|---|---|
| [docs/INSTALL-SERVER.md](docs/INSTALL-SERVER.md) | Installing, moving, upgrading, backing up or removing the server. HTTPS. |
| [docs/DEPLOY-CLIENTS.md](docs/DEPLOY-CLIENTS.md) | Getting the client onto PCs with Group Policy (step by step), MSI, or by hand. |
| [docs/USER-GUIDE.md](docs/USER-GUIDE.md) | For the staff who will send alerts. Safe to hand out as it is. |
| [docs/SECURITY.md](docs/SECURITY.md) | What is protected, what is not, and how to tighten it. Read before going live. |
| [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Something is not working. Starts from the symptom. |
| [docs/API.md](docs/API.md) | Sending alerts from scripts or other systems. |
| [docs/BUILDING.md](docs/BUILDING.md) | Rebuilding the programs from source, renaming the product. |

## What has and has not been tested

Being straight about this matters more than looking finished.

**Tested, on Linux, during development:**

* The server: automated unit tests plus a 33-step end-to-end test covering
  sign-in, forced password change, CSRF protection, lock-out after repeated bad
  passwords, targeting by name / OU / AD group / subnet, restricted senders,
  API tokens, delivery, acknowledgement and recall.
* Load: 1,500 simulated PCs connected at once used about 40 MB of memory on
  the server, and all 1,500 received an alert within 70 ms of each other.
* The web console: driven through every page in a real (headless) browser.
* The client: the actual `FoghornClient.exe` in this folder was run (under
  Mono) against the actual server — it connected, displayed corner, centre and
  full-screen alerts, reported them as shown, and removed one when recalled.
* The install scripts, scheduled-task definition and Group Policy template
  were syntax-checked, and the MSI's tables inspected.

**Not yet tested — this needs you:**

* Nothing has been run on real Windows or a real domain. The Windows-only
  parts — the Windows service, the install scripts, always-on-top and
  no-focus-stealing behaviour, DPI scaling, reading the OU and AD groups, the
  watchdog task, the MSI — are written carefully against documented behaviour
  but are unproven.

So: **pilot on one PC first.** `FoghornClient.exe --test` proves the display
side; `FoghornClient.exe --status` proves the settings and connection.
[docs/DEPLOY-CLIENTS.md](docs/DEPLOY-CLIENTS.md) builds that pilot into the steps.

## Licence

MIT — see [LICENSE](LICENSE). The server bundles `golang.org/x/sys`
(BSD-3-Clause; licence in `server/vendor/golang.org/x/sys/LICENSE`).
