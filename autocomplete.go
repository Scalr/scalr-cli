package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func runAutocomplete() {

	filename, _ := os.Executable()
	filename = filepath.Base(filename)

	trimmed := strings.TrimLeft(os.Getenv("COMP_LINE"), filename)
	trimmed = strings.TrimLeft(trimmed, " ")

	flags := strings.Split(trimmed, " ")

	//List basic flags
	autoBasic(flags)

	//Load all available flags and options from OpenAPI
	allFlags := collectFlagsAndOptions()

	prefix := flags[len(flags)-1]

	//List all available commands
	autoCommands(allFlags, flags, prefix)

	//Get current command
	var command string
	for _, flag := range flags {
		if flag[:1] != "-" {
			command = flag
			break
		}
	}

	//List flag-options for specific flag
	autoOptions(allFlags, command, prefix)

	//List flags for specific command
	autoFlags(allFlags, flags, command, prefix)

}

func listComplete(items []string, prefix string) {

	for _, item := range items {
		if !strings.HasPrefix(item, prefix) || item == prefix {
			continue
		}

		fmt.Println(item)
	}

	os.Exit(0)

}

//List basic flags
func autoBasic(flags []string) {

	if flags[0] != "" && flags[0][:1] == "-" && len(flags) == 1 {
		listComplete([]string{"-version ", "-help ", "-verbose ", "-configure ", "-update ", "-autocomplete "}, flags[0])
	}
}

//List all available commands
func autoCommands(allFlags map[string]map[string][]string, flags []string, prefix string) {
	if len(flags) <= 1 || (len(flags) == 2 && flags[0][:1] == "-") {

		var commands []string

		for command := range allFlags {
			commands = append(commands, command+" ")
		}

		listComplete(commands, prefix)
	}
}

//List flags for specific command
func autoFlags(allFlags map[string]map[string][]string, flags []string, command string, prefix string) {

	var params []string

	for item := range allFlags[command] {
		params = append(params, "-"+item+"=")
	}

	listComplete(params, prefix)

}

//List flag-options for specific flag
func autoOptions(allFlags map[string]map[string][]string, command string, prefix string) {
	if strings.Contains(prefix, "=") {

		var params []string

		parts := strings.Split(prefix, "=")
		flag := strings.TrimLeft(parts[0], "-")
		parts2 := strings.Split(parts[1], ",")

		//Collect previous options for lists
		options := strings.Join(parts2[:len(parts2)-1], ",")
		if options != "" {
			options = options + ","
		}

		prefix = options + parts2[len(parts2)-1]

		for _, parameter := range allFlags[command][flag] {
			params = append(params, options+parameter)
		}

		listComplete(params, prefix)
	}
}

//Load all available flags and options from OpenAPI
func collectFlagsAndOptions() map[string]map[string][]string {

	allFlags := make(map[string]map[string][]string)

	doc := loadAPI()

	for _, path := range doc.Paths {
		for _, object := range path.Operations() {

			command := strings.ReplaceAll(object.OperationID, "_", "-")

			allFlags[command] = make(map[string][]string)

			for _, parameter := range object.Parameters {

				//Ignore paging parameters
				if parameter.Value.Name == "page[number]" || parameter.Value.Name == "page[size]" {
					continue
				}

				allFlags[command][renameFlag(parameter.Value.Name)] = []string{}

				//Collect valid flag values
				var enums []any
				if parameter.Value.Schema.Value.Type == "array" &&
					parameter.Value.Schema.Value.Items != nil &&
					parameter.Value.Schema.Value.Items.Value.Enum != nil {
					enums = parameter.Value.Schema.Value.Items.Value.Enum
				}

				if parameter.Value.Schema.Value.Enum != nil {
					enums = parameter.Value.Schema.Value.Enum
				}

				for _, enum := range enums {
					allFlags[command][renameFlag(parameter.Value.Name)] = append(allFlags[command][renameFlag(parameter.Value.Name)], enum.(string))
				}

			}

			if object.RequestBody == nil {
				continue
			}

			//Get contentType of this command
			var contentType string
			for contentType = range object.RequestBody.Value.Content {
			}

			//If no schema is defined for the body, no need to look for futher fields
			if object.RequestBody.Value.Content[contentType].Schema == nil {
				continue
			}

			//Recursively collect all required fields
			requiredFlags := collectRequired(object.RequestBody.Value.Content[contentType].Schema.Value)

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
					}

					allFlags[command][flagName] = []string{}

					//Collect valid flag values
					if attribute.Value.Enum != nil {

						for _, enum := range attribute.Value.Enum {
							allFlags[command][flagName] = append(allFlags[command][flagName], enum.(string))
						}

					}

				}

			}

			collectAttributes(object.RequestBody.Value.Content[contentType].Schema.Value, "", "")

		}
	}

	return allFlags

}

func enableAutocomplete() {

	fname, err := exec.LookPath("scalr")
	if err != nil {
		fmt.Println("Could not find any 'scalr' binary in your $PATH. Please place a scalr binary in your $PATH before activating tab auto-complete.")
		os.Exit(1)
	}

	fname, err = filepath.Abs(fname)
	checkErr(err)

	//Get user home dir
	home, err := os.UserHomeDir()
	checkErr(err)

	//Guess current shell
	shell := filepath.Base(os.Getenv("SHELL"))
	checkErr(err)

	theConfig := ""

	switch shell {
	case "bash":
		//Install auto-complete for bash
		theConfig = autoCompleteBash(home, fname)
	case "zsh":
		//Install auto-complete for zsh
		theConfig = autoCompleteZsh(home, fname)
	default:
		fmt.Println("Could not find any shell that supports the auto-complete feature.")
		os.Exit(1)
	}

	fmt.Println("Auto-complete has been enabled in " + theConfig + "! Please restart your shell to enable it.")
	os.Exit(0)

}

//Install auto-complete for bash
func autoCompleteBash(home string, fname string) string {

	theConfig := home + "/" + ".bashrc"

	f, err := os.OpenFile(theConfig, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0666)
	checkErr(err)
	defer f.Close()

	findLine := regexp.MustCompile("^complete (.*) scalr$")

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		if findLine.MatchString(scanner.Text()) {
			fmt.Println("Looks like auto-complete is already installed in " + theConfig + ". Please restart your shell to enable it.")
			os.Exit(0)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString("complete -o nospace -C " + fname + " scalr\n")
	checkErr(err)

	return theConfig
}

//Install auto-complete for zsh
func autoCompleteZsh(home string, fname string) string {

	theConfig := home + "/" + ".zshrc"

	f, err := os.OpenFile(theConfig, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0666)
	checkErr(err)
	defer f.Close()

	findLine := regexp.MustCompile("^complete (.*) scalr$")

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {

		if findLine.MatchString(scanner.Text()) {
			fmt.Println("Looks like auto-complete is already installed in " + theConfig + ". Please restart your shell to enable it.")
			os.Exit(0)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString("autoload -U +X compinit\n" +
		"compinit\n" +
		"autoload -U +X bashcompinit\n" +
		"bashcompinit\n" +
		"complete -o nospace -C " + fname + " scalr\n")
	checkErr(err)

	return theConfig

}
