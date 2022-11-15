package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/getkin/kin-openapi/openapi3"
)

type Parameter struct {
	varType     string
	orgName     string
	description string
	required    bool
	enum        []any
	location    string
	value       *string
}

//Rename flags with odd names that causes issues in some shells
func renameFlag(name string) string {
	name = strings.ReplaceAll(name, "[", "-")
	name = strings.ReplaceAll(name, "]", "")

	return name
}

func parseCommand(format string, verbose bool) {

	doc := loadAPI()

	command := flag.Arg(0)

	for uri, path := range doc.Paths {
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
				if parameter.Value.Name == "page[number]" || parameter.Value.Name == "page[size]" || parameter.Value.Name == "fields" {
					continue
				}

				if parameter.Value.Schema.Value.Type == "string" ||
					parameter.Value.Schema.Value.Type == "boolean" ||
					parameter.Value.Schema.Value.Type == "integer" ||
					parameter.Value.Schema.Value.Type == "array" {

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
				fmt.Println("IGNORE UNSUPPORTED FIELD", parameter.Value.Name, parameter.Value.Schema.Value.Type)

			}

			var requiredFlags map[string]bool

			if method == "POST" || method == "PATCH" || method == "DELETE" {

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
							if attribute.Value.Type == "object" {
								collectAttributes(attribute.Value, flagName+"-", "")
								continue
							}

							//Arrays might include objects that needs to be drilled down deeper
							if attribute.Value.Type == "array" && attribute.Value.Items.Value.Type == "object" {
								collectAttributes(attribute.Value.Items.Value, flagName+"-", "array")
								continue
							}

							required := false
							if requiredFlags[flagName] {
								required = true
							}

							//If flag is required and only one value is available, no need to offer it to the user
							if required && attribute.Value.Enum != nil && len(attribute.Value.Enum) == 1 {
								continue
							}

							flagName = shortenName(flagName)

							flags[flagName] = Parameter{
								location: "body",
								required: required,
								enum:     attribute.Value.Enum,
								value:    new(string),
							}

							subFlag.StringVar(flags[flagName].value, flagName, "", attribute.Value.Description)

						}

					}

					collectAttributes(action.RequestBody.Value.Content[contentType].Schema.Value, "", "")

				}

			}

			//Find command possition in args
			pos := 1
			for index, arg := range os.Args {
				if arg == command {
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
				if (stat.Mode() & os.ModeCharDevice) == 0 {

					if len(missing) > 0 {
						fmt.Printf("Missing required flag(s): %s\n", missing)
						os.Exit(1)
					}

					var stdin []byte
					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						stdin = append(stdin, scanner.Bytes()...)
					}
					err := scanner.Err()
					checkErr(err)

					body = string(stdin)

				} else {

					if len(missingBody) > 0 || len(missing) > 0 {
						fmt.Printf("Missing required flag(s): %s\n", append(missing, missingBody...))
						os.Exit(1)
					}

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
							if attribute.Value.Type == "object" {
								collectAttributes(attribute.Value, path+".")
								continue
							}

							//Special case for arrays of objects used in relationships
							if attribute.Value.Type == "array" && attribute.Value.Items.Value.Type == "object" {
								path = path + ".id"
								attribute.Value.Type = "relationship"
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

							switch attribute.Value.Type {
							case "relationship":
								//Special case for arrays in relationships
								for _, item := range strings.Split(value, ",") {
									sub := gabs.New()
									sub.Set(item, "id")
									sub.Set(attribute.Value.Items.Value.Properties["type"].Value.Enum[0], "type")

									raw.ArrayAppendP(sub.Data(), strings.TrimSuffix(path, ".id"))
								}

							case "boolean":
								val, _ := strconv.ParseBool(value)
								raw.SetP(val, path)

							case "string":
								raw.SetP(value, path)

							case "integer":
								val, _ := strconv.Atoi(value)
								raw.SetP(val, path)

							case "array":
								raw.SetP(strings.Split(value, ","), path)

							default:
								//TODO: If code reaches here, means we need to add support for more field types!
								fmt.Println("IGNORE UNSUPPORTED FIELD", name, attribute.Value.Type)
							}

						}

					}

					collectAttributes(action.RequestBody.Value.Content[contentType].Schema.Value, "")

					body = raw.StringIndent("", "  ")
				}

			} else {
				if len(missing) > 0 {
					fmt.Printf("Missing required flag(s): %s\n", missing)
					os.Exit(1)
				}
			}

			//Make request to the API
			callAPI(method, uri, query, body, contentType, verbose, format)

			return
		}
	}

	//Command not found
	fmt.Printf("\nCommand '%s' not found. Use -help to list available commands.\n\n", command)
	os.Exit(1)
}

