package store

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

const bucketName = "ha-store"

type Store struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	bucket   jetstream.ObjectStore
	replicas int
}

func New(url string, replicas int) (*Store, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.ReconnectBufSize(-1),
		nats.RetryOnFailedConnect(true),
	)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("failed to connect to nats")
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	// Wait for an active connection before proceeding
	for !nc.IsConnected() {
		log.Warn().Msg("waiting for NATS connection...")
		time.Sleep(time.Second)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		log.Error().Err(err).Msg("failed to create jetstream context")
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	// Initialize single shared bucket, retrying on transient stream errors
	// that occur when the JetStream cluster is still electing leaders
	var bucket jetstream.ObjectStore
	for attempt := range 10 {
		if attempt > 0 {
			log.Warn().Int("attempt", attempt+1).Msg("retrying object store initialization")
			time.Sleep(time.Second)
		}
		bucket, err = js.CreateOrUpdateObjectStore(context.Background(), jetstream.ObjectStoreConfig{
			Bucket:      bucketName,
			Description: "HA Store object storage",
			Replicas:    replicas,
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Error().Err(err).Str("bucket", bucketName).Int("replicas", replicas).Msg("failed to create object store")
		return nil, fmt.Errorf("failed to create object store: %w", err)
	}

	return &Store{
		nc:       nc,
		js:       js,
		bucket:   bucket,
		replicas: replicas,
	}, nil
}

func (s *Store) Close() {
	if s.nc != nil {
		s.nc.Close()
	}
}

func (s *Store) GetBucket() jetstream.ObjectStore {
	return s.bucket
}
