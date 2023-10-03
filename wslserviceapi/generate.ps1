${env:PATH}="${env:PATH};$(go env GOPATH)\bin"

protoc.exe --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative "wslserviceapi.proto"