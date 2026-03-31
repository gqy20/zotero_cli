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

## Suggested release flow

```bash
git tag v0.0.1
git push origin v0.0.1
```

If you want to test packaging manually from GitHub without creating a tag, you can use the `workflow_dispatch` trigger on the Release workflow. That manual run will build artifacts but will not publish a GitHub Release unless the workflow is running on a tag.
