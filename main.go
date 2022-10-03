package main

import (
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
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
)

const (
	versionCLI = "0.0.0"

	colorReset = "\033[0m"

	colorRed = "\033[31m"
	//colorGreen  = "\033[32m"
	//colorYellow = "\033[33m"
	colorBlue = "\033[34m"
	//colorPurple = "\033[35m"
	//colorCyan   = "\033[36m"
	//colorWhite  = "\033[37m"
)

func main() {

	if len(os.Args[1:]) == 0 {
		printInfo()
		return
	}

	//Disable unwanted built-in flag features
	flag.Usage = func() {}
	flag.Bool("h", false, "")

	help := flag.Bool("help", false, "")
	configure := flag.Bool("configure", false, "")
	verbose := flag.Bool("verbose", false, "")
	version := flag.Bool("version", false, "")
	format := flag.String("format", "json", "")
	update := flag.Bool("update", false, "")

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

	//Load configuration
	if os.Getenv("SCALR_HOSTNAME") == "" || os.Getenv("SCALR_TOKEN") == "" {

		home, err := os.UserHomeDir()
		checkErr((err))

		home = home + "/.scalr/"
		config := "scalr.conf"

		content, err := ioutil.ReadFile(home + config)
		if err != nil {
			fmt.Print("\n", "Not configured! Please run 'scalr -configure' or set environment variables SCALR_HOSTNAME and SCALR_TOKEN", "\n\n")
			return
		}

		jsonParsed, err := gabs.ParseJSON(content)
		checkErr(err)

		ScalrHostname = jsonParsed.Search("hostname").Data().(string)
		ScalrToken = jsonParsed.Search("token").Data().(string)

	} else {
		//Read config from Environment
		ScalrHostname = os.Getenv("SCALR_HOSTNAME")
		ScalrToken = os.Getenv("SCALR_TOKEN")
	}

	if *help {
		printHelp()
		return
	}

	parseCommand(*format, *verbose)

}

//Check for error and panic
func checkErr(e error) {
	if e != nil {
		panic(e)
	}
}

//Loads OpenAPI specification
func loadAPI() *openapi3.T {

	home, err := os.UserHomeDir()
	checkErr(err)

	home = home + "/.scalr/"
	spec := "cache-" + fmt.Sprintf("%x", md5.Sum([]byte(ScalrHostname))) + "-openapi-preview.yml"

	if _, err := os.Stat(home); os.IsNotExist(err) {
		os.MkdirAll(home, 0700)
	}

	if info, err := os.Stat(home + spec); !os.IsNotExist(err) {
		if time.Since(info.ModTime()).Hours() > 24 {
			//Cache is more than 24 hours old, re-Download...
			downloadFile("https://"+ScalrHostname+"/api/iacp/v3/openapi-preview.yml", home+spec)
		}
	} else {
		//Download spec
		downloadFile("https://"+ScalrHostname+"/api/iacp/v3/openapi-preview.yml", home+spec)
	}

	openapi3.SchemaFormatValidationDisabled = true
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	//Prevent loading external example files which makes the CLI too slow
	loader.ReadFromURIFunc = disableExternalFiles(openapi3.ReadFromURIs(openapi3.ReadFromHTTP(http.DefaultClient), openapi3.ReadFromFile))

	doc, err := loader.LoadFromFile(home + spec)

	//api, _ := url.Parse("https://scalr.io/api/iacp/v3/openapi-preview.yml")
	//doc, err := loader.LoadFromURI(api)

	checkErr(err)

	//Validate the specification
	err = doc.Validate(loader.Context)
	checkErr(err)

	return doc
}

func disableExternalFiles(reader openapi3.ReadFromURIFunc) openapi3.ReadFromURIFunc {

	return func(loader *openapi3.Loader, location *url.URL) (buf []byte, err error) {

		//Skip examples
		if strings.Contains(location.Path, "/examples/") {
			return
		}

		return reader(loader, location)
	}
}

//Downloads a file
func downloadFile(URL string, fileName string) {

	client := &http.Client{}

	req, err := http.NewRequest("GET", URL, nil)
	checkErr(err)

	req.Header.Set("User-Agent", "scalr-cli/"+versionCLI)

	resp, err := client.Do(req)
	checkErr(err)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	if resp.StatusCode != 200 {
		panic(errors.New("received non-200 response code from server"))
	}

	//Create a empty file
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	checkErr(err)
	defer file.Close()

	file.WriteString(string(body))
	file.Sync()

}

//Recursively collect all required fields
func collectRequired(root *openapi3.Schema) map[string]bool {

	requiredFields := make(map[string]bool)

	var recursive func(*openapi3.Schema, string)

	//Function to support nested objects
	recursive = func(nested *openapi3.Schema, prefix string) {

		//data should always be considered as required
		if prefix == "" && nested.Properties["data"] != nil {
			recursive(nested.Properties["data"].Value, prefix+"data-")
		}

		//Collect all availble attributes for this command
		for _, name := range nested.Required {

			requiredFields[prefix+name] = true

			//Nested object, needs to drill down deeper
			if nested.Properties[name].Value.Type == "object" {
				recursive(nested.Properties[name].Value, prefix+name+"-")
				continue
			}

			//Nested array of objects, needs to dril down deeper
			if nested.Properties[name].Value.Type == "array" && nested.Properties[name].Value.Items.Value.Type == "object" {
				recursive(nested.Properties[name].Value.Items.Value, prefix+name+"-")
				continue
			}

		}

	}

	recursive(root, "")

	return requiredFields
}
