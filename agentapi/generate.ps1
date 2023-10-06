${env:PATH}="${env:PATH};$(go env GOPATH)\bin"
${env:PATH}="${env:PATH};${env:LocalAppData}\Pub\Cache\bin"

protoc.exe --proto_path=. --go_out="go/" --go_opt=paths=source_relative --go-grpc_out="go/" --go-grpc_opt=paths=source_relative "agentapi.proto"
protoc.exe --proto_path=. --dart_out="grpc:dart/lib/src/" "agentapi.proto"