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

Creates `~/.faynosync/config.yaml` and prompts for `server` and `owner`.

Default config:

```yaml
server: https://example.com
owner: example
```

### `faynosync config view`

Prints current config from `~/.faynosync/config.yaml`.

### `faynosync config set <server|owner> [value]`

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

# It is highly recommended to use 'EOF' (quoted heredoc delimiter) because shells may try to parse parameter expansion before here-doc formation
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

## Build and run

```bash
go build -o faynosync .
./faynosync --log-level info init
./faynosync config view
./faynosync config set server https://updates.example.com
./faynosync upload --file ./test.apk --app myapp --version 1.2.3 --publish
```