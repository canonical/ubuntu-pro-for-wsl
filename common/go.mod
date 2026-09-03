module github.com/canonical/ubuntu-pro-for-wsl/common

go 1.26.0

require (
	github.com/golang/protobuf v1.5.4
	github.com/google/uuid v1.6.0
	github.com/sirupsen/logrus v1.10.1
	github.com/snapcore/go-gettext v0.0.0-20230721153050-9082cdc2db05
	github.com/stretchr/testify v1.12.1
	github.com/ubuntu/decorate v0.0.0-20250213124239-8228e241ee19
	github.com/ubuntu/gowsl v0.0.0-20251112191800-0ef2623cc8fb
	go.yaml.in/yaml/v3 v3.0.5
	google.golang.org/grpc v1.83.2
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)
