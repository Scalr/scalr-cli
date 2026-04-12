package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	ScalrHostname string
	ScalrToken    string
	ScalrAccount  string
	BasePath      string
	// Version information - set at build time
	versionCLI = "dev"     // Default for development builds
	buildDate  = "unknown" // Build timestamp
)

// Color codes — initialized with values, disabled by -no-color, NO_COLOR env, or CI env.
var (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorBlue  = "\033[34m"
)

// disableColors turns off all ANSI color codes (for CI, piped output, NO_COLOR).
func disableColors() {
	colorReset = ""
	colorRed = ""
	colorBlue = ""
}

func main() {

	//Handle panics
	defer func() {
		err := recover()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	//Disable unwanted built-in flag features
	flag.Usage = func() {}
	flag.Bool("h", false, "")

	help := flag.Bool("help", false, "")
	configure := flag.Bool("configure", false, "")
	verbose := flag.Bool("verbose", false, "")
	version := flag.Bool("version", false, "")
	format := flag.String("format", "", "")
	update := flag.Bool("update", false, "")
	autocomplete := flag.Bool("autocomplete", false, "")
	quiet := flag.Bool("quiet", false, "")
	columns := flag.String("columns", "", "")
	fields := flag.String("fields", "", "")
	pageSize := flag.Int("page-size", 0, "")
	pageNum := flag.Int("page", 0, "")
	profile := flag.String("profile", "", "")
	queryExpr := flag.String("query", "", "")
	noColor := flag.Bool("no-color", false, "")

	//Only parse the flags if this is not a tab completion request
	if os.Getenv("COMP_LINE") == "" {

		if len(os.Args[1:]) == 0 {
			printInfo()
			return
		}

		if len(os.Args) == 3 && (os.Args[2] == "-help" || os.Args[2] == "--help") {
			os.Args = []string{os.Args[0], os.Args[2], os.Args[1]}
		}

		flag.Parse()

		if *version {
			runVersion()
			return
		}

		if *configure {
			runConfigure()
			return
		}

		if *update {
			runUpdate()
			return
		}

		if *autocomplete {
			enableAutocomplete()
			return
		}

	}

	// Disable colors in CI environments or when explicitly requested
	// Respects the NO_COLOR convention (https://no-color.org/)
	if *noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("CI") != "" {
		disableColors()
	}

	//Load config from environment
	ScalrHostname = os.Getenv("SCALR_HOSTNAME")
	ScalrToken = os.Getenv("SCALR_TOKEN")
	ScalrAccount = os.Getenv("SCALR_ACCOUNT")

	// Determine which profile to use
	activeProfile := *profile
	if activeProfile == "" {
		activeProfile = os.Getenv("SCALR_PROFILE")
	}

	//Load config from scalr.conf
	ScalrHostname, ScalrToken, ScalrAccount = loadConfigScalr(ScalrHostname, ScalrToken, ScalrAccount, activeProfile)

	if ScalrToken == "" {
		//Load config from credentials.tfrc.json
		ScalrHostname, ScalrToken = loadConfigTerraform(ScalrHostname, ScalrToken)
	}

	if ScalrHostname == "" {
		ScalrHostname = "scalr.io"
	}

	if ScalrToken == "" && !*help && flag.Arg(0) != "assume-service-account" {
		//End here if this is a completion request
		if os.Getenv("COMP_LINE") != "" {
			return
		}

		fmt.Fprint(os.Stderr, "\n", "Not configured! Please run 'scalr -configure' or set environment variables SCALR_HOSTNAME and SCALR_TOKEN", "\n\n")
		return
	}

	//This is tab compretion request
	if os.Getenv("COMP_LINE") != "" {
		runAutocomplete()
		return
	}

	if *help {
		printHelp()
		return
	}

	// Handle built-in commands that bypass OpenAPI
	if flag.Arg(0) == "wait-for-run" {
		// Parse sub-flags for wait-for-run
		waitFlags := flag.NewFlagSet("wait-for-run", flag.ExitOnError)
		waitFlags.Usage = func() {}
		waitRun := waitFlags.String("run", "", "")
		waitTimeout := waitFlags.Duration("timeout", 30*time.Minute, "")

		// Find command position in args
		pos := 1
		for i, arg := range os.Args {
			if arg == "wait-for-run" {
				pos = i
				break
			}
		}
		waitFlags.Parse(os.Args[pos+1:])

		// Need to load API to set BasePath
		doc := loadAPI()
		_ = doc
		waitForRun(*waitRun, *waitTimeout)
		return
	}

	// Determine actual output format (auto-detect TTY if not explicitly set)
	formatExplicit := *format != ""
	actualFormat := *format
	if actualFormat == "" {
		actualFormat = "json"
	}
	actualFormat = resolveFormat(actualFormat, formatExplicit)

	parseCommand(actualFormat, *verbose, *quiet, *columns, *fields, *pageSize, *pageNum, *queryExpr)
}

// Check for error and panic
func checkErr(e error) {
	if e != nil {
		panic(e)
	}
}

// Load config from scalr.conf (supports both flat format and profile-based format)
func loadConfigScalr(hostname string, token string, account string, profile string) (string, string, string) {
	home, err := os.UserHomeDir()
	checkErr(err)

	home = home + "/.scalr/"
	config := "scalr.conf"

	content, err := os.ReadFile(home + config)
	if err != nil {
		return hostname, token, account
	}

	jsonParsed, err := gabs.ParseJSON(content)
	checkErr(err)

	// Determine the config source: profile-based or flat (legacy)
	configSource := jsonParsed

	if profile != "" {
		// Explicit profile requested
		if jsonParsed.Exists(profile) {
			configSource = jsonParsed.Path(profile)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Profile '%s' not found in scalr.conf, using defaults.\n", profile)
			return hostname, token, account
		}
	} else if jsonParsed.Exists("default") && !jsonParsed.Exists("hostname") {
		// New format detected (has "default" key but no top-level "hostname")
		configSource = jsonParsed.Path("default")
	}
	// Otherwise: legacy flat format, configSource stays as jsonParsed

	if configSource.Search("hostname") != nil && hostname == "" {
		hostname = configSource.Search("hostname").Data().(string)
	}

	if configSource.Search("token") != nil && token == "" {
		token = configSource.Search("token").Data().(string)
	}

	if configSource.Search("account") != nil && account == "" {
		account = configSource.Search("account").Data().(string)
	}

	return hostname, token, account
}

// Load config from credentials.tfrc.json
func loadConfigTerraform(hostname string, token string) (string, string) {
	home, err := os.UserHomeDir()
	checkErr(err)

	content, err := os.ReadFile(home + "/.terraform.d/credentials.tfrc.json")
	if err != nil {
		return hostname, token
	}

	jsonParsed, err := gabs.ParseJSON(content)
	checkErr(err)

	if hostname != "" {
		//Try to load token for current hostname
		if jsonParsed.Search("credentials", hostname, "token") != nil {
			token = jsonParsed.Search("credentials", hostname, "token").Data().(string)
		}
	} else {
		credentials := jsonParsed.Search("credentials").ChildrenMap()
		if len(credentials) == 1 {
			//Only exactly one credential entry exists, use it
			for key, value := range credentials {
				hostname = key
				if value.Search("token") != nil {
					token = value.Search("token").Data().(string)
				}
			}
		}
	}

	return hostname, token
}

// Adds token to credentials.tfrc.json and scalr.conf
func addTerraformToken(hostname string, token string) {
	home, err := os.UserHomeDir()
	checkErr(err)

	filePath := home + "/.terraform.d/credentials.tfrc.json"

	content, err := os.ReadFile(filePath)
	if err != nil {
		// Create directory if it does not exist
		os.MkdirAll(home+"/.terraform.d/", 0700)

		content = []byte("{}")
	}

	jsonParsed, err := gabs.ParseJSON(content)
	checkErr(err)

	jsonParsed.Set(token, "credentials", hostname, "token")

	err = os.WriteFile(filePath, []byte(jsonParsed.StringIndent("", "  ")), 0600)
	checkErr(err)

	filePath = home + "/.scalr/scalr.conf"

	content, err = os.ReadFile(filePath)
	if err != nil {
		// Create directory if it does not exist
		os.MkdirAll(home+"/.scalr/", 0700)

		content = []byte("{}")
	}

	jsonParsed, err = gabs.ParseJSON(content)
	checkErr(err)

	jsonParsed.Set(hostname, "hostname")
	jsonParsed.Set(token, "token")

	err = os.WriteFile(filePath, []byte(jsonParsed.StringIndent("", "  ")), 0600)
	checkErr(err)
}

// Loads OpenAPI specification
func loadAPI() *openapi3.T {
	cacheDir, err := os.UserCacheDir()
	checkErr(err)

	cacheDir = cacheDir + "/.scalr/"

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		os.MkdirAll(cacheDir, 0700)
	}

	spec := cacheDir + "cache-openapi-public.yml"

	specURL := "https://" + ScalrHostname + "/api/iacp/v3/openapi-public.yml"

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Prevent loading external example files which makes the CLI too slow
	loader.ReadFromURIFunc = disableExternalFiles(
		openapi3.ReadFromURIs(
			openapi3.ReadFromHTTP(http.DefaultClient),
			openapi3.ReadFromFile,
		),
	)

	var doc *openapi3.T

	if info, err := os.Stat(spec); !os.IsNotExist(err) {
		if time.Since(info.ModTime()).Hours() > 24 {
			// Cache is more than 24 hours old, re-Download...
			var dlErr error
			doc, dlErr = downloadAndValidateSpec(loader, specURL, spec, cacheDir)
			if dlErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not refresh API spec: %s. Using cached version.\n", dlErr)
				doc = nil
			}
		}
	} else {
		// Download spec
		var dlErr error
		doc, dlErr = downloadAndValidateSpec(loader, specURL, spec, cacheDir)
		if dlErr != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not download API specification from %s: %s\n", ScalrHostname, dlErr)
			fmt.Fprintf(os.Stderr, "Please check your SCALR_HOSTNAME setting and network connection.\n")
			os.Exit(1)
		}
	}

	if doc == nil {
		var err error
		doc, err = loader.LoadFromFile(spec)
		checkErr(err)
	}

	//Validate the specification
	err = doc.Validate(loader.Context)
	checkErr(err)

	//Read BasePath from servers section, if exists
	BasePath = ""
	if doc.Servers != nil {
		//fmt.Printf("%+#v", doc.Servers[0].URL)

		u := strings.ReplaceAll(doc.Servers[0].URL, "{", "")
		u = strings.ReplaceAll(u, "}", "")

		parts, err := url.Parse(u)
		checkErr(err)

		BasePath = parts.Path
	}

	return doc
}

