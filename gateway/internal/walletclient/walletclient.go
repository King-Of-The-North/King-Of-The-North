// Package walletclient is the Gateway's gRPC client to the Wallet service. The
// Gateway is the REST ingress; it never touches the ledger directly — every money
// operation routes through Wallet over gRPC (AGENTS.md: proto is the contract).
package walletclient

import (
	"fmt"

	walletv1 "github.com/king-of-the-north/king-of-the-north/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the generated WalletServiceClient plus its connection.
type Client struct {
	walletv1.WalletServiceClient
	conn *grpc.ClientConn
}

// Dial connects to the Wallet gRPC server at addr (e.g. "localhost:9091"). Plaintext
// for the demo — TLS/mTLS is a production concern, not this milestone.
func Dial(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("walletclient: dial %s: %w", addr, err)
	}
	return &Client{
		WalletServiceClient: walletv1.NewWalletServiceClient(conn),
		conn:                conn,
	}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }
