//go:build integration

package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/testcontainers/testcontainers-go"
	tcvalkey "github.com/testcontainers/testcontainers-go/modules/valkey"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/config"
)

func TestCacheAgainstValkey(t *testing.T) {
	ctx := context.Background()

	container, err := tcvalkey.Run(ctx, "valkey/valkey:9.1.3-alpine3.24")
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start Valkey: %v", err)
	}

	address, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("read Valkey address: %v", err)
	}

	cache, err := Connect(ctx, config.Valkey{Address: address}, aws.Config{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer cache.Close()

	if _, found, err := cache.Get(ctx, "items"); err != nil || found {
		t.Fatalf("empty cache returned found=%v err=%v", found, err)
	}

	if err := cache.Set(ctx, "items", []byte(`[{"id":1}]`), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, found, err := cache.Get(ctx, "items")
	if err != nil || !found || string(value) != `[{"id":1}]` {
		t.Fatalf("get returned %q found=%v err=%v", value, found, err)
	}

	if err := cache.Delete(ctx, "items"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, found, err := cache.Get(ctx, "items"); err != nil || found {
		t.Fatalf("deleted key returned found=%v err=%v", found, err)
	}
}
