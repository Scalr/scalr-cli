package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/getkin/kin-openapi/openapi3"
)

// Exit codes for scripting/CI use
const (
	ExitSuccess        = 0 // Command succeeded
	ExitError          = 1 // Any error (bad input, 4xx, missing flags, not found, etc.)
	ExitTransientError = 3 // Transient error (5xx, network, timeout) — safe to retry
)

// OutputOptions controls how API responses are rendered to the user.
type OutputOptions struct {
	Format    string // "json" (default), "table", "csv"
	Fields    string // comma-separated field list (filters output and controls table/csv column order)
	Query     string // dot-path expression like ".name" or ".[].id"
	Quiet     bool   // suppress all output (only exit code matters)
	Verbose   bool   // print HTTP request/response to stderr
}

// PaginationOptions controls how list responses are paginated.
// Zero values mean "default behavior" (fetch all pages at size 100).
type PaginationOptions struct {
	Page     int // specific page number; 0 means fetch all pages
	PageSize int // items per page; 0 means use default (100)
}

type Parameter struct {
	varType     string
	orgName     string
	description string
	required    bool
	enum        []any
	location    string
	value       *string
}

// Command aliases: short names for frequently used commands.
// Targets must match an actual operation ID (kebab-case) from the OpenAPI spec.
// If an alias target no longer exists, the CLI will report "Command not found"
// after expanding the alias.
var commandAliases = map[string]string{
	"ws":   "get-workspaces",
	"envs": "list-environments",
	"runs": "get-runs",
}

// Rename flags with odd names that causes issues in some shells
func renameFlag(name string) string {
	name = strings.ReplaceAll(name, "[", "-")
	name = strings.ReplaceAll(name, "]", "")

	return name
}

