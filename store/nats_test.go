package store_test

import (
	"testing"

	"github.com/f1bonacc1/ha-store/store"

	"github.com/stretchr/testify/assert"
)

func TestNew_Error(t *testing.T) {
	// Invalid URL
	s, err := store.New("invalid-url", 1)
	assert.Error(t, err)
	assert.Nil(t, s)
}
