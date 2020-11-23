package relax

import (
	"context"
	"errors"
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

// region Operations
type Modifiers struct {
	UseCache   bool
	UseLimiter bool
}

type Modifier func(m *Modifiers)

// region Modifiers

func UseCache(use bool) Modifier {
	return func(m *Modifiers) {
		m.UseCache = use
	}
}

func UseLimiter(use bool) Modifier {
	return func(m *Modifiers) {
		m.UseLimiter = use
	}
}

// endregion

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
	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

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