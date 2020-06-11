#!/bin/sh

buildDate=`date +"%F %T %z"`
goVersion=`go version`
goVersion=${goVersion#"go version "}

go build --ldflags "-X main.Version=v0.0.1 \
    -X main.SeaweedfsSupported=v1.54+ \
    -X main.GitCommit=$(git rev-parse HEAD) \
    -X 'main.BuidDate=$buildDate' \
    -X 'main.GoVersion=$goVersion'" \
    -o ./build/weeder ./cmd/weeder
