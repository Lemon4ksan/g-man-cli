// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/profiles/chrome"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/miyako/generic"
)

// setupProxyConfig configures connection manager socket proxy and web proxies rotator on clientCfg.
func setupProxyConfig(cfg Config, clientCfg *steam.Config, logger log.Logger) error {
	if cfg.ProxyCM != "" {
		clientCfg.Socket.Connector.ProxyURL = cfg.ProxyCM
		clientCfg.Socket.Connector.ConnectTimeout = 30 * time.Second

		logger.Info("Configured Connection Manager socket proxy", log.String("proxy", cfg.ProxyCM))
	}

	var (
		middleware []aoni.Middleware
		baseDoer   aoni.HTTPDoer = &http.Client{Timeout: 10 * time.Second}
	)

	if cfg.RestLogs {
		middleware = append(middleware, log.LoggingMiddleware(logger))
	}

	if len(cfg.ProxiesWeb) > 0 {
		var rotatableClients []aoni.ClientWithProxy
		for _, proxyURL := range cfg.ProxiesWeb {
			parsedURL, err := url.Parse(proxyURL)
			if err != nil {
				logger.Error("Skipping invalid proxy URL", log.String("url", proxyURL), log.Err(err))
				continue
			}

			transport := &http.Transport{
				Proxy:                 http.ProxyURL(parsedURL),
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
			}

			httpClient := &http.Client{
				Transport: transport,
				Timeout:   15 * time.Second,
			}

			rotatableClients = append(rotatableClients, aoni.ClientWithProxy{
				Client:   httpClient,
				ProxyURL: proxyURL,
			})
		}

		if len(rotatableClients) > 0 {
			rotatorConfig := aoni.ProxyRotatorConfig{
				MaxFails:            3,
				RetryAfter:          45 * time.Second,
				HealthCheckURL:      "https://api.steampowered.com/ISteamDirectory/GetCMList/v1",
				HealthCheckInterval: 2 * time.Minute,
			}

			rotator, err := aoni.NewProxyRotator(rotatorConfig, rotatableClients...)
			if err != nil {
				return fmt.Errorf("failed to initialize proxy rotator: %w", err)
			}

			stickyRotator := rotator.WithStickySessions(aoni.StickyKeyFromCookie("sessionid"))

			retryMiddleware := aoni.RetryMiddleware(aoni.RetryOptions{
				MaxRetries: 3,
				Backoff:    500 * time.Millisecond,
			}, aoni.ProxyRetryCondition(rotator))

			middleware = append(middleware, retryMiddleware)

			baseDoer = stickyRotator

			logger.Info("Configured HTTP WebAPI proxy rotator", log.Int("proxies_count", len(rotatableClients)))
		}
	}

	chainedDoer := aoni.Chain(baseDoer, middleware...)

	if cfg.CircuitBreakerEnabled {
		cb := aoni.NewCircuitBreaker(aoni.CircuitBreakerConfig{
			FailureThreshold: 5,
			Cooldown:         30 * time.Second,
		})

		chainedDoer = aoni.Chain(chainedDoer, aoni.CircuitBreakerMiddleware(cb, nil))

		logger.Info("Configured circuit breaker middleware")
	}

	clientCfg.HTTP = chainedDoer

	restClient := aoni.NewClient(chainedDoer).
		WithTLSFingerprint(aoni.BrowserChrome).
		WithUserAgent(chrome.UserAgentWindows).
		WithMaxResponseSize(100 * 1024 * 1024).
		WithConnectionPool(aoni.ConnectionPoolConfig{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		})

	if cfg.DoTEndpoint != "" {
		dotHost := generic.Coalesce(cfg.DoTHost, "cloudflare-dns.com")

		restClient = restClient.WithDoT(cfg.DoTEndpoint, dotHost).
			WithDNSCache(10 * time.Minute)

		logger.Info("Configured DNS-over-TLS resolver",
			log.String("endpoint", cfg.DoTEndpoint),
			log.String("host", dotHost),
		)
	}

	clientCfg.REST = restClient

	return nil
}