// pascalToKebab converts PascalCase to kebab-case, handling consecutive capitals
// as acronyms: "PolicyGroups" -> "policy-groups", "SSHKey" -> "ssh-key",
// "VCSProvider" -> "vcs-provider".
func pascalToKebab(s string) string {
	if s == "" {
		return ""
	}
	isUpper := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	isLower := func(c byte) bool { return c >= 'a' && c <= 'z' }

	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Insert dash before an uppercase letter that starts a new word.
		// A new word begins when the previous char is lowercase, OR when the next
		// char is lowercase (end of acronym run: "SSHKey" — 'K' before 'e').
		if i > 0 && isUpper(c) {
			prevLower := isLower(s[i-1])
			nextLower := i+1 < len(s) && isLower(s[i+1])
			if prevLower || nextLower {
				b.WriteByte('-')
			}
		}
		if isUpper(c) {
			b.WriteByte(c + ('a' - 'A'))
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func parseCommand(out OutputOptions, page PaginationOptions) {

	doc := loadAPI()

	command := flag.Arg(0)

	// originalArg is what the user actually typed (possibly an alias).
	// We need it later to find its position in os.Args for sub-flag parsing.
	originalArg := command

	// Resolve command aliases to the actual OpenAPI operation ID.
	if target, ok := commandAliases[command]; ok {
		command = target
	}

	for uri, path := range doc.Paths.Map() {
		for method, action := range path.Operations() {
			if strings.ReplaceAll(action.OperationID, "_", "-") != command {
				continue
			}

			//Found command, setup flags
			subFlag := flag.NewFlagSet(command, flag.ExitOnError)

			//Disable unwanted built-in flag features
			subFlag.Usage = func() {}

			query := url.Values{}

			contentType := ""

			//Will hold all valid flag values
			flags := make(map[string]Parameter)

			//Collect all valid URI flags for this command
			for _, parameter := range action.Parameters {

				//Ignore some flags
				if parameter.Value.Name == "page[number]" ||
					parameter.Value.Name == "page[size]" ||
					parameter.Value.Name == "fields" ||
					parameter.Value.Name == "Prefer" {
					continue
				}

				if parameter.Value.Schema.Value.Type.Is("string") ||
					parameter.Value.Schema.Value.Type.Is("boolean") ||
					parameter.Value.Schema.Value.Type.Is("integer") ||
					parameter.Value.Schema.Value.Type.Is("array") {

					flags[renameFlag(parameter.Value.Name)] = Parameter{
						location: parameter.Value.In,
						varType:  "string",
						orgName:  parameter.Value.Name,
						required: parameter.Value.Required,
						value:    new(string),
					}

					subFlag.StringVar(flags[renameFlag(parameter.Value.Name)].value, renameFlag(parameter.Value.Name), "", parameter.Value.Description)

					continue
				}

				//TODO: If code reaches here, means support for new field-type is needed!
				fmt.Fprintln(os.Stderr, "Warning: Unsupported field type, please report this issue:", parameter.Value.Name, parameter.Value.Schema.Value.Type)

			}

			var requiredFlags map[string]bool

			if method == "POST" || method == "PATCH" || method == "DELETE" {

				if action.RequestBody != nil {

					for contentType = range action.RequestBody.Value.Content {
					}

					//If no schema is defined for the body, no need to look for futher fields
					if action.RequestBody.Value.Content[contentType].Schema != nil {

						//Recursively collect all required fields
						requiredFlags = collectRequired(action.RequestBody.Value.Content[contentType].Schema.Value)

						var collectAttributes func(*openapi3.Schema, string, string)

						//Function to support nested objects
						collectAttributes = func(nested *openapi3.Schema, prefix string, inheritType string) {

							//Collect all availble attributes for this command
							for name, attribute := range nested.Properties {

								//Ignore read-only attributes in body
								if attribute.Value.ReadOnly {
									continue
								}

								flagName := prefix + name

								//Ignore ID-field that is redundant
								if flagName == "data-id" && inheritType == "" {
									continue
								}

								//Nested object, needs to drill down deeper
								if attribute.Value.Type.Is("object") {
									collectAttributes(attribute.Value, flagName+"-", "")
									continue
								}

								//Arrays might include objects that needs to be drilled down deeper
								if attribute.Value.Type.Is("array") && attribute.Value.Items.Value.Type.Is("object") {
									collectAttributes(attribute.Value.Items.Value, flagName+"-", "array")
									continue
								}

								// Resolve enum from AnyOf if Type is nil (e.g. provider-name, working-directory)
								enum := attribute.Value.Enum
								if attribute.Value.AnyOf != nil {
									for _, item := range attribute.Value.AnyOf {
										if item.Value.Enum != nil {
											enum = item.Value.Enum
										}
									}
								}

								required := false
								if requiredFlags[flagName] {
									required = true
								}

								//If flag is required and only one value is available, no need to offer it to the user
								if required && enum != nil && len(enum) == 1 {
									continue
								}

								flagName = shortenName(flagName)

								flags[flagName] = Parameter{
									location: "body",
									required: required,
									enum:     enum,
									value:    new(string),
								}

								subFlag.StringVar(flags[flagName].value, flagName, "", attribute.Value.Description)

							}

						}

						collectAttributes(action.RequestBody.Value.Content[contentType].Schema.Value, "", "")

					}

				}

			}

			//Find command position in args — use originalArg, not resolved command,
			//because aliases (e.g. "envs") appear in os.Args, not their target ("list-environments").
			pos := 1
			for index, arg := range os.Args {
				if arg == originalArg {
					pos = index
					break
				}
			}

			//Validate all flags
			subFlag.Parse(os.Args[pos+1:])

			//If command has -account flag and no value set, use default account-ID
			if flag, ok := flags["account"]; ok && *flag.value == "" {
				*flag.value = ScalrAccount
			}

			//If command has -account-id flag and no value set, use default account-ID
			if flag, ok := flags["account-id"]; ok && *flag.value == "" {
				*flag.value = ScalrAccount
			}

			var missing []string
			var missingBody []string

			//Sort flag values to correct locations
			for name, parameter := range flags {

				//Ignore empty flags..
				if *parameter.value == "" {

					//..Unless required
					if parameter.required {
						if parameter.location == "query" || parameter.location == "path" {
							missing = append(missing, name)
						} else {
							missingBody = append(missingBody, name)
						}

					}

					continue
				}

				// Attempt name-to-ID resolution for path/query parameters
				if parameter.location == "path" || parameter.location == "query" {
					*parameter.value = resolveNameToID(name, *parameter.value)
				}

				switch parameter.location {
				case "query":
					//This flag value should be sent as a query parameter
					query.Add(parameter.orgName, *parameter.value)

				case "path":
					//This flag value goes in the URI
					uri = strings.Replace(uri, "{"+parameter.orgName+"}", *parameter.value, 1)
				}

			}

			var body string

			if method == "POST" || method == "PATCH" || method == "DELETE" {

				//If stdin contains data, use that as Body
				stat, _ := os.Stdin.Stat()
				if stat.Mode()&os.ModeNamedPipe != 0 ||
					(stat.Mode()&os.ModeCharDevice == 0) && stat.Size() > 0 {

					if len(missing) > 0 {
						fmt.Fprintf(os.Stderr, "Missing required flag(s): %s\n", missing)
						os.Exit(ExitError)
					}

					var stdin []byte
					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						stdin = append(stdin, scanner.Bytes()...)
					}
					err := scanner.Err()
					checkErr(err)

					body = string(stdin)
				}

				if len(body) == 0 {
					// FIXME: Disable required attributes for PATCH requests as the specs are incorrect
					if method != "PATCH" {
						if len(missingBody) > 0 || len(missing) > 0 {
							fmt.Fprintf(os.Stderr, "Missing required flag(s): %s\n", append(missing, missingBody...))
							os.Exit(ExitError)
						}
					} else {
						if len(missing) > 0 {
							fmt.Fprintf(os.Stderr, "Missing required flag(s): %s\n", missing)
							os.Exit(ExitError)
						}
					}

					if action.RequestBody != nil {
						raw := gabs.New()

						var collectAttributes func(*openapi3.Schema, string)

						//Function to support nested objects
						collectAttributes = func(nested *openapi3.Schema, prefix string) {

							//Collect all availble attributes for this command
							for name, attribute := range nested.Properties {

								//Ignore read-only attributes in body
								if attribute.Value.ReadOnly {
									continue
								}

								path := prefix + name

								//Nested object, needs to drill down deeper
								if attribute.Value.Type.Is("object") {
									collectAttributes(attribute.Value, path+".")
									continue
								}

								//Special case for arrays of objects used in relationships
								if attribute.Value.Type.Is("array") && attribute.Value.Items.Value.Type.Is("object") {
									path = path + ".id"
									continue
								}

								flagName := strings.ReplaceAll(path, ".", "-")

								required := false
								if requiredFlags[flagName] {
									required = true
								}

								//Special case to auto-add type in relationships if ID is set
								if strings.HasPrefix(flagName, "data-relationships-") && name == "type" {
									id := strings.Replace(shortenName(flagName), "-data-type", "-id", 1)

									if *flags[id].value != "" {
										required = true
									} else {
										required = false
									}
								}

								//If required and only one value is available, use it
								if required && attribute.Value.Enum != nil && len(attribute.Value.Enum) == 1 {
									raw.SetP(attribute.Value.Enum[0], path)
									continue
								}

								flagName = shortenName(flagName)

								if _, ok := flags[flagName]; !ok {
									continue
								}

								value := *flags[flagName].value

								//Skip attribute if not set
								if value == "" {
									continue
								}

								theType := attribute.Value.Type

								// Resolve type from AnyOf (e.g. provider-name, working-directory)
								if theType == nil && attribute.Value.AnyOf != nil {
									for _, item := range attribute.Value.AnyOf {
										if item.Value.Type != nil {
											theType = item.Value.Type
											break
										}
									}
								}

								//If no type is specified, it's a relationship
								if theType == nil {
									theType = &openapi3.Types{}
								}

								//If this is a relationship, strip prefix and -data- to shorten flag-names
								if attribute.Value.Type != nil && attribute.Value.Type.Is("object") {
									flagName = strings.TrimPrefix(flagName, "data-relationships-")
									flagName = strings.Replace(flagName, "-data-id", "-id", 1)
								}

								switch {
								case theType.Is("object"):
									//Special case for arrays in relationships
									for _, item := range strings.Split(value, ",") {
										sub := gabs.New()
										sub.Set(item, "id")
										sub.Set(attribute.Value.Items.Value.Properties["type"].Value.Enum[0], "type")

										raw.ArrayAppendP(sub.Data(), strings.TrimSuffix(path, ".id"))
									}

								case theType.Is("boolean"):
									val, _ := strconv.ParseBool(value)
									raw.SetP(val, path)

								case theType.Is("string"):
									raw.SetP(value, path)

								case theType.Is("integer"):
									val, _ := strconv.Atoi(value)
									raw.SetP(val, path)

								case theType.Is("array"):
									raw.SetP(strings.Split(value, ","), path)

								default:
									//TODO: If code reaches here, means we need to add support for more field types!
									fmt.Fprintln(os.Stderr, "Warning: Unsupported field type:", name, attribute.Value.Type)
								}

							}

						}

						collectAttributes(action.RequestBody.Value.Content[contentType].Schema.Value, "")

						body = raw.StringIndent("", "  ")
					}
				}

			} else {
				if len(missing) > 0 {
					fmt.Fprintf(os.Stderr, "Missing required flag(s): %s\n", missing)
					os.Exit(ExitError)
				}
			}

			// Special case for assume-service-account.
			if command == "assume-service-account" {
				// Extract hostname from parameter
				email := flags["service-account-email"].value

				parts := strings.Split(*email, "@")
				if len(parts) != 2 || parts[1] == "" {
					fmt.Fprintln(os.Stderr, "Error: Invalid service account email format")
					os.Exit(ExitError)
				}

				host := parts[1]

				// Validate hostname to prevent SSRF attacks
				if !isValidExternalHost(host) {
					fmt.Fprintf(os.Stderr, "Error: Invalid hostname '%s' extracted from service account email\n", host)
					os.Exit(ExitError)
				}

				ScalrHostname = host
			}

			// Detect resource type from the x-resource extension (used for table column defaults).
			// Used only as a fallback; formatTable prefers the "type" field from the actual response data.
			resourceType := ""
			if action.Extensions["x-resource"] != nil {
				if rt, ok := action.Extensions["x-resource"].(string); ok {
					resourceType = pascalToKebab(rt)
				}
			}

			//Make request to the API
			callAPI(method, uri, query, body, contentType, resourceType, out, page)

			return
		}
	}

	//Command not found
	fmt.Fprintf(os.Stderr, "\nCommand '%s' not found. Use -help to list available commands.\n\n", command)
	os.Exit(ExitError)
}

// Helper function to shorter flag-names for convenience
func shortenName(flagName string) string {

	//If this is an attribute, strip prefix to shorten flag-names
	flagName = strings.TrimPrefix(flagName, "data-attributes-")

	//If this is a relationship, strip prefix and -data- to shorten flag-names
	if strings.HasPrefix(flagName, "data-relationships-") {
		flagName = strings.TrimPrefix(flagName, "data-relationships-")
		flagName = strings.Replace(flagName, "-data-id", "-id", 1)
	}

	flagName = strings.TrimPrefix(flagName, "data-")

	return flagName
}

// isValidExternalHost rejects hostnames that point to localhost or private networks to prevent SSRF
func isValidExternalHost(host string) bool {
	// Must contain at least one dot (reject "localhost", single-label names)
	if !strings.Contains(host, ".") {
		return false
	}

	// Reject if it parses as an IP address (we expect a domain name)
	if ip := net.ParseIP(host); ip != nil {
		return false
	}

	// Reject well-known localhost aliases
	lower := strings.ToLower(host)
	if strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") {
		return false
	}

	return true
}

