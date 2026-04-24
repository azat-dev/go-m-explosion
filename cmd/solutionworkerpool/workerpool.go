package main

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrWorkerPoolTerminated = errors.New("worker pool is terminated")
	ErrWorkerMustBeStarted  = errors.New("worker must be started")
)

// Invariants:
// - Workers must be started before doing any task
// - Worker pool must be closed at the end of work with it
// - We can't execute a new task on stopped worker pool
// - Even if worker pool is started twice than create only needed workers
// - If worker pool is closed than user must create a new one

type workerPoolState interface {
	Start() error
	Close()
	Do(ctx context.Context, action func(context.Context) error) error
}

var _ workerPoolState = (*workerPoolIdle)(nil)

// IDLE
type workerPoolIdle struct {
	numberOfWorkers int
	moveToNextState func(st workerPoolState)
}

func newWorkerPoolIdle(numberOfWorkers int, moveToNextState func(st workerPoolState)) *workerPoolIdle {
	return &workerPoolIdle{numberOfWorkers: numberOfWorkers, moveToNextState: moveToNextState}
}

func (w *workerPoolIdle) Start() error {
	isStoppedMu := sync.RWMutex{}

	tasks := make(chan func(), 2*w.numberOfWorkers)
	var workersGroup sync.WaitGroup

	doWork := func() {
		// release wait group on exit
		defer workersGroup.Done()

		// Take one task from the channel
		// or
		// if there are no any task
		// than park the current go routine
		// If the channel is closed than continue until all tasks won't be finished
		for task := range tasks {
			// Execute task
			task()
		}
	}

	i := 0

	// Start all workers
	for i < w.numberOfWorkers {
		// Add a new worker into waiting group
		workersGroup.Add(1)
		// Start worker task in a new go routine
		go doWork()
		i++
	}

	// some workers maybe in the queue
	// and haven't been started yet
	// but that's ok
	w.moveToNextState(
		newWorkerPoolStarted(
			&workersGroup,
			tasks,
			&isStoppedMu,
			new(false),
			w.moveToNextState,
		),
	)

	return nil
}

func (w *workerPoolIdle) Close() {
	// Do nothing
}

func (w *workerPoolIdle) Do(ctx context.Context, action func(context.Context) error) error {
	return ErrWorkerMustBeStarted
}

// Started

var _ workerPoolState = (*workerPoolStarted)(nil)

// Started
type workerPoolStarted struct {
	wg              *sync.WaitGroup
	tasks           chan func()
	moveToNextState func(st workerPoolState)
	isStoppedMutex  *sync.RWMutex
	isStopped       *bool
}

func newWorkerPoolStarted(
	wg *sync.WaitGroup,
	tasks chan func(),
	isStoppedMutex *sync.RWMutex,
	isStopped *bool,
	moveToNextState func(st workerPoolState),
) *workerPoolStarted {
	return &workerPoolStarted{
		wg:              wg,
		tasks:           tasks,
		moveToNextState: moveToNextState,
		isStoppedMutex:  isStoppedMutex,
		isStopped:       isStopped,
	}
}

func (w *workerPoolStarted) Start() error {
	// Do nothing
	return nil
}

func (w *workerPoolStarted) Close() {

	defer w.isStoppedMutex.Unlock()
	// If someone reads the state of write it than we wait
	w.isStoppedMutex.Lock()

	if *w.isStopped {
		// Do nothing
		return
	}

	*w.isStopped = true
	close(w.tasks)

	// Park the current go routine
	// till all workers won't stop
	w.wg.Wait()
	w.moveToNextState(newWorkerPoolStopped())
}

func (w *workerPoolStarted) Do(ctx context.Context, action func(context.Context) error) error {

	// Create a new buffered channel (one time) for response of the task
	res := make(chan error, 1)

	// Create a new task
	task := func() {
		select {
		// Check if context is stopped
		case <-ctx.Done():
			res <- ctx.Err()
		default:
			res <- action(ctx)
		}
	}

	w.isStoppedMutex.RLock()
	if *w.isStopped {
		w.isStoppedMutex.RUnlock()
		return ErrWorkerPoolTerminated
	}

	// tasks channel guarded by isStoppedMutex
	// Step1: send a new task
	w.tasks <- task
	w.isStoppedMutex.RUnlock()

	// Step2: wait for response
	err := <-res
	return err
}

// Stopped

var _ workerPoolState = (*workerPoolStopped)(nil)

type workerPoolStopped struct {
}

func newWorkerPoolStopped() *workerPoolStopped {
	return &workerPoolStopped{}
}

func (w *workerPoolStopped) Start() error {
	return ErrWorkerPoolTerminated
}

func (w *workerPoolStopped) Close() {
	// Do nothing
}

func (w *workerPoolStopped) Do(ctx context.Context, action func(context.Context) error) error {
	return ErrWorkerPoolTerminated
}

// Public structure

type WorkerPool struct {
	state workerPoolState
}

func NewWorkerPool(limit int) *WorkerPool {

	mu := sync.Mutex{}
	wp := WorkerPool{}

	wp.state = newWorkerPoolIdle(limit, func(st workerPoolState) {
		defer mu.Unlock()
		mu.Lock()

		wp.state = st
	})
	return &wp
}

func (l *WorkerPool) Start() error {
	return l.state.Start()
}

func (l *WorkerPool) Close() {
	l.state.Close()
}

func (l *WorkerPool) Do(ctx context.Context, action func(context.Context) error) error {
	return l.state.Do(ctx, action)
}

func (l *WorkerPool) WrapFunc(action func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		return l.Do(ctx, action)
	}
}
