# faynosync CLI

`faynoSync-cli` is a command line tool focused on uploading new application versions to FaynoSync in a native and predictable way.

The main problem it solves is inconsistent shell escaping in some CI environments. Instead of relying on CI-specific quoting behavior, this CLI provides a unified upload flow and stable changelog input modes.

## Runtime settings priority

- `FAYNOSYNC_TOKEN` is required and loaded only from environment.
- `server` is loaded from config and can be overridden by `FAYNOSYNC_URL`.
- `owner` is loaded from config and can be overridden by `FAYNOSYNC_ACCOUNT`.

## Commands

Global flag:

- `--log-level <level>` where level is `trace|debug|info|warn|error|fatal|panic` (default: `info`)

### `faynosync init`

Creates `~/.faynosync/config.yaml` and prompts for `server`, `owner`, and `tuf`.

Default config:

```yaml
server: https://example.com
owner: example
tuf: false
```

### `faynosync config view`

Prints current config from `~/.faynosync/config.yaml`.

### `faynosync config set <server|owner|tuf> [value]`

Updates a config field. If `value` is not provided, CLI prompts for it.

### `faynosync upload [flags]`

Uploads one or more files to `<server>/upload` using `multipart/form-data`.

Authentication and server source:

- `FAYNOSYNC_TOKEN` (required) is sent as `Authorization: Bearer <token>`.
- `server` comes from config or `FAYNOSYNC_URL`.

Core flags:

- `--app <name>`
- `--file <path>` (repeatable, at least one required)
- `--version <value>`
- `--channel <value>`
- `--platform <value>`
- `--arch <value>`
- `--updater <value>` where value is `manual|velopack|squirrel_darwin|squirrel_windows|electron-builder|tauri` (validated locally)
- `--signature <value>` Tauri base64 signature (e.g. `--signature "$(cat myapp.app.tar.gz.sig)"`)
- `--publish[=true|false]`
- `--critical[=true|false]`
- `--intermediate[=true|false]`
- `--changelog <text>`
- `--changelog-file <path>`
- `--changelog-stdin`

Important: changelog input modes are mutually exclusive. Use only one of `--changelog`, `--changelog-file`, or `--changelog-stdin`.

For Markdown with special symbols, prefer `--changelog-file` or `--changelog-stdin`.

## Upload examples

```bash
faynosync upload \
  --app test \
  --file ./test.apk \
  --version 1.2.3 \
  --channel stable \
  --platform android \
  --arch universal \
  --publish \
  --critical \
  --intermediate \
  --changelog "Bugfixes"

faynosync upload --file ./test.rpm --file ./test.deb --app myapp --publish=true

faynosync upload \
  --app myapp \
  --file ./myapp.app.tar.gz \
  --version 1.0.0 \
  --channel stable \
  --platform darwin \
  --arch amd64 \
  --updater tauri \
  --signature "$(cat ./myapp.app.tar.gz.sig)" \
  --publish

faynosync upload --file ./test.rpm --app myapp --changelog-file ./CHANGELOG.md

cat ./CHANGELOG.md | faynosync upload --file ./test.rpm --app myapp --changelog-stdin

# Simple stdin example
go run main.go upload \
--app=cli \
--file=./faynoSync-cli \
--version=0.0.0.1 \
--channel=nightly \
--platform=linux \
--arch=amd64 \
--publish \
--critical \
--intermediate \
--changelog-stdin <<EOF
# Changes
- fixed ! bug
- added ${feature}
EOF

# Sometimes it's recommended to use 'EOF' (quoted heredoc delimiter) because shells may try to parse parameter expansion before here-doc formation
go run main.go upload \
  --app=cli \
  --file=./faynoSync-cli \
  --version=0.0.0.1 \
  --channel=nightly \
  --platform=linux \
  --arch=amd64 \
  --publish \
  --critical \
  --intermediate \
  --changelog-stdin <<'EOF'
# Changes
- fixed ! bug
- added ${feature}
- ]!-%^:;"{<+"\&££,!#>${$]>|:=?£:^[(`<):.&.(@{:"@=>
EOF
```

## CI / GitHub Actions

This repository ships a composite action (`action.yml` in the repo root). It downloads the prebuilt CLI binary matching the runner OS/arch from the release you pin to, then runs `faynosync upload`. No Go toolchain is needed on the runner.

```yaml
- name: Upload to faynoSync
  uses: ku9nov/faynoSync-cli@v1
  with:
    app: myapp
    file: |
      ./dist/myapp.deb
      ./dist/myapp.rpm
    version: 1.2.3
    channel: stable
    platform: linux
    arch: amd64
    publish: true
    changelog-file: ./CHANGELOG.md
  env:
    FAYNOSYNC_TOKEN: ${{ secrets.FAYNOSYNC_TOKEN }}
    FAYNOSYNC_URL: ${{ secrets.FAYNOSYNC_URL }}
    FAYNOSYNC_ACCOUNT: ${{ secrets.FAYNOSYNC_ACCOUNT }}
```

Notes:

- Pin the action to the floating major tag `@v1` to get patches automatically, or to an exact release like `@v1.0.0`. The CLI binary version is derived from that tag (`@v1` resolves to the latest release). Override it with the `cli-version` input if needed.
- `file` accepts multiple paths (one per line) — each becomes a separate `--file`.
- `FAYNOSYNC_TOKEN` is required. `FAYNOSYNC_URL` and `FAYNOSYNC_ACCOUNT` override the config `server`/`owner`. Keep all three in `secrets` — values referenced via `${{ secrets.* }}` are automatically masked in logs, and the CLI never prints the token or server URL.
- Boolean inputs: `publish`, `critical`, `intermediate` (add the flag when `true`).
- A full example workflow lives in [`examples/github-actions`](./examples/github-actions).

## CI / Jenkins

A ready-to-use Jenkins Shared Library step wrapping `faynosync upload`, with setup instructions, lives in [`examples/jenkins`](./examples/jenkins).

## Build and run

```bash
go build -o faynosync-cli .
./faynosync-cli --log-level info init
./faynosync-cli config view
./faynosync-cli config set server https://updates.example.com
./faynosync-cli upload --file ./test.apk --app myapp --version 1.2.3 --publish
```