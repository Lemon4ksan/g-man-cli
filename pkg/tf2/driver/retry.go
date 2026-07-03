// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"context"
	"errors"
	"time"

	"github.com/lemon4ksan/g-man/pkg/steam/service"
)

const (
	maxRetries     = 3
	retryBaseDelay = 1 * time.Second
)

// retryOnRetriable calls fn and retries on retriable Steam API errors (5xx, 429).
func retryOnRetriable(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			var steamErr *service.SteamAPIError
			if errors.As(err, &steamErr) && steamErr.IsRetriable() {
				if attempt < maxRetries {
					delay := retryBaseDelay * time.Duration(1<<attempt)

					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
						continue
					}
				}
			}

			return lastErr
		}

		return nil
	}

	return lastErr
}
