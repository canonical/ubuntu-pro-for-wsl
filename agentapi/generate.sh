#!/bin/sh
set -eu

PATH=$PATH:$(go env GOPATH)/bin protoc --proto_path=. --go_out=go/ --go_opt=paths=source_relative --go-grpc_out=go/ --go-grpc_opt=paths=source_relative agentapi.proto
PATH=$PATH:${PUB_CACHE:-"$HOME/.pub-cache"}/bin protoc --proto_path=. --dart_out=grpc:dart/lib/src/ agentapi.proto
