// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"time"

	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/miyako/log"
)

const (
	maxRetries     = 3
	retryBaseDelay = 1 * time.Second
)

// retryOnRetriable invokes fn and retries with exponential backoff on transient Steam API errors (HTTP 5xx, rate limits).
//
// Preconditions:
//   - ctx must carry cancellation timeouts to abort retry loops promptly.
func retryOnRetriable(ctx context.Context, logger log.Logger, operation string, fn func() error) error {
	var lastErr error

	for attempt := range maxRetries + 1 {
		if err := fn(); err != nil {
			lastErr = err

			if !isRetriableError(err) || attempt >= maxRetries {
				return lastErr
			}

			delay := retryBaseDelay * time.Duration(1<<attempt)
			logger.Warn("Retriable Steam API error encountered, scheduling retry",
				log.String("operation", operation),
				log.Int("attempt", attempt+1),
				log.Int("max_retries", maxRetries),
				log.Duration("delay", delay),
				log.Err(err),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		return nil
	}

	return lastErr
}

func isRetriableError(err error) bool {
	var steamErr *service.SteamAPIError
	return errors.As(err, &steamErr) && steamErr.IsRetriable()
}
