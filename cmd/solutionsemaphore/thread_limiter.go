package main

import (
	"context"
)

type ThreadLimiter struct {
	semaphore chan struct{}
}

func NewThreadLimiter(limit int) *ThreadLimiter {
	return &ThreadLimiter{
		semaphore: make(chan struct{}, limit),
	}
}

func (l *ThreadLimiter) Do(ctx context.Context, action func(context.Context) error) error {
	// Park current go routine
	select {
	case l.semaphore <- struct{}{}:
		defer func() { <-l.semaphore }()
		return action(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *ThreadLimiter) WrapFunc(action func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return l.Do(ctx, action)
	}
}
