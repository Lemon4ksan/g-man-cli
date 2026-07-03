// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/fast"
	"github.com/lemon4ksan/aoni/fingerprint/profiles"
	"github.com/lemon4ksan/aoni/middleware"
	"github.com/lemon4ksan/aoni/netutil/dns"
	"github.com/lemon4ksan/aoni/netutil/proxy"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"
)

// setupProxy configures socket proxies for Steam Connection Manager (CM),
// uTLS Chrome browser TLS fingerprinting, DoT/DoH DNS resolvers, and sticky proxy rotation pools.
func setupProxy(cfg Config, clientCfg *steam.Config, logger log.Logger) (*fast.Client, error) {
	if cfg.ProxyCM != "" {
		clientCfg.Socket.Connector.ProxyURL = cfg.ProxyCM
		clientCfg.Socket.Connector.ConnectTimeout = 30 * time.Second

		logger.Info("Configured Connection Manager socket proxy", log.String("proxy", cfg.ProxyCM))
	}

	baseOpts := []aoni.ClientOption{
		option.WithBrowserProfile(aoni.BrowserChrome, profiles.Windows),
		option.WithTimeout(30 * time.Second),
		option.WithMaxResponseSize(10 * 1024 * 1024),
		option.WithConnectionPool(aoni.ConnectionPoolConfig{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		}),
	}

	if cfg.DoTEndpoint != "" {
		dotHost := generic.Coalesce(cfg.DoTHost, "cloudflare-dns.com")

		res := dns.NewFallbackResolver(
			dns.NewStdlibResolver(),
			dns.NewDoTResolver(cfg.DoTEndpoint, dotHost),
			dns.NewDoHResolver("https://1.1.1.1/dns-query", "cloudflare-dns.com", nil),
		)

		baseOpts = append(baseOpts, option.WithDNSResolver(dns.NewInMemoryDNSCache(5*time.Minute, res)))

		logger.Info("Configured DNS resolver for fast transport",
			log.String("endpoint", cfg.DoTEndpoint),
			log.String("host", dotHost),
		)
	} else {
		res := dns.NewFallbackResolver(
			dns.NewStdlibResolver(),
			dns.NewDoHResolver("https://1.1.1.1/dns-query", "cloudflare-dns.com", nil),
			dns.NewDoHResolver("https://8.8.8.8/dns-query", "dns.google", nil),
		)

		baseOpts = append(baseOpts, option.WithDNSResolver(dns.NewInMemoryDNSCache(5*time.Minute, res)))
	}

	webProxies := cfg.ProxiesWeb
	if len(webProxies) == 0 && cfg.ProxyCM != "" {
		webProxies = []string{cfg.ProxyCM}
	}

	if len(webProxies) == 1 {
		baseOpts = append(baseOpts, option.WithProxyString(webProxies[0]))
		logger.Info("Configured Web API proxy", log.String("proxy", webProxies[0]))
	} else if len(webProxies) > 1 {
		baseFast := fast.NewClient(baseOpts...)

		rotatableClients := make([]proxy.WithClient, 0, len(webProxies))
		for _, proxyURL := range webProxies {
			pClient := fast.NewStdClient(baseFast.With(option.WithProxyString(proxyURL)))

			rotatableClients = append(rotatableClients, proxy.WithClient{
				Client:   pClient,
				ProxyURL: proxyURL,
			})
		}

		rotatorConfig := proxy.RotatorConfig{
			MaxFails:            3,
			RetryAfter:          45 * time.Second,
			HealthCheckURL:      "https://api.steampowered.com/ISteamDirectory/GetCMList/v1",
			HealthCheckInterval: 2 * time.Minute,
			Logger:              logger,
		}

		rotator, err := proxy.NewRotator(rotatorConfig, rotatableClients...)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize web proxy rotator: %w", err)
		}

		stickyRotator := rotator.WithStickySessions(proxy.StickyKeyFromCookie("sessionid"))

		retryMiddleware := middleware.Retry(middleware.RetryOptions{
			MaxRetries: 3,
			Backoff:    500 * time.Millisecond,
		}, proxy.RetryCondition(rotator))

		chainedEngine := middleware.Chain(stickyRotator, retryMiddleware)
		httpEngine := aoni.NewRequestDoerAdapter(chainedEngine)

		baseOpts = append(baseOpts, option.WithEngine(httpEngine))

		logger.Info("Configured fast Web API proxy rotator pool", log.Int("count", len(webProxies)))
	}

	return fast.NewClient(baseOpts...), nil
}
