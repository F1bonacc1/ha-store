package testhelpers

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// StartEmbeddedNATS starts an embedded NATS server with JetStream enabled
func StartEmbeddedNATS(t *testing.T) *server.Server {
	opts := &server.Options{
		JetStream: true,
		Port:      -1, // Random port
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("Failed to create NATS server: %v", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatalf("NATS server not ready")
	}

	return ns
}

// ConnectNATS connects to the embedded NATS server
func ConnectNATS(t *testing.T, ns *server.Server) *nats.Conn {
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	return nc
}
