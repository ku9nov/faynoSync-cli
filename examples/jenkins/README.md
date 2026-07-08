# Jenkins integration

A ready-to-use Jenkins Shared Library step that wraps `faynosync-cli upload`.
It writes the changelog to a file (avoids CI shell-escaping problems), runs the
upload under a credentials binding, and marks the build `unstable` if the CLI
response does not contain `Upload completed`.

- [`faynosyncUpload.groovy`](./faynosyncUpload.groovy) — the shared step.

## Prerequisites

1. **`faynosync-cli` on the agent** — the binary must be on the agent's `PATH`.
   If it lives in `~/bin`, wrap the call site with
   `withEnv(["PATH+CUSTOM=${env.HOME}/bin"]) { ... }`.

2. **Config on the agent** — `~/.faynosync/config.yaml` with `server`, `owner`,
   `tuf`. Create it once with `faynosync-cli init`, or override at runtime with
   the `FAYNOSYNC_URL` / `FAYNOSYNC_ACCOUNT` env vars.

3. **API token as a Jenkins credential** — add a *Secret text* credential holding
   the FaynoSync token. The step reads it via `credentialsId` and exposes it to
   the CLI as `FAYNOSYNC_TOKEN`. Default id is `faynosync-token-<app>`.

## Install the shared library

**Manage Jenkins → System → Global Pipeline Libraries → Add:**

- **Name:** `faynosync-shared` (used in `@Library('faynosync-shared')`)
- **Default version:** e.g. `main`
- **Retrieval method:** Modern SCM → Git → your library repo URL
- Copy `faynosyncUpload.groovy` into that repo under `vars/faynosyncUpload.groovy`
  (the file name is the step name).

> A shared-library step must live in a `vars/` directory of the library repo.
> The `Library Path` field (from the *Pipeline: Groovy Libraries* plugin) lets
> you point at a subdirectory if the repo holds more than the library.

## Use it in a pipeline

```groovy
@Library('faynosync-shared') _

pipeline {
    agent any
    stages {
        stage('Upload') {
            steps {
                script {
                    def response = faynosyncUpload(
                        app:           'myapp',
                        version:       "1.2.3.${env.BUILD_NUMBER}",
                        channel:       'stable',
                        file:          './dist/myapp',            // String or List of paths
                        platform:      'linux',
                        arch:          'amd64',
                        changelog:     "Branch: ${env.BRANCH_NAME}",
                        credentialsId: 'faynosync-token-myapp',
                        publish:       true,
                        critical:      false,
                        intermediate:  false
                    )
                    echo response
                }
            }
        }
    }
}
```

## Parameters

| Key | Required | Default | Notes |
| --- | --- | --- | --- |
| `app` | yes | — | Application name. |
| `version` | yes | — | Version string. |
| `channel` | yes | — | Release channel. |
| `file` | no | `./<app>` | `String` for one file, or a `List` → multiple `--file` flags (e.g. `.deb` + `.rpm`). |
| `platform` | no | `linux` | |
| `arch` | no | `amd64` | |
| `credentialsId` | no | `faynosync-token-<app>` | Jenkins *Secret text* credential id → `FAYNOSYNC_TOKEN`. |
| `changelog` | no | `''` | Written to `CUSTOM_CHANGELOG.md`, passed via `--changelog-file`. |
| `publish` | no | `false` | Adds `--publish`. |
| `critical` | no | `false` | Adds `--critical`. |
| `intermediate` | no | `false` | Adds `--intermediate`. |
| `updater` | no | `''` | Adds `--updater <value>` when set. |
| `signature` | no | `''` | Adds `--signature <value>` when set (Tauri base64 signature). |

The step returns the raw CLI response string.
