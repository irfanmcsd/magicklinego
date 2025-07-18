package bybit

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimitedClient struct {
	httpClient  *http.Client
	lock        sync.Mutex
	rateLimits  map[string]*RateLimitInfo
	maxRetries  int
	defaultWait time.Duration
}

type RateLimitInfo struct {
	used      int
	limit     int
	window    time.Duration
	resetTime time.Time
}

func NewRateLimitedClient(client *http.Client) *RateLimitedClient {
	if client == nil {
		client = &http.Client{}
	}
	return &RateLimitedClient{
		httpClient:  client,
		rateLimits:  make(map[string]*RateLimitInfo),
		maxRetries:  5,
		defaultWait: 5 * time.Second,
	}
}

func (c *RateLimitedClient) SendWithRetry(req *http.Request, weight int) (*http.Response, error) {
	retryCount := 0
	key := req.URL.Path

	for {
		if err := c.applyRateLimiting(key, weight); err != nil {
			return nil, fmt.Errorf("rate limit delay failed: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			c.updateRateLimits(key, resp.Header)

			if resp.StatusCode == 429 || resp.StatusCode == 418 {
				if retryCount >= c.maxRetries {
					return nil, fmt.Errorf("max retries reached: %s", resp.Status)
				}
				delay := c.getRetryDelay(resp, retryCount)
				time.Sleep(delay)
				retryCount++
				continue
			}
			return resp, nil
		}

		if isTransient(err) {
			if retryCount >= c.maxRetries {
				return nil, fmt.Errorf("transient error after max retries: %w", err)
			}
			time.Sleep(c.getRetryDelay(nil, retryCount))
			retryCount++
			continue
		}

		return nil, fmt.Errorf("request failed: %w", err)
	}
}

func (c *RateLimitedClient) applyRateLimiting(key string, weight int) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	info, exists := c.rateLimits[key]
	if !exists {
		return nil
	}

	if info.shouldDelay(weight) {
		time.Sleep(info.getDelay(weight))
	}
	return nil
}

func (c *RateLimitedClient) updateRateLimits(key string, headers http.Header) {
	c.lock.Lock()
	defer c.lock.Unlock()

	limitStr := headers.Get("X-RateLimit-Limit")
	remainingStr := headers.Get("X-RateLimit-Remaining")
	resetStr := headers.Get("X-RateLimit-Reset")

	if limitStr == "" || remainingStr == "" || resetStr == "" {
		return
	}

	limit, err1 := strconv.Atoi(limitStr)
	remaining, err2 := strconv.Atoi(remainingStr)
	resetUnix, err3 := strconv.ParseInt(resetStr, 10, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return
	}

	used := limit - remaining
	reset := time.Unix(resetUnix, 0)

	c.rateLimits[key] = &RateLimitInfo{
		used:      used,
		limit:     limit,
		window:    time.Until(reset),
		resetTime: reset,
	}
}

func (info *RateLimitInfo) shouldDelay(weight int) bool {
	remaining := info.limit - info.used - weight
	buffer := int(float64(info.limit) * 0.1) // 10% buffer
	return remaining <= buffer
}

func (info *RateLimitInfo) getDelay(weight int) time.Duration {
	remaining := info.limit - info.used - weight
	if remaining >= 0 {
		return 0
	}
	elapsed := time.Until(info.resetTime)
	perSecond := float64(info.limit) / info.window.Seconds()
	estimate := float64(-remaining) / perSecond
	return time.Duration(math.Max(estimate, elapsed.Seconds())) * time.Second
}

func (c *RateLimitedClient) getRetryDelay(resp *http.Response, retry int) time.Duration {
	if resp != nil {
		if val := resp.Header.Get("Retry-After"); val != "" {
			if seconds, err := strconv.Atoi(val); err == nil {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	backoff := math.Pow(2, float64(retry))
	jitter := rand.Float64()*0.5 + 0.75 // range: [0.75, 1.25)
	return time.Duration(backoff*jitter) * time.Second
}

func isTransient(err error) bool {
	return errors.Is(err, http.ErrHandlerTimeout) ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "temporary")
}
