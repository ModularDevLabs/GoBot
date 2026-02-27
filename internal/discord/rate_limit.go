package discord

import (
	"context"
	"time"
)

type RateLimiter struct {
	ch chan struct{}
}

func NewRateLimiter(ratePerSecond int) *RateLimiter {
	if ratePerSecond <= 0 {
		ratePerSecond = 1
	}
	ch := make(chan struct{}, ratePerSecond)
	for i := 0; i < ratePerSecond; i++ {
		ch <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for i := 0; i < ratePerSecond; i++ {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return &RateLimiter{ch: ch}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.ch:
		return nil
	}
}
