package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/Jeffail/gabs/v2"
)

func runUpdate() {
	response, err := http.Get("https://api.github.com/repos/Scalr/scalr-cli/releases/latest")
	checkErr(err)
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	checkErr(err)

	data, err := gabs.ParseJSON(body)
	checkErr(err)

	latest := strings.TrimLeft(data.Search("tag_name").Data().(string), "v")

	if latest == versionCLI {
		fmt.Printf("Binary is already at latest version, which is %s \n", versionCLI)
		os.Exit(0)
	}

	fmt.Printf("Latest version is %s, which is different from current installed version %s. \n", latest, versionCLI)

	fmt.Printf("Downloading version %s... \n", latest)

	resp, err := http.Get("https://github.com/Scalr/scalr-cli/releases/download/v" + latest + "/scalr-cli_" + latest + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip")
	checkErr(err)
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	checkErr(err)

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	checkErr(err)

	fmt.Println("Replacing current binary with downloaded version... ")

	for _, zipFile := range zipReader.File {

		if zipFile.Name != "scalr" {
			continue
		}

		f, err := zipFile.Open()
		checkErr(err)
		defer f.Close()

		unzippedFileBytes, err := ioutil.ReadAll(f)
		checkErr(err)

		e, err := os.Executable()
		checkErr(err)

		//Delete current binary to make way for the new one
		err = os.Remove(e)
		checkErr(err)

		out, err := os.Create(e)
		checkErr(err)
		defer out.Close()

		_, err = out.Write(unzippedFileBytes)
		checkErr(err)
		out.Sync()

		fmt.Printf("All done! Your binary is now at version %s \n", latest)

	}
}
