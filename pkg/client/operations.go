package client

import (
	"context"
	"errors"
	"net/http"
)

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
