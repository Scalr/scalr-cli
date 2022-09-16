package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func printInfo() {
	fmt.Print("\n", "Usage: scalr [OPTION] COMMAND [FLAGS]", "\n\n")

	fmt.Print("  'scalr' is a command-line interface tool that communicates directly with the Scalr API", "\n\n")

	fmt.Print("Examples:", "\n")
	fmt.Print("  $ scalr -help", "\n")
	fmt.Print("  $ scalr -help get-workspaces", "\n")
	fmt.Print("  $ scalr get-foo-bar -flag=value", "\n")
	fmt.Print("  $ scalr -verbose create-foo-bar -flag=value -flag2=value2", "\n")
	fmt.Print("  $ scalr create-foo-bar < json-blob.txt", "\n\n")

	fmt.Print("Environment variables:", "\n")
	fmt.Print("  SCALR_HOSTNAME", "  ", "Scalr Hostname, i.e example.scalr.io", "\n")
	fmt.Print("  SCALR_TOKEN", "     ", "Scalr API Token", "\n\n")

	fmt.Print("Options:", "\n")
	fmt.Print("  -version", "    ", "Shows current version of this binary", "\n")
	fmt.Print("  -help", "       ", "Shows documentation for all (or specified) command(s)", "\n")
	fmt.Print("  -verbose", "    ", "Shows complete request and response communication data", "\n")
	fmt.Print("  -configure", "  ", "Run configuration wizard", "\n\n")
	//fmt.Print("  -format=STRING", "  ", "Specify output format. Options: json (default), table", "\n")

}

//Prints CLI help
func printHelp() {

	//Help for specified command
	if flag.Arg(0) != "" {
		printHelpCommand(flag.Arg(0))
		return
	}

	//Load OpenAPI specification
	doc := loadAPI()

	groups := make(map[string]map[string]string)

	for _, path := range doc.Paths {
		for _, method := range path.Operations() {

			var group string

			json.Unmarshal(method.ExtensionProps.Extensions["x-resource"].(json.RawMessage), &group)

			//Fallback to Tag if x-resource group is missing
			if group == "" {
				group = strings.Title(method.Tags[0])
			}

			//Add a space before each uppercase letter
			group = strings.TrimPrefix(string(regexp.MustCompile(`([A-Z])`).ReplaceAll([]byte(group), []byte(" $1"))), " ")

			//If group does not exist, add to map
			if groups[group] == nil {
				groups[group] = make(map[string]string)
			}

			groups[group][strings.ReplaceAll(method.OperationID, "_", "-")] = method.Summary

		}
	}

	//Create a sorted array with group names
	sortedGroups := make([]string, 0, len(groups))
	for group := range groups {
		sortedGroups = append(sortedGroups, group)
	}
	sort.Strings(sortedGroups)

	for _, group := range sortedGroups {
		fmt.Println("\n" + group + ":")

		//Create a sorted array with commands
		sortedCommands := make([]string, 0, len(groups[group]))
		maxLength := 0
		for command := range groups[group] {
			sortedCommands = append(sortedCommands, command)

			if len(command) <= maxLength {
				continue
			}

			maxLength = len(command)
		}
		sort.Strings(sortedCommands)

		for _, command := range sortedCommands {
			fmt.Println(" ", command, strings.Repeat(" ", maxLength-len(command)), groups[group][command])
		}
	}

}

