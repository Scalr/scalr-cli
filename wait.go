package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Jeffail/gabs/v2"
)

// Terminal states — the run is finished and will not change further.
var terminalStates = map[string]bool{
	"applied":              true,
	"errored":              true,
	"discarded":            true,
	"canceled":             true,
	"planned_and_finished": true,
	"force_canceled":       true,
}

// States that definitely block on human input regardless of run configuration.
// A run sitting in these states will not progress without someone clicking approve.
var blockedOnApprovalStates = map[string]bool{
	"policy_checked":  true, // Soft-mandatory policy failed; needs override
	"policy_override": true, // Override was requested; awaiting action
	"cost_estimated":  true, // Cost estimate produced; awaiting approval (if gated)
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

		// Hard stop: states that definitely block on human input regardless of config
		// (policy override or cost approval).
		if blockedOnApprovalStates[status] {
			fmt.Println(runData.StringIndent("", "  "))
			fmt.Fprintf(os.Stderr, "Run %s is blocked waiting for approval (status: %s). Cannot proceed automatically.\n", runID, status)
			os.Exit(ExitError)
		}

		// "planned" is ambiguous — the run auto-applies if auto-apply is on, otherwise
		// it sits waiting for manual confirmation. Check the run's auto-apply flag.
		// If auto-apply is off, the run is effectively blocked.
		if status == "planned" {
			autoApply, ok := runData.Path("auto-apply").Data().(bool)
			if ok && !autoApply {
				fmt.Println(runData.StringIndent("", "  "))
				fmt.Fprintf(os.Stderr, "Run %s requires manual confirmation (auto-apply is disabled). Cannot proceed automatically.\n", runID)
				os.Exit(ExitError)
			}
			// Otherwise keep polling — the run will transition to confirmed/applying shortly.
		}
		// "confirmed" is a brief transitional state on the way to applying; just keep polling.

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

	setScalrHeaders(req)

	res, err := doWithRetry(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Request failed: %s\n", err)
		os.Exit(ExitTransientError)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	checkErr(err)

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
		status, _ = item.Path("status").Data().(string)
	}

	return status, item
}
