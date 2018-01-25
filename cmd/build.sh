#!/bin/bash

cd "$( dirname "${BASH_SOURCE[0]}" )"

version=1.2
echo "creating tool binary version $version"

mkdir -p ../bin
build() {
    local name
    local goos
    local goarch
    local goarm
    local cgo
    local armv

    goos="GOOS=$1"
    goarch="GOARCH=$2"
    arch=$3
    if [[ $2 == "arm" ]]; then
        armv=`echo $arch | grep -o [0-9]`
        goarm="GOARM=$armv"
    fi

    name=redis_tool-$arch-$version
    echo "building $name"
    echo $cgo $goos $goarch $goarm go build \"-ldflags=-s -w\"
    eval $cgo $goos $goarch $goarm go build \"-ldflags=-s -w\" || exit 1

    if [[ $1 == "windows" ]]; then
        mv cmd.exe ../bin/$name.exe
    else
        mv cmd ../bin/$name
    fi
}

build darwin amd64 mac64
build linux amd64 linux64
build windows amd64 win64
