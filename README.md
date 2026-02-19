# faynosync CLI

Simple CLI skeleton for managing local `faynosync` configuration.

## Project structure

- `main.go` - entrypoint that wires stdin/stdout and runs CLI.
- `internal/cli` - command routing and user interaction.
- `internal/config` - config file path, read/write, defaults, and field updates.

## Commands

Global flag:

- `--log-level <level>` where level is `trace|debug|info|warn|error|fatal|panic` (default: `info`)

### `faynosync init`

Creates:

- `~/.faynosync/config.yaml`

Prompts for `server` and `owner` values (press Enter to use defaults).

Default content:

```yaml
server: https://example.com
owner: example
```

### `faynosync config view`

Prints current config from `~/.faynosync/config.yaml`.

### `faynosync config set <server|owner> [value]`

Updates a config field. If `value` is omitted, CLI prompts for it.

Examples:

```bash
faynosync --log-level info init
faynosync config set server https://updates.example.com
faynosync config set owner my-team
faynosync config set owner
```

## Build and run

```bash
go build -o faynosync .
./faynosync --log-level info init
./faynosync config view
./faynosync config set server https://updates.example.com
```