# Contributing

Thank you for looking at Foghorn. Bug reports and pull requests are welcome.

## Reporting a bug

Open an issue on GitHub and include:

- **What you did**, **what you expected**, **what happened**.
- **Which part** is misbehaving — server, web console, client, MSI, installer.
- **Version**. Server: `foghorn-server.exe version`. Client: shown in the
  `--status` box, or the first line of `%LOCALAPPDATA%\Foghorn\client.log`.
- **Relevant log lines** — the last few from `C:\ProgramData\Foghorn\foghorn.log`
  (server) or `%LOCALAPPDATA%\Foghorn\client.log` (client). Redact anything
  sensitive.
- **Windows version** (`winver`) for client bugs.

If it is a security issue, do not open a public issue. Email the maintainer
directly instead.

## Proposing a change

Small fixes: open a pull request directly. Larger changes: open an issue first
so we can talk through the shape of it before you spend time on code.

The project has a deliberate philosophy that shapes what belongs in it:

- **One server binary, no external dependencies.** Anything with a package
  manager (a database, a message queue, a JavaScript framework) is out of
  scope.
- **The web console works offline.** No CDN, no web fonts, no external scripts.
- **The client works on any Windows 10 or 11 PC out of the box.** No runtime
  install, no admin rights to run.
- **Docs are part of the change.** If a feature or a flag is added, its
  documentation is part of the pull request, not a follow-up.

Ideas that are outside the philosophy above may still be worth doing as a fork
or a plug-in — please do not be discouraged from raising them.

## Development

See [docs/BUILDING.md](docs/BUILDING.md) for how to build from source.

### Server (Go)

- Format with `gofmt`. `go vet ./...` should pass.
- Tests live in `server/foghorn_test.go`. Add tests for new behaviour, and run
  `go test ./...` before opening a pull request.
- The server has no third-party dependencies beyond `golang.org/x/sys`, which
  is vendored. Please do not add more.

### Client (C#)

- The client targets .NET Framework 4.x and is compiled by the C# compiler
  that ships with Windows. Please **do not** use language features newer than
  C# 5 or APIs newer than .NET Framework 4.0 (see `client\build.cmd`).
- If Visual Studio suggests "modernising" the source, decline — that is what
  keeps the client buildable on any Windows PC without an SDK.

### Web console

- No build step, no framework, no npm. Vanilla JavaScript, hand-written CSS
  in `server/web/`, embedded into the server binary at build time.
- All user-supplied text goes through the DOM API as `textContent`. Never
  build markup by string concatenation.

### Docs

- Markdown, one thought per paragraph, plain English. Prefer verbs to
  buzzwords.
- Cross-links between docs use relative paths (`../README.md`, `SECURITY.md#…`)
  so they work on GitHub and in a local clone.
- Screenshots go in `docs/images/`.

## Commit messages

The first line is a short summary in the imperative — "Fix the TrimEnd call",
not "Fixed" or "Fixes". Explain **why** in the body if it is not obvious. If a
commit fixes an issue, mention it (`Closes #12`).

## Licence

By opening a pull request you agree that your contribution is offered under
the project's MIT licence.