// Make a request to the Scalr API
func callAPI(method string, uri string, query url.Values, body string, contentType string, resourceType string, out OutputOptions, pageOpts PaginationOptions) {

	output := gabs.New()
	output.Array()

	// Pagination: use user-specified page size or default to 100
	effectivePageSize := 100
	if pageOpts.PageSize > 0 {
		effectivePageSize = pageOpts.PageSize
	}
	query.Add("page[size]", strconv.Itoa(effectivePageSize))

	// If -page is specified, only fetch that single page
	singlePage := pageOpts.Page > 0
	startPage := 1
	if singlePage {
		startPage = pageOpts.Page
	}

	var lastPaginationMeta *gabs.Container

	for pageNum := startPage; true; pageNum++ {

		query.Set("page[number]", strconv.Itoa(pageNum))

		if out.Verbose {
			fmt.Fprintln(os.Stderr, method, "https://"+ScalrHostname+BasePath+uri+"?"+query.Encode())

			if contentType != "" {
				fmt.Fprintln(os.Stderr, "Content-Type = "+contentType)
				fmt.Fprintln(os.Stderr, body)
			}

		}

		// Show spinner for non-verbose, non-quiet, TTY sessions
		var stopSpinner func()
		if !out.Verbose && !out.Quiet {
			if pageNum > 1 {
				stopSpinner = paginationSpinner(pageNum)
			} else {
				stopSpinner = startSpinner("")
			}
		}

		req, err := http.NewRequest(method, "https://"+ScalrHostname+BasePath+uri+"?"+query.Encode(), strings.NewReader(body))
		checkErr(err)

		setScalrHeaders(req)

		if contentType != "" {
			req.Header.Add("Content-Type", contentType)
		}

		res, err := doWithRetry(req)
		if stopSpinner != nil {
			stopSpinner()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Request failed: %s\n", err)
			os.Exit(ExitTransientError)
		}

		resBody, err := io.ReadAll(res.Body)
		res.Body.Close()
		checkErr(err)

		if out.Verbose {
			//Show raw server response
			fmt.Fprintln(os.Stderr, string(resBody))
		}

		if res.StatusCode >= 300 {
			showError(resBody, res.StatusCode)
		}

		//Empty response (e.g. 204 No Content from DELETE), quit early
		if len(resBody) == 0 {
			if !out.Quiet && isTerminal() {
				fmt.Fprintln(os.Stderr, "Done.")
			}
			return
		}

		//If not a JSON:API response, just render it raw
		//These responses don't follow the data/attributes structure so they
		//bypass table/csv formatting — but we still pretty-print valid JSON.
		if !strings.HasPrefix(res.Header.Get("content-type"), "application/vnd.api+json") {
			if !out.Quiet {
				if parsed, err := gabs.ParseJSON(resBody); err == nil {
					fmt.Println(parsed.StringIndent("", "  "))
				} else {
					fmt.Println(string(resBody))
				}
			}

			if uri == "/service-accounts/assume" && strings.HasPrefix(res.Header.Get("content-type"), "application/json") {
				response, err := gabs.ParseJSON(resBody)
				checkErr(err)

				// Extract token from response
				token := response.Path("access-token").Data().(string)

				// Save token to credentials.tfrc.json and scalr.conf
				addTerraformToken(ScalrHostname, token)
			}

			return
		}

		//Check if paging is needed
		response, err := gabs.ParseJSON(resBody)
		checkErr(err)

		//If data is empty, just send empty array
		if len(response.Search("data").Children()) == 0 && len(response.Search("data").ChildrenMap()) == 0 {
			break
		}

		arrayResponse := response.Exists("data", "0")

		newItems := parseData(response)

		//If this is a single item response, return it instead of an array
		if !arrayResponse {
			output = newItems.Search("0")
			break
		}

		for _, data := range newItems.Children() {
			output.ArrayAppend(data.Data())
		}

		// Save pagination metadata for display
		if response.Exists("meta", "pagination") {
			lastPaginationMeta = response.Path("meta.pagination")
		}

		// If fetching a single page, stop after one iteration
		if singlePage {
			break
		}

		if response.Path("meta.pagination.next-page").Data() == nil {
			break
		}

	}

	if !out.Quiet {
		// Detect whether output is a list or a single object.
		// output starts as an empty array (gabs.Array()). For single-item
		// responses it gets reassigned to the item itself (a map). For list
		// responses it stays as an array with elements appended.
		// We check .Exists("0") to distinguish: arrays have numeric indices,
		// single objects do not.
		isArray := output.Exists("0")

		// Handle empty results: if output is still the initial empty array
		// (no items appended, not reassigned to a single item), render
		// appropriately instead of falling through to formatKeyValue.
		isEmpty := false
		if arr, ok := output.Data().([]interface{}); ok && len(arr) == 0 {
			isEmpty = true
		}
		if isEmpty {
			if out.Format == "json" {
				fmt.Println("[]")
			} else {
				fmt.Fprintln(os.Stderr, "No results found.")
			}
		} else {
			if out.Fields != "" {
				output = filterFields(output, out.Fields, isArray)
			}

			// Apply query expression if requested
			if out.Query != "" {
				result, isSimple := applyQuery(output, out.Query, isArray)
				formatQueryResult(result, isSimple)
			} else {
				// Format and display output
				formatOutput(output, out.Format, isArray, out.Fields, resourceType)
			}
		}

		// Show pagination summary in table/csv mode.
		// When -page was used, show which page we fetched.
		// When all pages were fetched (default), show only the total count —
		// showing "page 5 of 5" is misleading since we displayed every page.
		if (out.Format == "table" || out.Format == "csv") && out.Query == "" && lastPaginationMeta != nil && !isEmpty {
			totalCount := lastPaginationMeta.Path("total-count").Data()
			if singlePage {
				totalPages := lastPaginationMeta.Path("total-pages").Data()
				formatPaginationInfo(startPage, totalPages, totalCount)
			} else if totalCount != nil {
				formatTotalCount(totalCount)
			}
		}
	}
}

