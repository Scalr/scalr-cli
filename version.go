package main

import "fmt"

func runVersion() {
	fmt.Printf("scalr-cli version %s\n", versionCLI)
	fmt.Printf("Build date: %s\n", buildDate)
}
