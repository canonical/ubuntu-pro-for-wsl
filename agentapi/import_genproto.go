package agentapi

// This is a workaround for an "ambiguous import error":
//
//    ../../../go/pkg/mod/google.golang.org/grpc@v1.75.0/status/status.go:35:2: ambiguous import: found package google.golang.org/genproto/googleapis/rpc/status in multiple modules:
//	  google.golang.org/genproto v0.0.0-20200825200019-8632dd797987 (~/go/pkg/mod/google.golang.org/genproto@v0.0.0-20200825200019-8632dd797987/googleapis/rpc/status)
//	  google.golang.org/genproto/googleapis/rpc v0.0.0-20250818200422-3122310a409c (~/go/pkg/mod/google.golang.org/genproto/googleapis/rpc@v0.0.0-20250818200422-3122310a409c/status)
//    ~/go/pkg/mod/google.golang.org/grpc@v1.75.0/internal/status/status.go:34:6: could not import google.golang.org/genproto/googleapis/rpc/status (invalid package name: "")
//    ~/go/pkg/mod/google.golang.org/grpc@v1.75.0/status/status.go:35:6: could not import google.golang.org/genproto/googleapis/rpc/status (invalid package name: "")
//
// The error goes away when `google.golang.org/genproto` is added to go.mod, for example via
// `go get google.golang.org/genproto`. However, `go mod tidy` removes it again, unless we
// actually import it in the code, which is why we import it here.
//
// https://github.com/googleapis/go-genproto/issues/1015 seems to be related.
//
//nolint:revive // We want a blank import here (see comment above).
import _ "google.golang.org/genproto/protobuf/ptype"