func disableExternalFiles(reader openapi3.ReadFromURIFunc) openapi3.ReadFromURIFunc {

	return func(loader *openapi3.Loader, location *url.URL) (buf []byte, err error) {

		//Skip examples
		if strings.Contains(location.Path, "/examples/") {
			return []byte("value: {}"), nil
		}

		return reader(loader, location)
	}
}

// Downloads the API spec to a temp file, validate it parses, then replaces the cache.
// Returns the parsed doc so it can be reused without loading from disk again.
func downloadAndValidateSpec(loader *openapi3.Loader, specURL string, specPath string, cacheDir string) (
	*openapi3.T,
	error,
) {
	tmpFile := cacheDir + "cache-openapi-public.yml.tmp"

	if err := downloadFile(specURL, tmpFile); err != nil {
		os.Remove(tmpFile)
		return nil, err
	}

	// Validate the downloaded spec can be parsed before replacing the cache
	doc, err := loader.LoadFromFile(tmpFile)
	if err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("downloaded spec is invalid: %s", err)
	}

	if err := os.Rename(tmpFile, specPath); err != nil {
		os.Remove(tmpFile)
		return nil, err
	}

	return doc, nil
}

// Downloads a file
func downloadFile(URL string, fileName string) error {

	client := &http.Client{}

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("received non-200 response code (%d) from server", resp.StatusCode)
	}

	//Create a empty file
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString(string(body))
	file.Sync()

	return nil
}

// Recursively collect all required fields
func collectRequired(root *openapi3.Schema) map[string]bool {

	requiredFields := make(map[string]bool)

	var recursive func(*openapi3.Schema, string)

	//Function to support nested objects
	recursive = func(nested *openapi3.Schema, prefix string) {

		//data should always be considered as required
		if prefix == "" && nested.Properties["data"] != nil {

			if nested.Properties["data"].Value.Type.Is("array") {
				recursive(nested.Properties["data"].Value.Items.Value, prefix+"data-")
			} else {
				recursive(nested.Properties["data"].Value, prefix+"data-")
			}

		}

		//Collect all availble attributes for this command
		for _, name := range nested.Required {

			requiredFields[prefix+name] = true

			//Nested object, needs to drill down deeper
			if nested.Properties[name].Value.Type.Is("object") {
				recursive(nested.Properties[name].Value, prefix+name+"-")
				continue
			}

			//Nested array of objects, needs to dril down deeper
			if nested.Properties[name].Value.Type.Is("array") && nested.Properties[name].Value.Items.Value.Type.Is("object") {
				recursive(nested.Properties[name].Value.Items.Value, prefix+name+"-")
				continue
			}

		}

	}

	recursive(root, "")

	return requiredFields
}
