package streams

import (
	"context"

	"google.golang.org/grpc"
)

// MultiClient represents a connected multiClient to the Windows Agent.
// It abstracts away the multiple streams into a single object.
// It only provides communication primitives, it does not handle the logic of the messages themselves.
type MultiClient = multiClient

// connect connects to the three streams. Call Close to release resources.
func Connect(ctx context.Context, conn *grpc.ClientConn) (c *MultiClient, err error) {
	return connect(ctx, conn)
}
