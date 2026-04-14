package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/Jeffail/gabs/v2"
)

// openResource opens the Scalr dashboard URL for the given resource type and ID/name.
//
// URL patterns:
//   account:     https://{hostname}/v2/a/{account-id}/
//   environment: https://{hostname}/v2/e/{env-id}/workspaces/
//   workspace:   https://{hostname}/v2/e/{env-id}/workspaces/{workspace-id}/
//   run:         https://{hostname}/v2/e/{env-id}/workspaces/{workspace-id}/runs/{run-id}/
func openResource(resourceType string, identifier string) {

	var dashURL string

	switch resourceType {
	case "account", "acc":
		accountID := ScalrAccount
		if identifier != "" {
			accountID = resolveNameToID("account", identifier)
		}
		if accountID == "" {
			fmt.Fprintln(os.Stderr, "Error: No account specified. Use -account env var or pass an account ID.")
			os.Exit(ExitError)
		}
		dashURL = fmt.Sprintf("https://%s/v2/a/%s/", ScalrHostname, accountID)

	case "environment", "env":
		if identifier == "" {
			fmt.Fprintln(os.Stderr, "Error: Environment name or ID required.")
			fmt.Fprintln(os.Stderr, "Usage: scalr open environment <name-or-id>")
			os.Exit(ExitError)
		}
		envID := resolveNameToID("environment", identifier)
		dashURL = fmt.Sprintf("https://%s/v2/e/%s/workspaces/", ScalrHostname, envID)

	case "workspace", "ws":
		if identifier == "" {
			fmt.Fprintln(os.Stderr, "Error: Workspace name or ID required.")
			fmt.Fprintln(os.Stderr, "Usage: scalr open workspace <name-or-id>")
			os.Exit(ExitError)
		}
		wsID := resolveNameToID("workspace", identifier)
		envID := fetchRelationshipID("/workspaces/"+wsID, "environment")
		dashURL = fmt.Sprintf("https://%s/v2/e/%s/workspaces/%s/", ScalrHostname, envID, wsID)

	case "run":
		if identifier == "" {
			fmt.Fprintln(os.Stderr, "Error: Run ID required.")
			fmt.Fprintln(os.Stderr, "Usage: scalr open run <run-id>")
			os.Exit(ExitError)
		}
		runID := identifier
		wsID := fetchRelationshipID("/runs/"+runID, "workspace")
		envID := fetchRelationshipID("/workspaces/"+wsID, "environment")
		dashURL = fmt.Sprintf("https://%s/v2/e/%s/workspaces/%s/runs/%s/", ScalrHostname, envID, wsID, runID)

	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown resource type '%s'.\n", resourceType)
		fmt.Fprintln(os.Stderr, "Supported: account, environment, workspace, run")
		os.Exit(ExitError)
	}

	fmt.Fprintln(os.Stderr, dashURL)
	openBrowser(dashURL)
}

// fetchRelationshipID fetches a resource by API path and extracts a relationship ID.
// For example, fetchRelationshipID("/workspaces/ws-xxx", "environment") returns "env-yyy".
func fetchRelationshipID(apiPath string, relationshipName string) string {

	apiURL := "https://" + ScalrHostname + BasePath + apiPath

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(ExitError)
	}

	req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)
	req.Header.Add("Authorization", "Bearer "+ScalrToken)

	res, err := scalrHTTPClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Request failed: %s\n", err)
		os.Exit(ExitTransientError)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(ExitError)
	}

	if res.StatusCode >= 300 {
		showError(resBody, res.StatusCode)
	}

	response, err := gabs.ParseJSON(resBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid API response\n")
		os.Exit(ExitError)
	}

	// Extract relationship ID: data.relationships.{name}.data.id
	relID, ok := response.Path("data.relationships." + relationshipName + ".data.id").Data().(string)
	if !ok || relID == "" {
		fmt.Fprintf(os.Stderr, "Error: Could not find %s relationship for %s\n", relationshipName, apiPath)
		os.Exit(ExitError)
	}

	return relID
}

// openBrowser opens a URL in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		fmt.Fprintln(os.Stderr, "Cannot open browser on this platform. URL printed above.")
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening browser: %s\n", err)
		fmt.Fprintln(os.Stderr, "Open the URL above manually.")
	}
}
