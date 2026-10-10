# Build a release bundle

The release build supports Linux x86-64 and requires Git, Java 17, Go 1.25.0,
GNU tar, gzip, and `sha256sum`. The Gradle wrapper downloads the pinned Gradle
version on its first run.

From a clean checkout at the exact release tag, run:

```bash
bash scripts/build-release.sh v0.1.0
```

The archive, checksum file, and version-specific installer are written under
`dist/`. The GitHub Actions release workflow runs the same command for pushed
`v*` tags and publishes those files to the matching GitHub Release.
