package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/redis/go-redis/v9"
)

const emptyBodySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

type valkeyCache struct {
	client *redis.Client
	ttl    time.Duration
}

func newValkeyCache(configuration config, credentials aws.CredentialsProvider) *valkeyCache {
	options := &redis.Options{
		Addr: configuration.ValkeyAddress,
		DB:   0,
	}

	if configuration.ValkeyTLS {
		host, _, _ := strings.Cut(configuration.ValkeyAddress, ":")
		options.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		}
	}

	if configuration.ValkeyIAMAuthentication {
		provider := &valkeyIAMCredentials{
			credentials: credentials,
			signer:      v4.NewSigner(),
			username:    configuration.ValkeyUsername,
			cacheName:   strings.ToLower(configuration.ValkeyCacheName),
			region:      configuration.AWSRegion,
		}
		options.CredentialsProviderContext = provider.Retrieve
	}

	return &valkeyCache{
		client: redis.NewClient(options),
		ttl:    configuration.ValkeyCacheTTL,
	}
}

func (cache *valkeyCache) Close() error {
	return cache.client.Close()
}

func (cache *valkeyCache) Ping(ctx context.Context) error {
	return cache.client.Ping(ctx).Err()
}

func (cache *valkeyCache) GetItems(ctx context.Context) ([]item, bool, error) {
	value, err := cache.client.Get(ctx, "nexus:items").Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var items []item
	if err := json.Unmarshal(value, &items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}

func (cache *valkeyCache) SetItems(ctx context.Context, items []item) error {
	value, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return cache.client.Set(ctx, "nexus:items", value, cache.ttl).Err()
}

func (cache *valkeyCache) DeleteItems(ctx context.Context) error {
	return cache.client.Del(ctx, "nexus:items").Err()
}

type valkeyIAMCredentials struct {
	credentials aws.CredentialsProvider
	signer      *v4.Signer
	username    string
	cacheName   string
	region      string
}

func (provider *valkeyIAMCredentials) Retrieve(ctx context.Context) (string, string, error) {
	credentials, err := provider.credentials.Retrieve(ctx)
	if err != nil {
		return "", "", err
	}

	requestURL := &url.URL{
		Scheme: "http",
		Host:   provider.cacheName,
		Path:   "/",
	}
	query := requestURL.Query()
	query.Set("Action", "connect")
	query.Set("User", provider.username)
	query.Set("X-Amz-Expires", "900")
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return "", "", err
	}

	signedURL, _, err := provider.signer.PresignHTTP(
		ctx,
		credentials,
		request,
		emptyBodySHA256,
		"elasticache",
		provider.region,
		time.Now(),
	)
	if err != nil {
		return "", "", err
	}

	return provider.username, strings.TrimPrefix(signedURL, "http://"), nil
}
