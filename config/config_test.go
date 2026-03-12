package config_test

import (
	"testing"

	"github.com/f1bonacc1/ha-store/config"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Test default values
	cfg := config.Load()
	assert.Equal(t, "nats://localhost:4222", cfg.NATSURL)
	assert.Equal(t, "8090", cfg.Port)
	assert.Equal(t, 1, cfg.Replicas)

	// Test custom values
	t.Setenv("NATS_URL", "nats://demo.nats.io:4222")
	t.Setenv("PORT", "9090")
	t.Setenv("REPLICAS", "3")
	cfg = config.Load()
	assert.Equal(t, "nats://demo.nats.io:4222", cfg.NATSURL)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, 3, cfg.Replicas)
}
