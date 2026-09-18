# Building from source

You do not need to build anything to use Foghorn — `dist\` has ready-made
programs. This is for when you want to change something.

## The client (`client\`) — builds on any Windows PC, nothing to install

The client is C# 5 for .NET Framework 4.x, deliberately: the compiler for that
ships inside Windows.

```bat
cd client
build.cmd
```

produces `client\FoghornClient.exe`. Copy it over `dist\FoghornClient.exe` (and
onto your deployment share).

Because it has to build with that older compiler, the source avoids newer C#
syntax (`$"…"` strings, `?.`, `nameof`, `out var`, expression-bodied members).
If you open it in Visual Studio and let it "modernise" the code, `build.cmd`
will stop working — build it as a normal .NET Framework 4.8 WinForms project
instead and that is fine too.

On Linux or macOS with Mono:

```bash
mcs -target:winexe -platform:anycpu -optimize+ -langversion:5 -out:dist/FoghornClient.exe \
    -win32icon:client/foghorn.ico -r:System.dll -r:System.Core.dll -r:System.Drawing.dll \
    -r:System.Windows.Forms.dll -r:System.Web.Extensions.dll client/src/*.cs
```

| File | What is in it |
|---|---|
| `src\Program.cs` | Start-up, command line, settings, who-am-I (OU, groups), logging. |
| `src\Poller.cs` | Talking to the server; retry and back-off. |
| `src\AlertForm.cs` | What alerts look like (`Look`), the two window styles, and the manager that stacks and queues them. |
| `src\Native.cs` | The few Windows API calls. |

## The server (`server\`) — needs Go 1.22 or later

Install Go from <https://go.dev/dl/>. Then, on Windows:

```bat
cd server
go test ./...
go build -mod=vendor -trimpath -ldflags "-s -w" -o ..\dist\foghorn-server.exe .
```

From Linux/macOS, for Windows: prefix the build with `GOOS=windows GOARCH=amd64`.

`-mod=vendor` matters. The one third-party dependency (`golang.org/x/sys`, for
the Windows service) is included in `server\vendor\`, and `go.mod` contains a
`replace` line pointing at a folder that only existed on the original build
machine. With `-mod=vendor` Go never looks for that folder. If you would rather
tidy it up on a machine with internet access: delete the `replace` line and the
`vendor` folder, then run `go mod tidy`.

The web console is plain HTML, CSS and JavaScript in `server\web\` — no build
step, no npm. It is embedded into the `.exe` when you build, so rebuild the
server after changing it.

| File | What is in it |
|---|---|
| `main.go` | Command line, config, first-run set-up, URL routes, start and stop. |
| `store.go` | The data model and saving to JSON files. |
| `api_client.go` | The endpoint PCs check in to; delivery and recall logic. |
| `api_admin.go` | Everything the web console and API tokens call. |
| `auth.go` | Passwords, sessions, tokens, lock-out, the permission check. |
| `match.go` | Targeting rules. |
| `service_windows.go` | Windows service install/run. |
| `foghorn_test.go` | Tests. Run them before and after any change. |

## The MSI and its transform (`deploy\msi\`)

On Linux with `msitools`: `wixl -a x64 -o dist/FoghornClient.msi deploy/msi/FoghornClient.wxs`.
Then re-apply the two post-build tweaks that make transforms work in a managed
install (wixl does not model them in source):

```bash
msibuild dist/FoghornClient.msi -q "UPDATE Component SET Condition='SERVERURL' WHERE Component='Settings'"
msibuild dist/FoghornClient.msi -q "UPDATE Property  SET Value='WIX_DOWNGRADE_DETECTED;WIX_UPGRADE_DETECTED;SERVERURL;CLIENTKEY;USESYSTEMPROXY' WHERE Property='SecureCustomProperties'"
```

On Windows with WiX Toolset v3: `candle -arch x64 FoghornClient.wxs` then
`light FoghornClient.wixobj`. In WiX source you would express the same two
tweaks as `<Condition>SERVERURL</Condition>` inside the `Settings` component
and `<Property Id="SecureCustomProperties" Value="…" />` — wixl does not accept
either. Bump `Version` in the `.wxs` for each release so upgrades work.

The transform (`.mst`) is generated on Windows from the base MSI by
`New-FoghornTransform.ps1`, using the built-in `WindowsInstaller.Installer`
COM object (`GenerateTransform` + `CreateTransformSummaryInfo`). This is the
same API WiX's `MakeMST` uses. It does not need to run on the machine that
built the MSI — anyone deploying can regenerate a transform whenever the
client key rotates.

## Calling it something other than "Foghorn"

The name people see comes from two places: the **organisation name** in
Settings (top of every alert) — no rebuild needed — and the word "Foghorn" in
the console's sidebar and page title (`server\web\app.js`, `index.html`).
Renaming the programs, service, registry keys and install folders is a
search-and-replace across `server\`, `client\` and `deploy\`; keep the registry
paths in `client\src\Program.cs`, `deploy\gpo\Foghorn.admx` and the install
scripts in step with each other.
