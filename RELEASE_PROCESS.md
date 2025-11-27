# Release Process

This document describes how to create a new release for Abbie.

## Prerequisites

- Write access to the GitHub repository
- All tests passing on `master` branch
- No breaking changes without version bump

## Release Steps

### 1. Prepare the Release

Ensure all changes are merged to `master` and CI is passing:

```bash
git checkout master
git pull origin master
```

### 2. Create and Push Tag

Use semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backwards compatible)
- **PATCH**: Bug fixes

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0: Initial stable release"

# Push tag to trigger release workflow
git push origin v1.0.0
```

### 3. Automated Release Process

Once the tag is pushed, GitHub Actions will automatically:

1. **Build Binaries**
   - Linux (amd64, arm64, arm)
   - macOS (amd64, arm64)
   - Windows (amd64)

2. **Create Docker Images**
   - Multi-arch images (amd64, arm64)
   - Push to `ghcr.io/davidhoenisch/abbie:v1.0.0`
   - Update `latest` tag

3. **Generate Release Notes**
   - Auto-generated changelog from commits
   - Grouped by feature/fix/perf
   - Includes checksums

4. **Publish GitHub Release**
   - All binaries attached
   - Docker pull instructions
   - Installation guide

### 4. Verify Release

After the workflow completes (~5-10 minutes):

1. Check [Releases page](https://github.com/DavidHoenisch/abbie/releases)
2. Verify all artifacts are present
3. Test Docker image:
   ```bash
   docker pull ghcr.io/davidhoenisch/abbie:v1.0.0
   docker run ghcr.io/davidhoenisch/abbie:v1.0.0 -h
   ```
4. Test binary download (pick your platform):
   ```bash
   wget https://github.com/DavidHoenisch/abbie/releases/download/v1.0.0/abbie_Linux_x86_64.tar.gz
   tar -xzf abbie_Linux_x86_64.tar.gz
   ./abbie -h
   ```

## Troubleshooting

### Release Workflow Failed

1. Check GitHub Actions logs
2. Common issues:
   - Docker push permissions (check GITHUB_TOKEN)
   - Go build errors (test locally first)
   - Invalid .goreleaser.yml syntax

### Need to Re-release

If you need to rebuild a release:

```bash
# Delete the tag locally and remotely
git tag -d v1.0.0
git push origin :refs/tags/v1.0.0

# Delete the GitHub release (via UI or gh CLI)
gh release delete v1.0.0

# Fix issues, then create tag again
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

## Testing Releases Locally

You can test the release process locally without publishing:

```bash
# Install goreleaser
brew install goreleaser  # macOS
# or
go install github.com/goreleaser/goreleaser@latest

# Test the configuration
goreleaser check

# Build snapshot (doesn't publish)
goreleaser release --snapshot --clean

# Check output in ./dist/
```

## Versioning Guidelines

Follow semantic versioning:

- `v0.x.x` - Pre-release, breaking changes allowed
- `v1.0.0` - First stable release
- `v1.x.x` - Stable, backwards compatible features
- `v2.0.0` - Breaking changes

## Post-Release

After a successful release:

1. Announce in relevant channels
2. Update documentation if needed
3. Monitor for issues
4. Plan next release

## Emergency Hotfix

For critical bugs in production:

```bash
# Create hotfix from release tag
git checkout -b hotfix/v1.0.1 v1.0.0

# Fix the bug
# ... make changes ...

# Commit and tag
git commit -m "fix: critical bug in routing"
git tag -a v1.0.1 -m "Hotfix v1.0.1: Fix critical routing bug"
git push origin hotfix/v1.0.1
git push origin v1.0.1

# Merge back to master
git checkout master
git merge hotfix/v1.0.1
git push origin master
```
