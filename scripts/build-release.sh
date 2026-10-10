#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd -P)"
RELEASE_TAG="${1:-}"

die() {
  printf 'build-release: %s\n' "$1" >&2
  exit 1
}

[[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] \
  || die 'usage: scripts/build-release.sh vX.Y.Z'

for command in go tar gzip sha256sum git; do
  command -v "$command" >/dev/null 2>&1 || die "missing command: $command"
done

[[ -f "$REPO_ROOT/gradlew" ]] || die 'Gradle wrapper is missing'
[[ -z "$(git -C "$REPO_ROOT" status --porcelain)" ]] \
  || die 'working tree must be clean before building a release'

TAG_COMMIT="$(git -C "$REPO_ROOT" rev-parse "$RELEASE_TAG^{commit}" 2>/dev/null || true)"
HEAD_COMMIT="$(git -C "$REPO_ROOT" rev-parse HEAD)"
[[ -n "$TAG_COMMIT" && "$TAG_COMMIT" == "$HEAD_COMMIT" ]] \
  || die "checkout must be exactly at tag $RELEASE_TAG"

RELEASE_VERSION="${RELEASE_TAG#v}"
ARCHIVE_NAME="map-data-fetcher-${RELEASE_TAG}-linux-amd64"
DIST_DIR="$REPO_ROOT/dist"
STAGE_DIR="$DIST_DIR/$ARCHIVE_NAME"
SOURCE_DATE_EPOCH="$(git -C "$REPO_ROOT" log -1 --format=%ct "$RELEASE_TAG")"

rm -rf -- "$STAGE_DIR"
mkdir -p "$STAGE_DIR/bin" "$STAGE_DIR/lib" "$STAGE_DIR/systemd" "$STAGE_DIR/config"

printf 'Building Java worker %s...\n' "$RELEASE_VERSION"
bash "$REPO_ROOT/gradlew" --no-daemon -PreleaseVersion="$RELEASE_VERSION" clean bootJar
install -Dm644 \
  "$REPO_ROOT/build/libs/map-data-fetcher-${RELEASE_VERSION}.jar" \
  "$STAGE_DIR/lib/map-data-fetcher.jar"

printf 'Building static Linux x86-64 gateway %s...\n' "$RELEASE_TAG"
(
  cd "$REPO_ROOT/gateway"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w -X github.com/YatinKare/map-data-fetcher/gateway/internal/version.Version=${RELEASE_TAG}" \
    -o "$STAGE_DIR/bin/map-data-gateway" \
    ./cmd/gateway
)

install -Dm644 "$REPO_ROOT/deploy/systemd/map-data-fetcher.service" \
  "$STAGE_DIR/systemd/map-data-fetcher.service"
install -Dm644 "$REPO_ROOT/config/gateway.env.example" \
  "$STAGE_DIR/config/gateway.env.example"
install -Dm644 "$REPO_ROOT/docs/INSTALL.md" "$STAGE_DIR/README.md"
sed "s/@RELEASE_TAG@/$RELEASE_TAG/g" "$REPO_ROOT/scripts/install.sh.in" \
  > "$STAGE_DIR/install.sh"
chmod 755 "$STAGE_DIR/install.sh"

ARCHIVE_PATH="$DIST_DIR/${ARCHIVE_NAME}.tar.gz"
CHECKSUM_PATH="${ARCHIVE_PATH}.sha256"
mkdir -p "$DIST_DIR"
tar --sort=name --mtime="@${SOURCE_DATE_EPOCH}" --owner=0 --group=0 \
  --numeric-owner -cf - -C "$DIST_DIR" "$ARCHIVE_NAME" \
  | gzip -n > "$ARCHIVE_PATH"
(cd "$DIST_DIR" && sha256sum "$(basename -- "$ARCHIVE_PATH")" > "$CHECKSUM_PATH")
cp "$STAGE_DIR/install.sh" "$DIST_DIR/install.sh"

printf '\nRelease files created in %s:\n' "$DIST_DIR"
printf '  %s\n  %s\n  %s\n' \
  "$ARCHIVE_PATH" "$CHECKSUM_PATH" "$DIST_DIR/install.sh"