func printHelpCommand(command string) {

	//Load OpenAPI specification
	doc := loadAPI()

	for _, path := range doc.Paths {
		for _, object := range path.Operations() {

			if command != strings.ReplaceAll(object.OperationID, "_", "-") {
				continue
			}

			flags := make(map[string]Parameter)

			for _, parameter := range object.Parameters {

				//Ignore paging parameters
				if parameter.Value.Name == "page[number]" || parameter.Value.Name == "page[size]" {
					continue
				}

				//Collect valid flag values
				var enum []any
				if parameter.Value.Schema.Value.Items != nil {
					enum = parameter.Value.Schema.Value.Items.Value.Enum
				}

				flags[renameFlag(parameter.Value.Name)] = Parameter{
					varType:     parameter.Value.Schema.Value.Type,
					description: renameFlag(parameter.Value.Description),
					required:    parameter.Value.Required,
					enum:        enum,
				}

			}

			if object.RequestBody == nil {
				fmt.Printf("\nUsage: scalr [OPTION] %s [FLAGS]\n\n", command)
			} else {

				//This command requires a body
				fmt.Printf("\nUsage: scalr [OPTION] %s [FLAGS] [< json-blob.txt]\n\n", command)

				//Get contentType of this command
				var contentType string
				for contentType = range object.RequestBody.Value.Content {
				}

				//Recursively collect all required fields
				requiredFlags := collectRequired(object.RequestBody.Value.Content[contentType].Schema.Value)

				relationshipDesc := make(map[string]string)

				var collectAttributes func(*openapi3.Schema, string, string)

				//Function to support nested objects
				//TODO: Should probably move this outside of the loop for performance reason, but will make code less readable
				collectAttributes = func(nested *openapi3.Schema, prefix string, inheritType string) {

					//Collect all availble attributes for this command
					for name, attribute := range nested.Properties {

						//Special collection of descriptions for relationships
						if name == "relationships" {
							for rel, desc := range attribute.Value.Properties {
								relationshipDesc[rel+"-id"] = desc.Value.Description
							}
						}

						//Ignore read-only attributes in body
						if attribute.Value.ReadOnly {
							continue
						}

						flagName := prefix + name

						//Ignore ID-field that is redundant
						if flagName == "data-id" {
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

						description := attribute.Value.Description

						//If this is an attribute, strip prefix to shorten flag-names
						flagName = strings.TrimPrefix(flagName, "data-attributes-")

						//If this is a relationship, strip prefix and -data- to shorten flag-names
						if strings.HasPrefix(flagName, "data-relationships-") {

							//If this is not the relationship ID field, ignore it
							if !strings.HasSuffix(flagName, "-id") {
								continue
							}

							flagName = strings.TrimPrefix(flagName, "data-relationships-")
							flagName = strings.Replace(flagName, "-data-id", "-id", 1)

							//Fetch description from parent instead
							description = relationshipDesc[flagName]
						}

						theType := attribute.Value.Type
						if inheritType != "" {
							theType = inheritType
						}

						flags[flagName] = Parameter{
							varType:     theType,
							description: description,
							required:    required,
							enum:        attribute.Value.Enum,
						}

					}

				}

				collectAttributes(object.RequestBody.Value.Content[contentType].Schema.Value, "", "")

			}

			var description string
			if object.Description != "" {
				description = object.Description
			} else if object.Summary != "" {
				description = object.Summary
			}

			fmt.Print("  ", strings.ReplaceAll(strings.TrimSpace(description), "\n", "\n  "), "\n")

			if len(flags) > 0 {

				fmt.Print("\nFlags:", "\n")

				//Create a sorted array with flags
				sortedFlags := make([]string, 0, len(flags))
				maxLength := 0
				for flg := range flags {
					sortedFlags = append(sortedFlags, flg)

					completeLength := len(flg + "=" + flags[flg].varType)

					if completeLength <= maxLength {
						continue
					}

					maxLength = completeLength
				}
				sort.Strings(sortedFlags)

				for _, flg := range sortedFlags {

					varType := strings.ToUpper(flags[flg].varType)
					if varType == "ARRAY" {
						varType = "LIST"
					}

					completeColor := "-" + flg + colorBlue + "=" + varType + colorReset
					complete := "-" + flg + "=" + varType

					//TODO: IF DESCRIPTION INCLUDES LINK, CONVERT IT TO A HTTP LINK TO THE DOCS
					description := strings.ReplaceAll(flags[flg].description, "\n", " ")

					if flags[flg].required {
						description = description + colorRed + " [*required]" + colorReset
					}

					fmt.Println(" ", completeColor, strings.Repeat(" ", maxLength-len(complete)+1), description)

					if flags[flg].enum != nil {

						options := make([]string, len(flags[flg].enum))

						for index, value := range flags[flg].enum {
							options[index] = value.(string)
						}

						fmt.Println(colorBlue, strings.Repeat(" ", maxLength+3), "[", strings.Join(options, ", "), "]", colorReset)
					}
				}
			}

			/*
				//Probably better to add a flag to show examples?

				if object.RequestBody != nil && object.RequestBody.Value.Content["application/vnd.api+json"].Examples["default"] != nil {

						fmt.Print("\njson-blob.txt example:", "\n")

						var data map[string]any
						//TODO: Loop through and show ALL examples, not only default!
						err := json.Unmarshal([]byte(object.RequestBody.Value.Content["application/vnd.api+json"].Examples["default"].Value.Value.(string)), &data)
						if err != nil {
							panic(err)
						}

						example, _ := json.MarshalIndent(data, "", "    ")
						exampleIndented := "  " + strings.ReplaceAll(string(example), "\n", "\n  ")

						fmt.Println(exampleIndented)
					}
			*/

			fmt.Println("")

			return
		}
	}

	fmt.Printf("\nCommand '%s' not found. Use -help to list available commands.\n\n", command)

}
