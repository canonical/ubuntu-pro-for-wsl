module github.com/canonical/ubuntu-pro-for-wsl/generate

go 1.26.0

require (
	github.com/canonical/ubuntu-pro-for-wsl/windows-agent v0.0.0-00010101000000-000000000000
	github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service v0.0.0-20260824122752-d9c61076a5c5
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	go.yaml.in/yaml/v3 v3.0.5
)

require (
	github.com/canonical/landscape-hostagent-api v0.0.0-20250919154603-590e7d7ae4e1 // indirect
	github.com/canonical/ubuntu-pro-for-wsl/agentapi v0.0.0-20260824122752-d9c61076a5c5 // indirect
	github.com/canonical/ubuntu-pro-for-wsl/common v0.0.0-20260824122752-d9c61076a5c5 // indirect
	github.com/canonical/ubuntu-pro-for-wsl/contractsapi v0.0.0-20260824122752-d9c61076a5c5 // indirect
	github.com/canonical/ubuntu-pro-for-wsl/storeapi/go-wrapper/microsoftstore v0.0.0-20260824122752-d9c61076a5c5 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.3.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.12.0 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/snapcore/go-gettext v0.0.0-20230721153050-9082cdc2db05 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/ubuntu/decorate v0.0.0-20250213124239-8228e241ee19 // indirect
	github.com/ubuntu/gowsl v0.0.0-20251112191800-0ef2623cc8fb // indirect
	golang.org/x/exp v0.0.0-20260527015227-08cc5374adb3 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/ini.v1 v1.67.3 // indirect
)

replace github.com/canonical/ubuntu-pro-for-wsl/windows-agent => ../windows-agent
