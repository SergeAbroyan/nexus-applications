package valkey

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/config"
)

type Cache struct {
	client *redis.Client
}

func Connect(ctx context.Context, cfg config.Valkey, awsConfig aws.Config) (*Cache, error) {
	options := &redis.Options{Addr: cfg.Address}

	if cfg.TLS {
		host, _, err := net.SplitHostPort(cfg.Address)
		if err != nil {
			return nil, fmt.Errorf("parse Valkey address: %w", err)
		}
		options.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	}

	if cfg.IAMAuthentication {
		authenticator := iamAuthenticator{
			credentials: awsConfig.Credentials,
			signer:      v4.NewSigner(),
			region:      awsConfig.Region,
			cacheName:   strings.ToLower(cfg.CacheName),
			username:    cfg.Username,
		}
		options.CredentialsProviderContext = authenticator.credentialsFor
	}

	client := redis.NewClient(options)
	if err := redisotel.InstrumentTracing(client, redisotel.WithDBStatement(false)); err != nil {
		return nil, errors.Join(err, client.Close())
	}

	cache := &Cache{client: client}
	if err := cache.Ping(ctx); err != nil {
		return nil, errors.Join(err, client.Close())
	}
	return cache, nil
}

func (cache *Cache) Ping(ctx context.Context) error {
	return cache.client.Ping(ctx).Err()
}

func (cache *Cache) Close() error {
	return cache.client.Close()
}

func (cache *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := cache.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (cache *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return cache.client.Set(ctx, key, value, ttl).Err()
}

func (cache *Cache) Delete(ctx context.Context, key string) error {
	return cache.client.Del(ctx, key).Err()
}
