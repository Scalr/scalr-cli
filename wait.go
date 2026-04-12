package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Jeffail/gabs/v2"
)

// Terminal states for Terraform runs — the run will not change without external action
var terminalStates = map[string]bool{
	"applied":              true,
	"errored":              true,
	"discarded":            true,
	"canceled":             true,
	"planned_and_finished": true,
	"force_canceled":       true,
}

// States that require human approval — the run is blocked waiting for input.
// These are treated as terminal in CI contexts because no automation can proceed.
var approvalStates = map[string]bool{
	"policy_checked":          true, // Awaiting policy override approval
	"policy_override":         true, // Policy override pending
	"cost_estimated":          true, // Awaiting cost approval
	"confirmed":               true, // Awaiting apply confirmation (when auto-apply is off)
	"planned":                 true, // Plan complete, awaiting confirmation to apply
}

// Success states (exit code 0)
var successStates = map[string]bool{
	"applied":              true,
	"planned_and_finished": true,
}

// waitForRun polls a run until it reaches a terminal state.
// Prints status transitions to stderr and exits with appropriate code.
func waitForRun(runID string, timeout time.Duration) {

	if runID == "" {
		fmt.Fprintln(os.Stderr, "Error: -run flag is required")
		os.Exit(ExitError)
	}

	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	maxInterval := 10 * time.Second
	lastStatus := ""

	fmt.Fprintf(os.Stderr, "Waiting for run %s...\n", runID)

	for {
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "Error: Timeout waiting for run %s after %s\n", runID, timeout)
			os.Exit(ExitTransientError)
		}

		status, runData := fetchRunStatus(runID)

		if status != lastStatus {
			if lastStatus != "" {
				fmt.Fprintf(os.Stderr, "%s -> %s\n", lastStatus, status)
			} else {
				fmt.Fprintf(os.Stderr, "Status: %s\n", status)
			}
			lastStatus = status
		}

		if terminalStates[status] {
			// Print the final run data to stdout
			fmt.Println(runData.StringIndent("", "  "))

			if successStates[status] {
				fmt.Fprintf(os.Stderr, "Run %s completed successfully (%s)\n", runID, status)
				os.Exit(ExitSuccess)
			} else {
				fmt.Fprintf(os.Stderr, "Run %s finished with status: %s\n", runID, status)
				os.Exit(ExitError)
			}
		}

		// Detect states that require human approval — no point waiting in CI
		if approvalStates[status] {
			fmt.Println(runData.StringIndent("", "  "))
			fmt.Fprintf(os.Stderr, "Run %s requires approval (status: %s). Cannot proceed automatically.\n", runID, status)
			os.Exit(ExitError)
		}

		time.Sleep(interval)

		// Backoff: increase interval up to max
		if interval < maxInterval {
			interval = interval + 1*time.Second
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}

// fetchRunStatus makes a GET request to fetch the run and returns its status and parsed data.
func fetchRunStatus(runID string) (string, *gabs.Container) {

	apiURL := "https://" + ScalrHostname + BasePath + "/runs/" + runID

	req, err := http.NewRequest("GET", apiURL, nil)
	checkErr(err)

	req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)
	req.Header.Add("Authorization", "Bearer "+ScalrToken)

	res, err := doWithRetry(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Request failed: %s\n", err)
		os.Exit(ExitTransientError)
	}

	resBody, err := io.ReadAll(res.Body)
	checkErr(err)
	res.Body.Close()

	if res.StatusCode >= 300 {
		showError(resBody, res.StatusCode)
	}

	response, err := gabs.ParseJSON(resBody)
	checkErr(err)

	parsed := parseData(response)

	// For single-object responses, parseData returns an array with one item
	item := parsed.Search("0")
	if item == nil {
		fmt.Fprintln(os.Stderr, "Error: Unexpected response format")
		os.Exit(ExitError)
	}

	status := ""
	if item.Exists("status") {
		status = item.Path("status").Data().(string)
	}

	return status, item
}
