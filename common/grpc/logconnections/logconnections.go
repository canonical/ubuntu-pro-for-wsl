// Package logconnections implements a stream interceptor to notify the pinged object on each new and ended connections.
package logconnections

import (
	"fmt"
	"reflect"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"google.golang.org/grpc"
)

type loggedServerStream struct {
	grpc.ServerStream
}

// StreamServerInterceptor notifies the pingued object on each new and ended connections.
// If the pingued object implements onNewConnectionner, it will have OnNewConnection called when the connection is established (can be used for logging for instance)
// If the pingued object implements onDoneConnectionner, it will have OnDoneConnection called when the connection was handled by the server (can be used to reset an internal timeout for instance).
func StreamServerInterceptor() func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		loggedss := loggedServerStream{
			ServerStream: ss,
		}

		if info != nil {
			log.Debugf(ss.Context(), "New request %s", info.FullMethod)
		}

		// We don't want to send the closing logs to the client, as the client
		// will log the closing of the connection itself.
		ctx := log.WithoutRemoteSend(ss.Context())

		defer func() {
			if info != nil {
				log.Debugf(ctx, "Request %s done", info.FullMethod)
			}
		}()
		err := handler(srv, loggedss)
		if err != nil {
			log.Infof(ctx, "Error sent to client: %v", err)
		}
		return err
	}
}

func (ss loggedServerStream) RecvMsg(m interface{}) error {
	var msg string
	err := ss.ServerStream.RecvMsg(m)
	v := reflect.ValueOf(m).Elem()
	t := v.Type()
	for i := range t.NumField() {
		n := t.Field(i).Name
		// Only print exported fields
		val := v.FieldByName(n)
		if !val.CanSet() {
			continue
		}
		msg += fmt.Sprintf("%s: %v, ", n, val)
	}

	log.Debugf(ss.Context(), "Requesting with parameters: %s", strings.TrimSuffix(msg, ", "))

	return err
}