//Helper function to shorter flag-names for convenience
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

//Make a request to the Scalr API
func callAPI(method string, uri string, query url.Values, body string, contentType string, verbose bool, format string) {

	output := gabs.New()
	output.Array()

	query.Add("page[size]", "100")

	for page := 1; true; page++ {

		query.Set("page[number]", strconv.Itoa(page))

		if verbose {
			fmt.Println(method, "https://"+ScalrHostname+BasePath+uri+"?"+query.Encode())

			if contentType != "" {
				fmt.Println("Content-Type = " + contentType)
				fmt.Println(body)
			}

		}

		req, err := http.NewRequest(method, "https://"+ScalrHostname+BasePath+uri+"?"+query.Encode(), strings.NewReader(body))
		checkErr(err)

		req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)
		req.Header.Add("Authorization", "Bearer "+ScalrToken)
		req.Header.Add("Prefer", "profile=preview")

		if contentType != "" {
			req.Header.Add("Content-Type", contentType)
		}

		res, err := http.DefaultClient.Do(req)
		checkErr(err)

		resBody, err := ioutil.ReadAll(res.Body)
		checkErr(err)

		if verbose {
			//Show raw server response
			fmt.Println(string(resBody))
		}

		if res.StatusCode >= 300 {
			showError(resBody)
		}

		//Empty response, quit early
		if len(resBody) == 0 {
			return
		}

		//Check if paging is needed
		response, err := gabs.ParseJSON(resBody)
		checkErr(err)

		//If not a JSON:API response, just rend it raw
		if res.Header.Get("content-type") != "application/vnd.api+json" {
			output = response
			break
		}

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
			output.ArrayAppend(data)
		}

		if response.Path("meta.pagination.next-page").Data() == nil {
			break
		}

	}

	//TODO: Add different outputs, such as YAML, CSV and TABLE
	//formatJSON(resBody)
	fmt.Println(output.StringIndent("", "  "))

}

//Parse error response and show it to user
func showError(resBody []byte) {

	jsonParsed, err := gabs.ParseJSON(resBody)
	if err != nil {
		fmt.Println("Server did return a valid JSON response")
	} else {
		fmt.Println(jsonParsed.StringIndent("", "  "))
	}

	os.Exit(1)
}

//Data JSON:API data to make it easier to work with
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
		included.Set(include.Data(), include.Path("type").Data().(string)+"-"+include.Path("id").Data().(string))
	}

	for _, value := range response.Path("data").Children() {

		sub := gabs.New()

		sub.Set(value.Search("attributes").Data())
		sub.SetP(value.Search("id"), "id")
		sub.SetP(value.Search("type"), "type")

		for name, relationship := range value.Search("relationships").ChildrenMap() {

			if relationship.Data() == nil {
				continue
			}

			//Function to support relationship arrays
			//TODO: Should probably move this outside of the loop for performance reason, but will make code less readable
			var connectRelationship = func(rel *gabs.Container) *gabs.Container {

				relId := rel.Path("type").Data().(string) + "-" + rel.Path("id").Data().(string)

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

		output.ArrayAppend(sub)

	}

	return output

}
