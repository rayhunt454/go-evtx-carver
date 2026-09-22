#!/bin/bash
rm -f "cmd/build/*"
mkdir -p "cmd/build"


for p in "windows/amd64/.exe" "windows/386/.exe" "linux/amd64/" "linux/386/" "linux/arm64/" "darwin/amd64/" "darwin/arm64/"; do
    IFS='/' read -r os arch ext <<< "$p"
    echo "Building $os/$arch..."
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -ldflags="-s -w" -o "cmd/build/goevtxcarver-$os-$arch$ext" "cmd/evtx/main.go"
done

echo "Done!"
ls -lh cmd/build/