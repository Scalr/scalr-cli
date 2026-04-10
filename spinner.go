package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// spinner displays a simple progress indicator on stderr while a long operation runs.
// Returns a stop function that should be called when the operation completes.
func startSpinner(message string) func() {
	// Only show spinner when stderr is a terminal
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}

	done := make(chan struct{})
	frames := []rune{'|', '/', '-', '\\'}

	go func() {
		i := 0
		for {
			select {
			case <-done:
				// Clear the spinner line
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			default:
				if message != "" {
					fmt.Fprintf(os.Stderr, "\r%c %s", frames[i%len(frames)], message)
				} else {
					fmt.Fprintf(os.Stderr, "\r%c", frames[i%len(frames)])
				}
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	return func() {
		close(done)
		// Small delay to let the goroutine clean up
		time.Sleep(50 * time.Millisecond)
	}
}

// spinnerMessage updates the message shown next to the spinner.
// This is a helper for pagination progress.
func paginationSpinner(page int) func() {
	return startSpinner(fmt.Sprintf("Fetching page %d...", page))
}
