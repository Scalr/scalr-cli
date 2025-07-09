#!/bin/bash
set -e

# Get version from git tag or use provided version
if [ -z "$1" ]; then
  # Try to get version from git tag
  VERSION=$(git describe --tags --exact-match 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo "dev")
  # If we're not on a tag, append commit info
  if ! git describe --tags --exact-match >/dev/null 2>&1; then
    COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    VERSION="${VERSION}-${COMMIT_SHORT}"
  fi
else
  VERSION=$1
fi

# Get git commit hash
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

# Get build date
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building version: $VERSION"
echo "Build date: $BUILD_DATE"

declare -a os=("linux" "windows" "darwin")
declare -a arch=("386" "amd64" "arm" "arm64")

for (( j=0; j<${#os[@]}; j++ ));
do
  GOOS="${os[$j]}"
  for (( i=0; i<${#arch[@]}; i++ ));
  do
    GOARCH="${arch[$i]}"
    EXT=""

    # Skip unsupported combinations
    if [ $GOOS == 'darwin' ] && [ $GOARCH == '386' ]; then
      continue
    fi
    if [ $GOOS == 'darwin' ] && [ $GOARCH == 'arm' ]; then
      continue
    fi
    if [ $GOOS == 'windows' ]; then
      EXT=".exe"
    fi

    BINARY="scalr-cli_${VERSION}_${GOOS}_${GOARCH}${EXT}"

    # Build with embedded version information
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
      -ldflags="-s -w -X main.versionCLI=${VERSION} -X main.gitCommit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o bin/$BINARY \
      -a -ldflags '-extldflags "-static"' .

    cd bin
      chmod +x $BINARY
      mv $BINARY "scalr${EXT}"
      PACKAGE="scalr-cli_${VERSION}_${GOOS}_${GOARCH}.zip"
      zip -9 $PACKAGE "scalr${EXT}"
      sha256sum $PACKAGE >> "scalr-cli_${VERSION}_SHA256SUMS"
      rm "scalr${EXT}"
    cd ..
  done
done
