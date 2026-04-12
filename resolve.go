package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/Jeffail/gabs/v2"
)

// scalrIDPattern matches Scalr resource IDs like "ws-abc123", "env-def456", "run-ghi789"
var scalrIDPattern = regexp.MustCompile(`^[a-z]+-[a-zA-Z0-9]+$`)

// resolvableResources maps flag names to API list endpoints with name filter support.
// The value is the path to the list endpoint (relative to BasePath).
var resolvableResources = map[string]string{
	"workspace":      "/workspaces",
	"workspace-id":   "/workspaces",
	"environment":    "/environments",
	"environment-id": "/environments",
	"account":        "/accounts",
	"account-id":     "/accounts",
	"tag":            "/tags",
	"tag-id":         "/tags",
	"role":           "/roles",
	"role-id":        "/roles",
	"team":           "/teams",
	"team-id":        "/teams",
	"vcs-provider":    "/vcs-providers",
	"vcs-provider-id": "/vcs-providers",
	"agent-pool":      "/agent-pools",
	"agent-pool-id":   "/agent-pools",
}

// isScalrID checks if a value looks like a Scalr resource ID.
func isScalrID(value string) bool {
	return scalrIDPattern.MatchString(value)
}

// resolveNameToID attempts to resolve a human-readable name to a Scalr resource ID.
// If the value already looks like an ID, it is returned unchanged.
// If resolution fails or matches multiple resources, an error is printed and the program exits.
func resolveNameToID(flagName string, value string) string {
	if value == "" || isScalrID(value) {
		return value
	}

	endpoint, ok := resolvableResources[flagName]
	if !ok {
		// Not a resolvable resource — return as-is (might be a legitimate string value)
		return value
	}

	// Query the list endpoint with a name filter
	params := url.Values{}
	params.Set("filter[name]", value)
	params.Set("page[size]", "10")

	// For workspaces and some resources, also need account filter
	if ScalrAccount != "" && (strings.Contains(endpoint, "/workspaces") ||
		strings.Contains(endpoint, "/environments") ||
		strings.Contains(endpoint, "/tags") ||
		strings.Contains(endpoint, "/roles") ||
		strings.Contains(endpoint, "/teams")) {
		params.Set("filter[account]", ScalrAccount)
	}

	apiURL := "https://" + ScalrHostname + BasePath + endpoint + "?" + params.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		// Can't resolve — return original value and let the API handle it
		return value
	}

	req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)
	req.Header.Add("Authorization", "Bearer "+ScalrToken)

	res, err := scalrHTTPClient.Do(req)
	if err != nil {
		return value
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return value
	}

	if res.StatusCode >= 300 {
		// Resolution failed — return original and let the main request handle errors
		return value
	}

	response, err := gabs.ParseJSON(resBody)
	if err != nil {
		return value
	}

	items := response.Path("data").Children()

	if len(items) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No %s found with name '%s'\n", strings.TrimSuffix(flagName, "-id"), value)
		os.Exit(ExitError)
	}

	if len(items) == 1 {
		id := items[0].Path("id").Data().(string)
		name := ""
		if items[0].Exists("attributes", "name") {
			name = items[0].Path("attributes.name").Data().(string)
		}
		if name != "" {
			fmt.Fprintf(os.Stderr, "Resolved %s '%s' -> %s\n", flagName, value, id)
		}
		return id
	}

	// Multiple matches
	fmt.Fprintf(os.Stderr, "Error: Multiple %s resources match name '%s':\n", strings.TrimSuffix(flagName, "-id"), value)
	for _, item := range items {
		id := item.Path("id").Data().(string)
		name := ""
		if item.Exists("attributes", "name") {
			name = item.Path("attributes.name").Data().(string)
		}
		fmt.Fprintf(os.Stderr, "  %s  %s\n", id, name)
	}
	fmt.Fprintln(os.Stderr, "Please specify the exact ID.")
	os.Exit(ExitError)
	return value // unreachable
}
