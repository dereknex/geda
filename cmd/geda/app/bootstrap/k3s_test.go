package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartCluster(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), 300*time.Second)
	defer stop()
	cfg, err := StartCluster(ctx)
	assert.NotNil(t, cfg)
	assert.Nil(t, err, err)
}
