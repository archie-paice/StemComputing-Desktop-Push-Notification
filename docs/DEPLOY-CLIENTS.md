# Deploying the Foghorn client to your PCs

The client is one small program, `FoghornClient.exe` (about 64 KB). It needs
**.NET Framework 4.x, which is already part of Windows 10 and 11**, so there is
nothing else to install. It runs silently as whoever is logged on, shows nothing
until an alert arrives, and never runs with administrator rights.

Do the steps in order. Step 1 takes five minutes and saves hours.

- [Step 1 — Pilot on one PC by hand](#step-1--pilot-on-one-pc-by-hand)
- [Step 2 — Put the files on a share](#step-2--put-the-files-on-a-share)
- [Step 3 — Create the Group Policy Object](#step-3--create-the-group-policy-object)
- [Step 4 — Check it worked](#step-4--check-it-worked)
- [Upgrading and removing](#upgrading-and-removing)
- [Appendix A — Setting the server address with the Group Policy template (ADMX)](#appendix-a--setting-the-server-address-with-the-group-policy-template-admx)
- [Appendix B — Deploying the MSI instead](#appendix-b--deploying-the-msi-instead)
- [Appendix C — Intune, PDQ Deploy, SCCM and similar](#appendix-c--intune-pdq-deploy-sccm-and-similar)
- [Appendix D — Reference: what the installer changes, settings, command line](#appendix-d--reference)

**Before you start you need** the server running
([INSTALL-SERVER.md](INSTALL-SERVER.md)) and two pieces of information:

| | Example | Where to find it |
|---|---|---|
| **Server address** | `http://foghorn01:8080` | Printed by the server installer. It is the console address without the trailing `/`. |
| **Client key** | `3f9a1c…` (48 characters) | Web console → **Settings** → **Client key** → **Copy**. |

---

## Step 1 — Pilot on one PC by hand

Pick one typical student PC.

1. Copy these three files into one folder on it, e.g. `C:\Temp\Foghorn`:
   - `dist\FoghornClient.exe`
   - `deploy\Install-FoghornClient.ps1`
   - `deploy\Install-FoghornClient.cmd`

2. **See what alerts look like.** Open a normal Command Prompt in that folder:

   ```bat
   FoghornClient.exe --test
   ```

   Two sample alerts appear (corner and centre). Close both and a full-screen
   one appears. Check: are they on top of your other windows? Is the text
   sharp? Can you keep typing in another window while the corner alert is
   showing? If anything looks wrong here, stop and see
   [TROUBLESHOOTING.md](TROUBLESHOOTING.md#alerts-look-wrong).

3. **Install it.** Open Command Prompt **as administrator** in that folder:

   ```bat
   Install-FoghornClient.cmd -ServerUrl http://foghorn01:8080 -ClientKey PASTE-THE-KEY-HERE
   ```

   It should finish with `Foghorn client is installed.`

4. **Check the connection.** As a normal user:

   ```bat
   "C:\Program Files\Foghorn\FoghornClient.exe" --status
   ```

   A box shows the settings in use and ends with
   `Connection test: OK`. If it says `FAILED`, the box tells you why — fix that
   before going further.

5. **Send yourself an alert.** Log off, log on as a student. In the web console
   the PC appears under **Computers** with a green dot within a few seconds.
   Choose **Send alert** on that row and send something.

6. **Check the information used for targeting.** On the **Computers** page,
   confirm the **OU** column is right for that PC, and hover over the user name
   to see the AD groups the client reported. If you plan to target by OU or
   group, this is what the rules will match against.

When all six work, carry on.

---

## Step 2 — Put the files on a share

Group Policy start-up scripts run as the **computer account** before anyone
logs on, so the files must be somewhere computers can read. The domain's
`NETLOGON` share is the simplest: every domain computer can already read it.

1. On a domain controller (or from your PC, as a domain admin) open
   `\\YOURDOMAIN\NETLOGON` and create a folder called `Foghorn`.
2. Copy the same three files into it:

   ```
   \\YOURDOMAIN\NETLOGON\Foghorn\FoghornClient.exe
   \\YOURDOMAIN\NETLOGON\Foghorn\Install-FoghornClient.ps1
   \\YOURDOMAIN\NETLOGON\Foghorn\Install-FoghornClient.cmd
   ```

Any other share works too, provided **Domain Computers** has Read access to
both the share and the folder.

---

## Step 3 — Create the Group Policy Object

1. Open **Group Policy Management** (`gpmc.msc`).

2. Find the OU that contains the **computers** that should get the client
   (not the users). Right-click it → **Create a GPO in this domain, and Link it
   here…** → name it `Foghorn client` → **OK**.

   > Start with a small OU — one classroom — and widen it once you are happy.

3. Right-click the new GPO → **Edit**.

4. Go to **Computer Configuration → Policies → Windows Settings → Scripts
   (Startup/Shutdown)** and double-click **Startup**.

5. Stay on the first tab, **Scripts** (not "PowerShell Scripts"). Click **Add…**
   and fill in:

   | Field | Value |
   |---|---|
   | **Script Name** | `\\YOURDOMAIN\NETLOGON\Foghorn\Install-FoghornClient.cmd` |
   | **Script Parameters** | `-ServerUrl http://foghorn01:8080 -ClientKey PASTE-THE-KEY-HERE` |

   Click **OK**, then **OK** again.

   > Why the `.cmd` and not the `.ps1`? The `.cmd` file starts PowerShell with
   > the execution policy bypassed for this one script, so it works whatever
   > your PCs' PowerShell policy is.

6. **Make sure the network is up before the script runs.** In the same GPO go
   to **Computer Configuration → Policies → Administrative Templates → System →
   Logon** and set **Always wait for the network at computer startup and
   logon** to **Enabled**. Without this, PCs on a fast boot often run start-up
   scripts before they can see the share.

7. Close the editor.

That is the whole deployment. Each PC runs the script every time it starts. The
script is careful to do nothing if nothing needs doing, so this costs well under
a second per boot — and it means **upgrades are automatic**: replace the `.exe`
on the share and every PC picks it up at its next restart.

---

## Step 4 — Check it worked

On one PC in the OU:

1. Run `gpupdate /force` from an administrator prompt and then **restart**
   (choose *Restart*, not *Shut down* — see the note below).
2. After it boots, look at the install log:

   ```bat
   type C:\Windows\Temp\FoghornClient-install.log
   ```

   You should see `Installed FoghornClient.exe…`, `Set to start at log-on`,
   `Registered watchdog task`, `Foghorn client is installed.`
3. Log on as a student. The PC appears in the console under **Computers**.

> **Shut down vs restart.** Windows 10/11 "Fast Startup" makes *Shut down →
> power on* a kind of hibernate-resume, and start-up scripts do not run. They
> always run on a **Restart**, and on a cold boot if Fast Startup is off. Many
> schools disable Fast Startup by policy for exactly this reason
> (*Computer Configuration → Administrative Templates → System → Shutdown →
> Require use of fast startup* = **Disabled**). Either way every PC gets the
> client the first time it properly restarts — Windows Update will see to that.

If a PC does not get it, go to
[TROUBLESHOOTING.md](TROUBLESHOOTING.md#a-pc-never-appears-in-the-console).

---

## Upgrading and removing

**Upgrade every PC:** replace `FoghornClient.exe` on the share with the new one.
PCs install it at their next start-up. People already logged on keep the old
version until they next log on; old and new clients work with the server side by
side.

**Change the server address or client key:** edit the **Script Parameters** in
the GPO. PCs pick up the change at their next start-up. (If you expect to change
these more than once, [Appendix A](#appendix-a--setting-the-server-address-with-the-group-policy-template-admx)
is a better way to set them: changes apply within minutes, without a restart.)

**Remove from every PC:** copy `deploy\Uninstall-FoghornClient.ps1` and
`Uninstall-FoghornClient.cmd` to the share, then in the GPO **replace** the
start-up script with `\\YOURDOMAIN\NETLOGON\Foghorn\Uninstall-FoghornClient.cmd`
(no parameters). Leave it linked until the PCs have all restarted, then delete
the GPO.

**Remove from one PC:** run `Uninstall-FoghornClient.cmd` as administrator.

---

## Appendix A — Setting the server address with the Group Policy template (ADMX)

Instead of passing `-ServerUrl` and `-ClientKey` to the script, you can set them
as a proper Group Policy setting. Advantages: they show up in GPO reports,
changes reach PCs at the next policy refresh (about 90 minutes, or
`gpupdate`) with no restart, and they take priority over anything stored
locally. You still need Step 3 (or the MSI) to get the program itself onto PCs.

1. Copy the template into your policy store:

   | Copy this | To here (central store) |
   |---|---|
   | `deploy\gpo\Foghorn.admx` | `\\YOURDOMAIN\SYSVOL\YOURDOMAIN\Policies\PolicyDefinitions\` |
   | `deploy\gpo\en-US\Foghorn.adml` | `\\YOURDOMAIN\SYSVOL\YOURDOMAIN\Policies\PolicyDefinitions\en-US\` |

   No central store? Copy them to `C:\Windows\PolicyDefinitions\` (and its
   `en-US` subfolder) on the machine where you edit GPOs instead.

2. Edit the `Foghorn client` GPO. Go to **Computer Configuration → Policies →
   Administrative Templates → Foghorn**.

3. Open **Foghorn server connection**, choose **Enabled**, fill in **Server
   address** and **Client key**, **OK**.

4. In the start-up script (Step 3.5) you can now leave **Script Parameters**
   empty.

The other setting there, **Connect through the system web proxy**, should stay
*Not configured* unless PCs can only reach the Foghorn server through a proxy.

## Appendix B — Deploying the MSI instead

`dist\FoghornClient.msi` installs the program to `C:\Program Files\Foghorn` and
makes it start at log-on. Use it if you prefer Group Policy **Software
Installation** to start-up scripts.

1. Put `FoghornClient.msi` on a share Domain Computers can read.
2. In the GPO: **Computer Configuration → Policies → Software Settings →
   Software installation** → right-click → **New → Package…** → browse to the
   MSI **by its UNC path** → **Assigned**.
3. Set the server address and client key with [Appendix A](#appendix-a--setting-the-server-address-with-the-group-policy-template-admx).
   (Software Installation cannot pass settings to an MSI.)

Differences from the script method: the MSI does **not** add the watchdog that
restarts the client if a student closes it (see
[SECURITY.md](SECURITY.md#students-closing-the-client)), and upgrading means
adding the new MSI as an upgrade package rather than just replacing a file.

Installing the MSI by hand or from a deployment tool, you *can* pass settings:

```bat
msiexec /i FoghornClient.msi /qn SERVERURL=http://foghorn01:8080 CLIENTKEY=PASTE-THE-KEY
```

## Appendix C — Intune, PDQ Deploy, SCCM and similar

Deploy the three files from Step 1 as a package and run, as SYSTEM or an
administrator:

```bat
Install-FoghornClient.cmd -ServerUrl http://foghorn01:8080 -ClientKey PASTE-THE-KEY
```

Exit code `0` is success, `1` is failure (the reason is in
`C:\Windows\Temp\FoghornClient-install.log`). Detection rule: the file
`C:\Program Files\Foghorn\FoghornClient.exe` exists. Uninstall command:
`Uninstall-FoghornClient.cmd`.

## Appendix D — Reference

### What the installer changes on a PC

| Change | Detail |
|---|---|
| Program | `C:\Program Files\Foghorn\FoghornClient.exe` |
| Settings (only if you passed them) | `HKLM\SOFTWARE\Foghorn` → `ServerUrl`, `ClientKey` |
| Start at log-on | `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run` → `FoghornClient` |
| Watchdog | Scheduled task **Foghorn Client Watchdog**: runs as the logged-on user at log-on and every 5 minutes; does nothing if the client is already running. Skip it with `-NoWatchdog`. |
| Install log | `C:\Windows\Temp\FoghornClient-install.log` |

Per user, the client writes a small log to `%LOCALAPPDATA%\Foghorn\client.log`.

### Where the client looks for its settings

First match wins:

1. The command line: `--server URL --key KEY`
2. Group Policy: `HKLM\SOFTWARE\Policies\Foghorn` (Appendix A)
3. The installer's registry key: `HKLM\SOFTWARE\Foghorn`
4. A file called `foghorn-client.ini` next to the `.exe`:

   ```ini
   ServerUrl=http://foghorn01:8080
   ClientKey=3f9a1c…
   ```

The client re-reads its settings every time it checks in with the server (at
least twice a minute), so a change never needs a log-off.

### Client command line

| Command | What it does |
|---|---|
| `FoghornClient.exe` | Normal running. No window. A second copy in the same session exits immediately. |
| `FoghornClient.exe --test` | Shows one alert of each style. Needs no server. |
| `FoghornClient.exe --status` | Shows the settings in use, what the PC reports about itself, and tests the connection. |
| `FoghornClient.exe --server URL --key KEY` | Runs against a different server, for testing. |
