#!/bin/bash
set -e

VERSION=${1:-0.0.0}

declare -a os=("linux" "windows" "darwin")
declare -a arch=("386" "amd64" "arm" "arm64")

for (( j=0; j<${#os[@]}; j++ ));
do
  GOOS="${os[$j]}"
  for (( i=0; i<${#arch[@]}; i++ ));
  do
    GOARCH="${arch[$i]}"
    EXT=""

    if [ $GOOS == "windows" ]; then
      EXT=".exe"
    fi

    BINARY="scalr-cli_${VERSION}_${GOOS}_${GOARCH}${EXT}"
    go build -ldflags="-s -w" -o bin/$BINARY .
    cd bin
      chmod +x $BINARY
      PACKAGE="scalr-cli_${VERSION}_${GOOS}_${GOARCH}.zip"
      zip -9 $PACKAGE $BINARY
      sha256sum $PACKAGE >> "scalr-cli_${VERSION}_SHA256SUMS"
    cd ..
  done
done
