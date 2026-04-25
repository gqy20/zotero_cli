# GitHub Actions

This repository now includes a baseline CI/CD setup for the `zot` CLI.

## CI

Workflow: `.github/workflows/ci.yml`

- Runs on pushes to `main` and `master`
- Runs on every pull request
- Checks Go formatting with `gofmt`
- Runs `go test ./...`
- Verifies the CLI builds successfully
- Cross-builds release-style binaries for:
  - Linux `amd64`
  - Linux `arm64`
  - Windows `amd64`
  - macOS `amd64`
  - macOS `arm64`

## CD

Workflow: `.github/workflows/release.yml`

- Triggers when a tag matching `v*` is pushed, for example `v0.0.1`
- Re-runs the test suite before packaging
- Builds release archives for the same target platforms
- Injects version, commit, and build date into the binary at build time
- Generates a `checksums.txt` file for all release artifacts
- Publishes a GitHub Release and uploads packaged binaries automatically
- If `HOMEBREW_TAP_TOKEN` is configured, updates `gqy20/homebrew-tap` automatically
- Uploads release artifacts, checksums, `version.json`, and `latest` to Qiniu CDN

## Homebrew tap automation

The release workflow can update the external tap repository after a tagged release is published.

Required secret in this repository:

- `HOMEBREW_TAP_TOKEN`
  - Recommended: a fine-grained personal access token
  - Repository access: `gqy20/homebrew-tap`
  - Repository permissions: `Contents: Read and write`

Behavior:

- Downloads the release artifacts produced in the same workflow run
- Computes the macOS and Linux archive SHA256 values
- Rewrites `Formula/zotcli.rb` in `gqy20/homebrew-tap`
- Commits and pushes the tap update to `main`

If `HOMEBREW_TAP_TOKEN` is not configured, the release workflow still publishes the GitHub Release and simply skips the tap update job.

## Qiniu CDN automation

The release workflow treats Qiniu upload as part of the official release path.

Required secrets in this repository:

- `QINIU_ACCESS_KEY`
- `QINIU_SECRET_KEY`

Behavior:

- Fails before packaging when a tagged release is missing required Qiniu secrets
- Uploads the same release artifacts as GitHub Release under `github/zotero_cli/<tag>/`
- Uploads CDN `checksums.txt`
- Updates `github/zotero_cli/version.json` and `github/zotero_cli/latest`

## Suggested release flow

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

If you want to test packaging manually from GitHub without creating a tag, you can use the `workflow_dispatch` trigger on the Release workflow. That manual run will build artifacts but will not publish a GitHub Release unless the workflow is running on a tag.
