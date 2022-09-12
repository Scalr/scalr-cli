#!/bin/bash
set -e

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

    go build -ldflags="-s -w" -o "bin/scalr-$GOOS-$GOARCH$EXT" .
  done
done
