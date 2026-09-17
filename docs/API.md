# Sending alerts from scripts (API)

Anything that can make an HTTPS request can send a Foghorn alert: a PowerShell
script, a monitoring system, a fire-panel relay with a Raspberry Pi behind it.

## 1. Create a token

Web console → **Settings** → **API tokens** → **New token**. Name it after what
will use it. Copy the token (`fh_…`) when it is shown — it is not shown again.

A token can **send and recall alerts** and read the computer list and history.
It cannot change settings or accounts. Alerts it sends are signed with the
token's name. Delete the token to cut off whatever was using it.

Send it on every request as a header:

```
Authorization: Bearer fh_your_token_here
```

## 2. Send an alert

`POST /api/alerts` with a JSON body.

**PowerShell**

```powershell
$token = 'fh_your_token_here'
$alert = @{
    title   = 'Server maintenance at 17:00'
    message = 'Shared drives will be unavailable for about 20 minutes. Save your work before then.'
    level   = 'warning'
    display = 'corner'
    target  = @{ all = $true }
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Method Post -Uri 'http://foghorn01:8080/api/alerts' `
    -Headers @{ Authorization = "Bearer $token" } `
    -ContentType 'application/json; charset=utf-8' `
    -Body ([Text.Encoding]::UTF8.GetBytes($alert))
```

**curl**

```bash
curl -X POST http://foghorn01:8080/api/alerts \
  -H "Authorization: Bearer fh_your_token_here" \
  -H "Content-Type: application/json" \
  -d '{"title":"Leave the building now","level":"critical","display":"fullscreen","require_ack":true,"sound":true,"target":{"all":true}}'
```

The reply (HTTP `201`) includes the alert's `id` and `online_reach`, the number
of computers that were on and matched at that moment:

```json
{ "alert": { "id": "9f2c41d07a3be815", "status": "active", "...": "..." }, "online_reach": 212 }
```

### Fields

| Field | Type | Default | Meaning |
|---|---|---|---|
| `title` | text, ≤120 | **required** | The headline. |
| `message` | text, ≤2000 | empty | Body text. Line breaks (`\n`) are kept. Plain text only. |
| `level` | `info` \| `warning` \| `critical` | `info` | Shown as Notice / Warning / Urgent. |
| `display` | `corner` \| `center` \| `fullscreen` | `corner` | Where it appears. |
| `require_ack` | true/false | false | Must be closed with "I've read this"; acknowledgements are recorded. Forces `display_seconds` to 0. |
| `display_seconds` | 0–86400 | 0 | Close itself after this long. 0 = stays until closed. |
| `sound` | true/false | false | Play the Windows alert sound. |
| `link` | `http(s)://…`, ≤500 | empty | Adds an "Open link" button. |
| `expires_minutes` | 1–10080 | 10 | Keep delivering to PCs that log on within this many minutes. |
| `target` | object | **required** | Who gets it — see below. |

### Target

Exactly one of these shapes:

```json
{ "all": true }
```
```json
{ "group_ids": ["4be19c02d7a35f60"] }
```
```json
{ "rules": [ { "field": "hostname", "pattern": "LAB1-*" },
             { "field": "group",    "pattern": "Students-Year1" } ] }
```

Rules are OR'd: a computer matching **any** rule gets the alert.

| `field` | Matches against | Pattern |
|---|---|---|
| `hostname` | Computer name | Wildcards `*` and `?`. Not case-sensitive. |
| `user` | Logged-on username, with or without `DOMAIN\` | Wildcards. |
| `ou` | The computer's distinguished name | Plain text matches anywhere in it (`OU=Library`); or wildcards against the whole DN. |
| `group` | AD groups in the user's log-on token, with or without `DOMAIN\` | Wildcards. |
| `ip` | The address the PC connects from | CIDR (`10.20.30.0/24`) or wildcards (`10.20.*`). |

Group ids come from `GET /api/groups`.

## 3. Other calls

| Call | Does |
|---|---|
| `GET /api/alerts?limit=50` | Recent alerts, newest first, with totals. |
| `GET /api/alerts/{id}` | One alert with a per-computer delivery list. |
| `POST /api/alerts/{id}/recall` | Take it off every screen and stop delivering it. (A token may recall only alerts it sent.) |
| `POST /api/reach` with a `target` object as the body | How many computers that target matches: `{"online": 23, "known": 41, "sample": ["LAB1-PC01", …]}`. Sends nothing. |
| `GET /api/clients` | Every computer that has checked in, with `online` true/false. |
| `GET /api/groups` | Saved groups with their ids and rules. |
| `GET /healthz` | `{"status":"ok"}`. No token needed. For monitoring. |

## Errors

Errors are JSON with a code and a sentence you can show to a person:

```json
{ "error": "invalid", "message": "The link must be a full http:// or https:// address." }
```

| HTTP | `error` | Meaning |
|---|---|---|
| 400 | `invalid`, `bad_json` | Something wrong with the request; `message` says what. |
| 401 | `not_signed_in` | Token missing, mistyped or deleted. |
| 403 | `forbidden` | The token (or account) is not allowed to do that. |
| 404 | `not_found` | No such alert / endpoint. |

## Example: recall after ten minutes

```powershell
$h = @{ Authorization = "Bearer $token" }
$r = Invoke-RestMethod -Method Post -Uri "$server/api/alerts" -Headers $h -ContentType 'application/json' -Body $alert
Start-Sleep -Seconds 600
Invoke-RestMethod -Method Post -Uri "$server/api/alerts/$($r.alert.id)/recall" -Headers $h
```

> Treat a token like a password. Keep it out of scripts that students can read;
> on Windows, store it with `Export-Clixml` under the service account that runs
> the script, or in your secrets manager.
