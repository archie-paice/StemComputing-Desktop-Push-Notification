# Installing the Foghorn server

This covers installing, configuring, securing, backing up, upgrading, moving
and removing the server. If you only want it running, do **Part 1** and stop.

- [Part 1 — Install on Windows](#part-1--install-on-windows)
- [Part 2 — First sign-in](#part-2--first-sign-in)
- [Part 3 — Settings you might change](#part-3--settings-you-might-change) (port, HTTPS)
- [Part 4 — Looking after it](#part-4--looking-after-it) (backup, upgrade, move, remove, lost password)
- [Appendix A — Installing on Linux](#appendix-a--installing-on-linux)
- [Appendix B — Installing by hand, without the script](#appendix-b--installing-by-hand-without-the-script)

---

## What you need

| | |
|---|---|
| A machine that stays on | Windows Server 2016 or later, or Windows 10/11. A VM is fine. It does **not** need to be a domain controller and should not be one. |
| Resources | Tiny. About 40 MB of memory with 1,500 PCs connected. |
| Network | Every PC that should receive alerts must be able to reach this machine on one TCP port (8080 unless you change it). |
| A name for it | PCs will be told the server's address. A DNS name such as `foghorn01` or an alias like `foghorn.college.local` is better than an IP address, because you can move the server later without touching the PCs. |
| Rights | A local administrator on that machine. No domain rights are needed for the server. |

---

## Part 1 — Install on Windows

1. Copy the whole Foghorn folder to the server (anywhere — `C:\Install\Foghorn` is fine; it is only needed during install).

2. Click **Start**, type `powershell`, right-click **Windows PowerShell** and choose **Run as administrator**.

3. Go to the `deploy` folder and run the installer:

   ```powershell
   cd C:\Install\Foghorn\deploy
   powershell -ExecutionPolicy Bypass -File .\Install-FoghornServer.ps1
   ```

   To use a port other than 8080, add `-Port 9000` (or whichever).

4. After a few seconds you will see something like this. **Copy it somewhere safe for the next ten minutes** — you need the password once and the client key when you deploy the clients.

   ```
   Foghorn is running.

     Web console:  http://FOGHORN01:8080/

     Foghorn first-run details
     =========================
     Web console username : admin
     Temporary password   : hT4k-Wm2p-Qz8r-Lc5n
     Client key           : 3f9a1c…(48 characters)
   ```

That is the install finished. What the script did, so nothing is a mystery:

| It did this | Why |
|---|---|
| Copied `foghorn-server.exe` to `C:\Program Files\Foghorn Server` | Standard location, not writable by ordinary users. |
| Created `C:\ProgramData\Foghorn` and restricted it to Administrators and SYSTEM | It holds password hashes and the client key. |
| Registered the **Foghorn Alert Server** service (automatic start, restarts itself if it ever crashes) | So it survives reboots without anyone logging on. |
| Added an inbound Windows Firewall rule for the program (Domain and Private profiles) | So PCs and browsers can reach it. |

> **If it says the service is not answering:** something else is probably using
> the port. See [TROUBLESHOOTING.md](TROUBLESHOOTING.md#the-server-will-not-start).

---

## Part 2 — First sign-in

1. On any computer, open the **Web console** address the installer printed.
2. Sign in as `admin` with the temporary password.
3. You are asked to choose your own password straight away (at least 10 characters). Once you do, the file holding the temporary password deletes itself.
4. Open **Settings** and fill in **Organisation name**. It is shown at the top of every alert so people can see it is genuine.
5. Still in **Settings**, add an account for each person who will send alerts (**Add a person**). Give people their own accounts — every alert is signed with the sender's name.
   - **Administrator**: can do everything.
   - **Sender**: can send alerts only. You can limit a sender to particular groups, for example a tutor to their own classroom.

Next: [put the client on your PCs](DEPLOY-CLIENTS.md).

---

## Part 3 — Settings you might change

Server-level settings live in one small file:

```
C:\ProgramData\Foghorn\config.json
```

```json
{
  "listen": ":8080",
  "tls_cert": "",
  "tls_key": "",
  "trust_proxy_headers": false
}
```

Edit it in Notepad **run as administrator**, save, then restart the service:

```powershell
Restart-Service FoghornServer
```

| Setting | Meaning |
|---|---|
| `listen` | The port, written `":8080"`. To listen on one address only: `"10.0.0.5:8080"`. |
| `tls_cert`, `tls_key` | Paths to a certificate and its private key, in PEM format. Set both to turn on HTTPS. See below. |
| `trust_proxy_headers` | Leave `false` unless Foghorn sits behind your own reverse proxy (IIS/nginx) that sets `X-Forwarded-For`. If `true` when there is no proxy, anyone can fake their IP address in the logs. |

### Changing the port

Change `listen`, restart the service. The firewall rule follows the program,
not the port, so it needs no change. **PCs must be told the new address** —
update the Group Policy setting or re-run the client installer.

### Turning on HTTPS

Out of the box Foghorn uses plain HTTP, which means alert text and staff
passwords cross your network unencrypted. On a switched, internal network many
sites accept that for a pilot; for permanent use turn HTTPS on.
[SECURITY.md](SECURITY.md#https) has the reasoning. The steps:

1. **Get a certificate** for the name PCs will use (e.g. `foghorn.college.local`).
   - *If you have Active Directory Certificate Services:* request a **Web Server** certificate for that name. Domain PCs already trust your CA, so nothing needs deploying to them.
   - *If you don't:* any certificate works, but each PC must trust whoever issued it. Deploy the issuing CA certificate with Group Policy: *Computer Configuration → Policies → Windows Settings → Security Settings → Public Key Policies → Trusted Root Certification Authorities*.
2. **Export it as two PEM files** — the certificate (`foghorn.crt`, including any intermediate certificates after it) and the private key (`foghorn.key`, *without* a password). If you have a `.pfx`, OpenSSL converts it:

   ```
   openssl pkcs12 -in foghorn.pfx -clcerts -nokeys -out foghorn.crt
   openssl pkcs12 -in foghorn.pfx -nocerts -nodes  -out foghorn.key
   ```
3. Put both files in `C:\ProgramData\Foghorn\` (already restricted to administrators).
4. Edit `config.json`. Note the doubled backslashes — JSON requires them:

   ```json
   {
     "listen": ":8443",
     "tls_cert": "C:\\ProgramData\\Foghorn\\foghorn.crt",
     "tls_key": "C:\\ProgramData\\Foghorn\\foghorn.key",
     "trust_proxy_headers": false
   }
   ```
5. `Restart-Service FoghornServer`, then browse to `https://foghorn.college.local:8443/`.
6. Change the clients' server address to the new `https://…` one.

If the service will not start after this, the reason is on the last line of `C:\ProgramData\Foghorn\foghorn.log` (usually a path typo or a password-protected key).

---

## Part 4 — Looking after it

### Where everything is

| Path | Contents |
|---|---|
| `C:\Program Files\Foghorn Server\foghorn-server.exe` | The program. |
| `C:\ProgramData\Foghorn\config.json` | Port and HTTPS settings. |
| `C:\ProgramData\Foghorn\state.json` | Accounts, groups, templates, organisation name, client key. |
| `C:\ProgramData\Foghorn\alerts.json` | Alert history (the most recent 300 alerts). |
| `C:\ProgramData\Foghorn\clients.json` | Computers that have checked in (forgotten after 45 days' absence). |
| `C:\ProgramData\Foghorn\foghorn.log` | Who signed in, who sent what, errors. Rolls over at 10 MB. |

### Backing up

Copy the `C:\ProgramData\Foghorn` folder. That is everything. It is safe to
copy while the service is running. To restore: stop the service, put the folder
back, start the service.

### Upgrading

Run `Install-FoghornServer.ps1` from the new version's `deploy` folder, exactly
as in Part 1. It notices the existing service, stops it, swaps the program and
starts it again. Data and settings are untouched. Clients reconnect by
themselves within a minute.

### Moving to another machine

1. Install on the new machine (Part 1).
2. `Stop-Service FoghornServer` on **both** machines.
3. Copy `C:\ProgramData\Foghorn` from old to new, replacing what is there.
4. `Start-Service FoghornServer` on the new machine.
5. Point the DNS name at the new machine — or, if you used the machine's own name, change the clients' server address.
6. Uninstall from the old machine.

### Someone forgot their password

Another administrator: **Settings → Edit → Reset password**, and pass on the temporary password shown.

**The only administrator** is locked out: on the server, in an administrator PowerShell —

```powershell
cd "C:\Program Files\Foghorn Server"
.\foghorn-server.exe service stop
.\foghorn-server.exe reset-password admin
.\foghorn-server.exe service start
```

The second command prints a new temporary password. (It refuses to run while the service is up, because the running service would overwrite the change.)

### Service commands

```powershell
Get-Service FoghornServer          # is it running?
Restart-Service FoghornServer
cd "C:\Program Files\Foghorn Server"
.\foghorn-server.exe service status | start | stop
.\foghorn-server.exe version
```

### Removing it

```powershell
powershell -ExecutionPolicy Bypass -File .\Uninstall-FoghornServer.ps1              # keeps your data
powershell -ExecutionPolicy Bypass -File .\Uninstall-FoghornServer.ps1 -RemoveData  # removes everything
```

---

## Appendix A — Installing on Linux

The server is a single static binary.

```bash
sudo useradd --system --home /var/lib/foghorn --shell /usr/sbin/nologin foghorn
sudo install -m 0755 dist/foghorn-server-linux-amd64 /usr/local/bin/foghorn-server
sudo install -m 0644 deploy/linux/foghorn.service /etc/systemd/system/foghorn.service
sudo systemctl daemon-reload
sudo systemctl enable --now foghorn
sudo cat /var/lib/foghorn/FIRST-RUN.txt        # temporary password and client key
```

Data lives in `/var/lib/foghorn`; `config.json` there works as in Part 3. Open
the port in your firewall (`sudo ufw allow 8080/tcp` or equivalent). To reset a
password: `sudo systemctl stop foghorn`, then
`sudo -u foghorn foghorn-server reset-password -data /var/lib/foghorn admin`,
then start it again.

## Appendix B — Installing by hand, without the script

From an administrator prompt:

```bat
mkdir "C:\Program Files\Foghorn Server"
copy dist\foghorn-server.exe "C:\Program Files\Foghorn Server\"
mkdir C:\ProgramData\Foghorn
icacls C:\ProgramData\Foghorn /inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F
"C:\Program Files\Foghorn Server\foghorn-server.exe" service install
netsh advfirewall firewall add rule name="Foghorn Alert Server" dir=in action=allow program="C:\Program Files\Foghorn Server\foghorn-server.exe" profile=domain,private
type C:\ProgramData\Foghorn\FIRST-RUN.txt
```

To try it without installing anything, just run `foghorn-server.exe` in a
console window; it prints the first-run details and stops with Ctrl+C. (Run
that way it still keeps its data in `C:\ProgramData\Foghorn` unless you add
`-data C:\some\folder`.)
