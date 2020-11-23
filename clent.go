// Package relax provides HTTP client and server implementations that provide
// rate limitation and caching.
package relax

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

// region Creation

// Client represents an HTTP client with additional possible features
type Client struct {
	Cache       *cache.Cache
	Credentials *clientcredentials.Config
	HTTP        *http.Client
	Limiter     *rate.Limiter
	Timeout     time.Duration
}

// region Options

// ClientOption specifies how to create a client
type ClientOption func(c *Client)

// FromClient creates a client from a specified HTTP client
func FromClient(http *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTP = http
	}
}

// FromDefaultClient creates a client from the default HTTP implementation
func FromDefaultClient() ClientOption {
	return FromClient(http.DefaultClient)
}

// FromConfig creates a client from a specified OAuth client configuration
func FromConfig(cred *clientcredentials.Config) ClientOption {
	return func(c *Client) {
		c.Credentials = cred
		c.HTTP = cred.Client(context.Background())
	}
}

// FromCredentials creates a client from OAuth credentials
func FromCredentials(apiKey, apiKeySecret, tokenURL string) ClientOption {
	return FromConfig(&clientcredentials.Config{
		ClientID:     apiKey,
		ClientSecret: apiKeySecret,
		TokenURL:     tokenURL,
	})
}

// endregion

// region Features

// ClientFeature is a functional option for a client to specify additional optional
// features on top of the default implementation
type ClientFeature func(c *Client)

// WithTimeout allows the client to timeout after a specified duration
func WithTimeout(duration time.Duration) ClientFeature {
	return func(c *Client) {
		c.Timeout = duration
		c.HTTP.Timeout = duration
	}
}

// WithDefaultTimeout allows the client to timeout after a default duration of
// 5 seconds
func WithDefaultTimeout(duration time.Duration) ClientFeature {
	return WithTimeout(5 * time.Second)
}

// WithCache allows the client to utilize a cache with a specified expiration
// time and cleanup interval
func WithCache(defaultExpiration, cleanupInterval time.Duration) ClientFeature {
	return func(c *Client) {
		c.Cache = cache.New(defaultExpiration, cleanupInterval)
	}
}

// WithDefaultCache allows the client to utilize a cache with default values
func WithDefaultCache() ClientFeature {
	return WithCache(5*time.Minute, 10*time.Minute) // TODO Adjust default cache
}

// WithLimiter allows the client to use a rate limiter for requests
func WithLimiter(limit float64, burst int) ClientFeature {
	return func(c *Client) {
		c.Limiter = rate.NewLimiter(rate.Limit(limit), burst)
	}
}

// WithDefaultLimiter allows the client to use a rate limiter for requests
// using default values
func WithDefaultLimiter() ClientFeature {
	return WithLimiter(10, 10) // TODO Adjust default limits
}

// endregion

// New creates a new client with a specified option along with the optional
// features implemented.
func New(option ClientOption, feats ...ClientFeature) *Client {
	client := &Client{}
	option(client)
	for _, feat := range feats {
		feat(client)
	}
	return client
}

// endregion

// region Operations

// region Modifiers

// Modifier is a functional option for a request
type Modifier func(m *Modifiers)

// Modifiers represents the potential modifiers to use on a request
type Modifiers struct {
	UseCache   bool
	UseLimiter bool
}

// UseCache forces the client to attempt to pull from a cache, if the request
// was already made, or otherwise update the cache.
func UseCache(use bool) Modifier {
	return func(m *Modifiers) {
		m.UseCache = use
	}
}

// UseLimiter forces the client to obey rate limiting rules when performing the
// request or not.
func UseLimiter(use bool) Modifier {
	return func(m *Modifiers) {
		m.UseLimiter = use
	}
}

// endregion

// Do sends an HTTP request and returns an HTTP response, with the option of
// using modifiers to cache or limit the response
func (c *Client) Do(req *http.Request, mods ...Modifier) (*http.Response, error) {
	modifiers := &Modifiers{}
	for _, mod := range mods {
		mod(modifiers)
	}
	if modifiers.UseLimiter {
		if c.Limiter == nil {
			err := errors.New("relax: limiter not defined")
			return nil, err
		}
		err := c.Limiter.Wait(context.Background())
		if err != nil {
			return nil, err
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Get issues a GET to the specified URL, with the option of using modifiers to
// cache or limit the response
func (c *Client) Get(url string, mods ...Modifier) (resp *http.Response, err error) {
	modifiers := &Modifiers{}
	for _, mod := range mods {
		mod(modifiers)
	}
	if modifiers.UseCache {
		if c.Cache == nil {
			err := errors.New("relax: cache not defined")
			return nil, err
		}
		cached, found := c.Cache.Get(url)
		if found {
			return cached.(*http.Response), nil
		}
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err = c.Do(req)
	if err == nil {
		return nil, err
	}
	if modifiers.UseCache {
		c.Cache.SetDefault(url, resp)
	}
	return resp, err
}

// endregion
