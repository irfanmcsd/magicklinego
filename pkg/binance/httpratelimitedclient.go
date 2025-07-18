package binance

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
	for {
		if err := c.applyRateLimiting(req.URL.Path, weight); err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			c.updateRateLimits(resp.Header)
			if resp.StatusCode == 429 || resp.StatusCode == 418 {
				delay := c.getRetryDelay(resp, retryCount)
				if retryCount >= c.maxRetries {
					return nil, fmt.Errorf("max retries reached")
				}
				time.Sleep(delay)
				retryCount++
				continue
			}
			return resp, nil
		}

		if isTransient(err) {
			delay := c.getRetryDelay(nil, retryCount)
			if retryCount >= c.maxRetries {
				return nil, err
			}
			time.Sleep(delay)
			retryCount++
			continue
		}
		return nil, err
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
		delay := info.getDelay(weight)
		time.Sleep(delay)
	}
	return nil
}

func (c *RateLimitedClient) updateRateLimits(headers http.Header) {
	headerMapping := map[string]struct {
		key          string
		window       time.Duration
		defaultLimit int
	}{
		"X-MBX-USED-WEIGHT-1M": {"weight", 60 * time.Second, 1200},
		"X-MBX-ORDER-COUNT-1M": {"orders", 60 * time.Second, 1600},
		"X-MBX-ORDER-COUNT-1D": {"daily_orders", 24 * time.Hour, 10000},
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	for header, meta := range headerMapping {
		if values, ok := headers[header]; ok {
			if used, err := strconv.Atoi(values[0]); err == nil {
				reset := time.Now().Add(meta.window)
				c.rateLimits[meta.key] = &RateLimitInfo{
					used:      used,
					limit:     meta.defaultLimit,
					window:    meta.window,
					resetTime: reset,
				}
			}
		}
	}
}

func (info *RateLimitInfo) shouldDelay(weight int) bool {
	remaining := info.limit - info.used - weight
	buffer := int(float64(info.limit) * 0.1)
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
	base := math.Pow(2, float64(retry))
	jitter := rand.Float64()*0.5 + 0.75
	return time.Duration(base*jitter) * time.Second
}

func isTransient(err error) bool {
	return errors.Is(err, http.ErrHandlerTimeout) ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection reset")
}
