package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// spinner displays a simple progress indicator on stderr while a long operation runs.
// Returns a stop function that should be called when the operation completes.
// The stop function blocks until the spinner goroutine has fully cleaned up.
func startSpinner(message string) func() {
	// Only show spinner when stderr is a terminal
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	frames := []rune{'|', '/', '-', '\\'}

	go func() {
		defer wg.Done()
		i := 0
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				// Clear the spinner line completely
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				if message != "" {
					fmt.Fprintf(os.Stderr, "\r%c %s", frames[i%len(frames)], message)
				} else {
					fmt.Fprintf(os.Stderr, "\r%c", frames[i%len(frames)])
				}
				i++
			}
		}
	}()

	return func() {
		close(done)
		wg.Wait() // Block until the goroutine has cleared the line
	}
}

// paginationSpinner creates a spinner with a page-number message.
func paginationSpinner(page int) func() {
	return startSpinner(fmt.Sprintf("Fetching page %d...", page))
}
