package valkey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const tokenLifetimeSeconds = "900"

var emptyPayloadHash = func() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}()

type iamAuthenticator struct {
	credentials aws.CredentialsProvider
	signer      *v4.Signer
	region      string
	cacheName   string
	username    string
}

func (authenticator iamAuthenticator) credentialsFor(ctx context.Context) (string, string, error) {
	credentials, err := authenticator.credentials.Retrieve(ctx)
	if err != nil {
		return "", "", err
	}

	query := url.Values{
		"Action":        {"connect"},
		"User":          {authenticator.username},
		"X-Amz-Expires": {tokenLifetimeSeconds},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+authenticator.cacheName+"/?"+query.Encode(), nil)
	if err != nil {
		return "", "", err
	}

	signedURL, _, err := authenticator.signer.PresignHTTP(
		ctx, credentials, request, emptyPayloadHash, "elasticache", authenticator.region, time.Now(),
	)
	if err != nil {
		return "", "", err
	}
	return authenticator.username, strings.TrimPrefix(signedURL, "http://"), nil
}
