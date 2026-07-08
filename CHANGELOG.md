# Changelog

## 0.14.0

- Added `--updater` flag to `upload` for selecting the updater type (`manual`, `velopack`, `squirrel_darwin`, `squirrel_windows`, `electron-builder`, `tauri`); the value is validated locally and sent in the upload `data`.
- Added `--signature` flag to `upload` for passing the Tauri base64 signature in the upload `data`.

## 0.13.0

- Added a persistent per-device ID, stored in the config directory and sent as the `X-Device-ID` header on update checks.
- Switched update checks to the faynoSync Go SDK instead of direct HTTP calls.
- Added optional edge/CDN support via an `edge` setting (`config set edge` or `FAYNOSYNC_EDGE`); update checks try the edge first and fall back to the API.
- Logged the response `source` (edge or api) during upgrades.
- Fixed `upload` to fail with a non-zero exit code when the server rejects the request.
- Improved log readability with colorized, aligned, multi-line output (auto-disabled for non-terminals and when `NO_COLOR` is set).

## 0.12.0

- Added a full self-update flow with explicit update checks and user-facing upgrade command integration.
- Added release artifact download support and binary replacement logic for in-place CLI upgrades.
- Added TUF-based metadata and target download support, including a dedicated TUF config path for update workflows.
- Added rollback confirmation prompt to make downgrade/rollback scenarios explicit and safer during upgrades.
- Expanded automated test coverage for upgrade flows and enabled CI test workflow for recent update changes.

## 0.11.0
- Added version command to print the version of the CLI

## 0.10.0

- Added `upload` command with support for file uploads and metadata flags.
- Added changelog input modes: `--changelog`, `--changelog-file`, and `--changelog-stdin` (mutually exclusive).