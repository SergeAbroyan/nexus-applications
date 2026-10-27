package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

const DefaultPort = "8080"

type Config struct {
	Address     string
	DatabaseURL string
	QueueURL    string
	Valkey      Valkey
}

type Valkey struct {
	Address           string
	Username          string
	CacheName         string
	IAMAuthentication bool
	TLS               bool
	CacheTTL          time.Duration
}

func Load() (Config, error) {
	var env environment

	databaseURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(env.optional("DB_USER", "nexus"), env.required("DB_PASSWORD")),
		Host:     net.JoinHostPort(env.optional("DB_HOST", "localhost"), env.optional("DB_PORT", "5432")),
		Path:     env.optional("DB_NAME", "nexus"),
		RawQuery: url.Values{"sslmode": {env.optional("DB_SSLMODE", "prefer")}}.Encode(),
	}

	cfg := Config{
		Address:     net.JoinHostPort("", env.optional("PORT", DefaultPort)),
		DatabaseURL: databaseURL.String(),
		QueueURL:    env.required("SQS_QUEUE_URL"),
		Valkey: Valkey{
			Address:           net.JoinHostPort(env.optional("VALKEY_HOST", "localhost"), env.optional("VALKEY_PORT", "6379")),
			Username:          env.optional("VALKEY_USERNAME", ""),
			CacheName:         env.optional("VALKEY_CACHE_NAME", ""),
			IAMAuthentication: env.boolean("VALKEY_IAM_AUTH"),
			TLS:               env.boolean("VALKEY_TLS"),
			CacheTTL:          env.duration("VALKEY_CACHE_TTL", 30*time.Second),
		},
	}

	if cfg.Valkey.IAMAuthentication && (cfg.Valkey.Username == "" || cfg.Valkey.CacheName == "") {
		env.fail(errors.New("VALKEY_USERNAME and VALKEY_CACHE_NAME are required when VALKEY_IAM_AUTH is true"))
	}

	return cfg, errors.Join(env.problems...)
}

type environment struct {
	problems []error
}

func (env *environment) fail(err error) {
	env.problems = append(env.problems, err)
}

func (env *environment) optional(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (env *environment) required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		env.fail(fmt.Errorf("%s is required", name))
	}
	return value
}

func (env *environment) boolean(name string) bool {
	value := os.Getenv(name)
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		env.fail(fmt.Errorf("%s must be true or false", name))
	}
	return parsed
}

func (env *environment) duration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		env.fail(fmt.Errorf("%s must be a positive duration such as 30s", name))
	}
	return parsed
}
