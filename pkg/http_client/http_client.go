package http_client

import (
	"net"
	"net/http"
	"time"
)

func CreateHTTPClient() *http.Client {
	tr := &http.Transport{
		MaxIdleConns:          20,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       30 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	cli := &http.Client{
		Timeout:   2 * time.Second,
		Transport: tr,
	}

	return cli
}
