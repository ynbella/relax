package client

import (
	"context"
	"github.com/patrickmn/go-cache"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

type Type func(c *Client)

type Option func(c *Client)

type Client struct {
	Cache       *cache.Cache
	Credentials *clientcredentials.Config
	Http        *http.Client
	Limiter     *rate.Limiter
	Timeout     time.Duration
}

// region Creation

func FromClient(http *http.Client) Type {
	return func(c *Client) {
		c.Http = http
	}
}

func FromDefaultClient() Type {
	return FromClient(http.DefaultClient)
}

func FromConfig(cred *clientcredentials.Config) Type {
	return func(c *Client) {
		c.Credentials = cred
		c.Http = cred.Client(context.Background())
	}
}

func FromCredentials(apiKey, apiKeySecret, tokenUrl string) Type {
	return FromConfig(&clientcredentials.Config{
		ClientID:     apiKey,
		ClientSecret: apiKeySecret,
		TokenURL:     tokenUrl,
	})
}

// endregion

// region Options

func WithTimeout(duration time.Duration) Option {
	return func(c *Client) {
		c.Timeout = duration
		c.Http.Timeout = duration
	}
}

func WithDefaultTimeout(duration time.Duration) Option {
	return WithTimeout(5 * time.Second)
}

func WithCache(defaultExpiration, cleanupInterval time.Duration) Option {
	return func(c *Client) {
		c.Cache = cache.New(defaultExpiration, cleanupInterval)
	}
}

func WithDefaultCache() Option {
	return WithCache(5*time.Minute, 10*time.Minute) // TODO Adjust default cache
}

func WithLimiter(limit float64, burst int) Option {
	return func(c *Client) {
		c.Limiter = rate.NewLimiter(rate.Limit(limit), burst)
	}
}

func WithDefaultLimiter() Option {
	return WithLimiter(10, 10) // TODO Adjust default limits
}

// endregion

func New(typ Type, opts ...Option) *Client {
	client := &Client{}
	typ(client)
	for _, opt := range opts {
		opt(client)
	}
	return client
}
