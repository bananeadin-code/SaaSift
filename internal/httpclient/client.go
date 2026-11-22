package httpclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Options struct {
	Threads  int
	Timeout  time.Duration
	Proxy    string
	Insecure bool
}

func New(opts Options) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = opts.Threads
	transport.MaxIdleConnsPerHost = opts.Threads

	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)

		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			return nil, fmt.Errorf("invalid proxy %q (expected e.g. http://127.0.0.1:8000)", opts.Proxy)
		}

		transport.Proxy = http.ProxyURL(proxyURL)
	}

	if opts.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}
