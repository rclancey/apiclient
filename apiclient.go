package apiclient

import (
	//"bufio"
	//"crypto/sha1"
	//"encoding/hex"
	"encoding/json"
	//"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	//"net/http/httputil"
	"net/url"
	//"os"
	"path/filepath"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/pkg/errors"
	"github.com/rclancey/cache"
	"github.com/rclancey/cache/fs"
)

type APIClientOptions struct {
	BaseURL string
	RequestTimeout time.Duration
	CacheStore cache.CacheStore
	MaxCacheTime time.Duration
	MaxRequestsPerSecond float64
	Auth Authenticator
}

type HTTPClient struct {
	client *http.Client
	lastRequest time.Time
	rateLimit float64
}

func NewHTTPClient(opts APIClientOptions) (*HTTPClient, error) {
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 5 * time.Second
	}
	if opts.MaxRequestsPerSecond == 0 {
		opts.MaxRequestsPerSecond = 10.0
	}
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't open cookie jar")
	}
	cli := &http.Client{Jar: jar, Timeout: opts.RequestTimeout}
	return &HTTPClient{
		client: cli,
		lastRequest: time.Time{},
		rateLimit: opts.MaxRequestsPerSecond,
	}, nil
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.rateLimit > 0 {
		t := c.lastRequest.Add(time.Duration(float64(time.Second) / c.rateLimit))
		delay := t.Sub(time.Now())
		if delay > 0 {
			log.Printf("ratelimiting %s", delay)
			time.Sleep(delay)
		}
	}
	log.Println(req.Method, req.URL)
	return c.client.Do(req)
}

func (c *HTTPClient) Get(u string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

type APIClient struct {
	BaseURL *url.URL
	CacheDirectory string
	MaxCacheTime time.Duration
	MaxRequestsPerSecond float64
	Authenticator Authenticator
	cache *cache.Cache
	client *HTTPClient
	lastFetch time.Time
}

func NewAPIClient(opts APIClientOptions) (*APIClient, error) {
	if opts.CacheStore == nil {
		pth, err := filepath.Abs(filepath.Join(".", "var", "cache"))
		if err != nil {
			return nil, err
		}
		opts.CacheStore = fscache.NewFSCacheStore(pth)
	}
	if opts.MaxCacheTime == 0 {
		opts.MaxCacheTime = 24 * time.Hour
	}
	u, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "can't parse base url " + opts.BaseURL)
	}
	cli, err := NewHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	cacher := cache.NewCache(opts.CacheStore, cli)
	c := &APIClient{
		BaseURL: u,
		MaxCacheTime: opts.MaxCacheTime,
		Authenticator: opts.Auth,
		cache: cacher,
		client: cli,
		lastFetch: time.Unix(0, 0),
	}
	return c, nil
}

func (c *APIClient) Client() *HTTPClient {
	return c.client
}

func (c *APIClient) Do(req *http.Request) (*http.Response, error) {
	res, err := c.cache.CacheRequest(req, c.MaxCacheTime)
	if res != nil && err == nil {
		return res, nil
	}
	/*
	res, err = c.RateLimit(req)
	if err != nil {
		return res, errors.Wrap(err, "can't execute api request")
	}
	err = c.saveToCache(res)
	*/
	return res, errors.Wrap(err, "can't cache api response")
}

func (c *APIClient) Get(rsrc string, args url.Values) (*http.Response, error) {
	u, err := c.BaseURL.Parse(rsrc)
	if err != nil {
		return nil, errors.Wrap(err, "can't parse api request uri " + rsrc)
	}
	if args != nil {
		u.RawQuery = args.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't create api get request")
	}
	if c.Authenticator != nil {
		err = c.Authenticator.AuthenticateRequest(req)
		if err != nil {
			return nil, errors.Wrap(err, "can't auth api get request")
		}
	}
	return c.Do(req)
}

func (c *APIClient) GetObj(rsrc string, args url.Values, obj interface{}) error {
	res, err := c.Get(rsrc, args)
	if err != nil {
		return errors.Wrap(err, "can't execute api get request")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}
	ct := res.Header.Get("Content-Type")
	if ct != "application/json" {
		return errors.Errorf("not a json response (%s)", ct)
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "can't read api response")
	}
	err = json.Unmarshal(data, obj)
	if err != nil {
		return errors.Wrapf(err, "can't unmarshal api response into %T", obj)
	}
	return nil
}
