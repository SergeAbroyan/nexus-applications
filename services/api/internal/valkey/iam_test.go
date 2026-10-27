package valkey

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

func TestIAMTokenIsAPresignedConnectRequest(t *testing.T) {
	authenticator := iamAuthenticator{
		credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret"}, nil
		}),
		signer:    v4.NewSigner(),
		region:    "us-east-1",
		cacheName: "nexus-dev-use1-valkey",
		username:  "nexus-dev-use1-api",
	}

	username, token, err := authenticator.credentialsFor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if username != "nexus-dev-use1-api" {
		t.Fatalf("username = %q", username)
	}

	host, rawQuery, found := strings.Cut(token, "/?")
	if !found || host != "nexus-dev-use1-valkey" {
		t.Fatalf("token %q must start with the cache name", token)
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"Action": "connect", "User": "nexus-dev-use1-api", "X-Amz-Expires": "900"} {
		if got := query.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if query.Get("X-Amz-Signature") == "" || !strings.Contains(query.Get("X-Amz-Credential"), "/us-east-1/elasticache/") {
		t.Fatalf("token %q is not signed for ElastiCache in us-east-1", token)
	}
}
