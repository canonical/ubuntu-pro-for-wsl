#!/bin/sh
set -eu

PATH=$PATH:$(go env GOPATH)/bin protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative wslserviceapi.proto
