/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpclient

import "time"

// Config holds configuration for the HTTP client
type Config struct {
	// BaseURL is the base URL for all requests (e.g., "https://api.example.com")
	BaseURL string

	// Headers are common headers to include in all requests (e.g., Authorization, Content-Type)
	Headers map[string]string

	// Timeout is the maximum time for a single request (including retries)
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts
	// Default: 5
	MaxRetries int

	// RetryWaitMin is the minimum wait time between retries
	// go-retryablehttp uses exponential backoff starting from this value
	// Default: 1 second
	RetryWaitMin time.Duration

	// RetryWaitMax is the maximum wait time between retries
	// Exponential backoff will not exceed this value
	// Default: 30 seconds
	RetryWaitMax time.Duration

	// MaxIdleConns controls the maximum number of idle (keep-alive) connections
	// Default: 10
	MaxIdleConns int

	// IdleConnTimeout is the maximum amount of time an idle connection will remain idle
	// Default: 90 seconds
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout specifies the maximum amount of time waiting for a TLS handshake
	// Default: 10 seconds
	TLSHandshakeTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Timeout:             30 * time.Second,
		MaxRetries:          5,
		RetryWaitMin:        1 * time.Second,
		RetryWaitMax:        30 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		Headers:             make(map[string]string),
	}
}
