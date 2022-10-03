package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"

	"github.com/Jeffail/gabs/v2"
	"golang.org/x/term"
)

func runConfigure() {

	conf := gabs.New()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Scalr Hostname [ex: example.scalr.io]: ")
	scanner.Scan()
	conf.Set(scanner.Text(), "hostname")

	fmt.Print("Scalr Token (not echoed!): ")
	bytepw, _ := term.ReadPassword(int(syscall.Stdin))
	conf.Set(string(bytepw), "token")

	home, err := os.UserHomeDir()
	checkErr(err)

	home = home + "/.scalr/"
	config := "scalr.conf"

	if _, err := os.Stat(home); os.IsNotExist(err) {
		os.MkdirAll(home, 0700)
	}

	//Create a empty file
	file, err := os.OpenFile(home+config, os.O_RDWR|os.O_CREATE, 0600)
	checkErr(err)
	defer file.Close()

	fmt.Println("\nConfiguration saved in " + home + config)

	file.WriteString(conf.StringIndent("", "  ") + "\n")
	file.Sync()

}