// Parse error response and show human-readable error message.
// Falls back to raw JSON if the response doesn't follow JSONAPI error format.
// Uses distinct exit codes: ExitError (1) for 4xx, ExitTransientError (3) for 5xx.
func showError(resBody []byte, httpStatus ...int) {

	// Determine exit code based on HTTP status
	exitCode := ExitError
	if len(httpStatus) > 0 && httpStatus[0] >= 500 {
		exitCode = ExitTransientError
	}

	jsonParsed, err := gabs.ParseJSON(resBody)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: Server did not return a valid JSON response")
		fmt.Fprintln(os.Stderr, string(resBody))
		os.Exit(exitCode)
	}

	// Try to parse JSONAPI errors array for human-readable output
	if jsonParsed.Exists("errors") {
		printed := false
		for _, errObj := range jsonParsed.Path("errors").Children() {
			status := ""
			if errObj.Exists("status") {
				status = fmt.Sprintf("%v", errObj.Path("status").Data())
			}

			title := ""
			if errObj.Exists("title") {
				title = fmt.Sprintf("%v", errObj.Path("title").Data())
			}

			detail := ""
			if errObj.Exists("detail") {
				detail = fmt.Sprintf("%v", errObj.Path("detail").Data())
			}

			pointer := ""
			if errObj.ExistsP("source.pointer") {
				pointer = fmt.Sprintf("%v", errObj.Path("source.pointer").Data())
			}

			// Build a concise error line
			parts := make([]string, 0, 4)
			if status != "" {
				parts = append(parts, status)
			}
			if title != "" {
				parts = append(parts, title)
			}
			if detail != "" && detail != title {
				parts = append(parts, detail)
			}
			if pointer != "" {
				parts = append(parts, "(field: "+pointer+")")
			}

			if len(parts) > 0 {
				fmt.Fprintln(os.Stderr, "Error:", strings.Join(parts, ": "))
				printed = true
			}
		}

		if printed {
			os.Exit(exitCode)
		}
	}

	// Fallback: print raw JSON
	fmt.Fprintln(os.Stderr, jsonParsed.StringIndent("", "  "))
	os.Exit(exitCode)
}

