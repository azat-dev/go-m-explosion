package main

/*
#include <unistd.h>
// Simulated blocking C-function
void slow_call() {
    sleep(10); // Blocks the thread for 10 seconds
}
*/
import "C"
import (
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Start Prometheus metrics server in a separate goroutine
	// Access metrics at http://localhost:8080/metrics
	go func() {
		http.Handle(
			"/metrics",
			promhttp.Handler(),
		)
		http.ListenAndServe(
			":8080",
			nil,
		)
	}()

	// STEP 1: Strictly limit logical processors (P) to 2
	// This should theoretically limit the number of threads, but Go's sysmon has other plans.
	runtime.GOMAXPROCS(2)

	// Wait 30 seconds to establish a "baseline" in Grafana (idle state)
	time.Sleep(30 * time.Second)

	var wg sync.WaitGroup

	// STEP 2: Launch 100 "greedy" goroutines
	// Each will invoke a blocking C-call, forcing the scheduler to spawn new system threads (M).
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			C.slow_call()
		}()
	}

	// Wait for all blocking calls to complete
	wg.Wait()

	// Keep the process alive for another 45 seconds
	// This allows Prometheus to scrape the "post-explosion" state and cooldown.
	time.Sleep(45 * time.Second)
}
