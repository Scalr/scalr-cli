package main

import "fmt"

func runVersion() {
	fmt.Printf("scalr-cli version %s\n", versionCLI)
	fmt.Printf("Git commit: %s\n", gitCommit)
	fmt.Printf("Build date: %s\n", buildDate)
}