// Data JSON:API data to make it easier to work with
func parseData(response *gabs.Container) *gabs.Container {

	output := gabs.New()
	output.Array()

	//Convert non-array to array if needed
	if !response.Exists("data", "0") {
		item := response.Path("data").Data()
		response.Array("data")
		response.ArrayAppend(item, "data")
	}

	included := gabs.New()

	for _, include := range response.Path("included").Children() {
		typeVal, _ := include.Path("type").Data().(string)
		idVal, _ := include.Path("id").Data().(string)
		if typeVal != "" && idVal != "" {
			included.Set(include.Data(), typeVal+"-"+idVal)
		}
	}

	for _, value := range response.Path("data").Children() {

		sub := gabs.New()

		sub.Set(value.Search("attributes").Data())
		sub.SetP(value.Search("id"), "id")
		sub.SetP(value.Search("type"), "type")

		// Include links section if it exists (important for create-configuration-version upload-url)
		if value.Search("links") != nil {
			sub.SetP(value.Search("links"), "links")
		}

		// Include meta section if it exists (may contain additional metadata)
		if value.Search("meta") != nil {
			sub.SetP(value.Search("meta"), "meta")
		}

		for name, relationship := range value.Search("relationships").ChildrenMap() {

			if relationship.Data() == nil {
				continue
			}

			//Function to support relationship arrays
			//TODO: Should probably move this outside of the loop for performance reason, but will make code less readable
			var connectRelationship = func(rel *gabs.Container) *gabs.Container {

				relType, _ := rel.Path("type").Data().(string)
				relIdVal, _ := rel.Path("id").Data().(string)
				if relType == "" || relIdVal == "" {
					return rel
				}
				relId := relType + "-" + relIdVal

				if included.Exists(relId) {

					addition := gabs.New()

					//Include attributes
					addition.Set(included.Search(relId, "attributes").Data())

					//Include ID and type
					addition.Set(rel.Path("id"), "id")
					addition.Set(rel.Path("type"), "type")

					//Include sub-relationship IDs
					//TODO: Does this need support for arrays in sub-relationships? Probably...
					for subName, subRelationship := range included.Search(relId, "relationships").ChildrenMap() {

						if subRelationship.Data() == nil {
							continue
						}

						addition.Set(subRelationship.Path("data.id"), subName+"-id")
					}

					return addition
				}

				return rel

			}

			if !relationship.ExistsP("data.id") {
				//Assume this is an array
				sub.ArrayP(name)

				for _, value := range relationship.Path("data").Children() {
					sub.ArrayAppend(connectRelationship(value), name)
				}
				continue
			}

			sub.SetP(connectRelationship(relationship.Path("data")), name)

		}

		output.ArrayAppend(sub.Data())

	}

	return output

}
