package internal

/*
#include <unistd.h>
// Simulated blocking C-function
void slow_call() {
    sleep(10); // Blocks the thread for 10 seconds
}
*/
import "C"
import "context"

func DoSlowCall(context.Context) error {
	C.slow_call()
	return nil
}
