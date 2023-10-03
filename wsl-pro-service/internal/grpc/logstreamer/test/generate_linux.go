// Package test contains the log test generated struct for logstreamer package tests.
//
// This is a separate package to not embed that structure in the finale binary.
package test

//go:generate sh -c "PATH=\"$PATH:`go env GOPATH`/bin\" protoc --go_out=. --go_opt=paths=source_relative log_test.proto"
