# Changelog

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