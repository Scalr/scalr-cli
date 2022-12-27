package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

//Run scalr binary and capture JSON output
func run_test(params ...string) (string, *gabs.Container, error) {

	cmd := exec.Command("./scalr", params...)

	var out bytes.Buffer
	cmd.Stdout = &out

	code := cmd.Run()

	if len(out.Bytes()) == 0 {
		return "", gabs.New(), code
	}

	jsonParsed, err := gabs.ParseJSON(out.Bytes())

	if err != nil {
		return "Server responded with invalid JSON: " + out.String(), gabs.New(), code
	}

	return "", jsonParsed, code
}

func Test_Check(t *testing.T) {

	t.Log("Check required environment variables")

	_, ok := os.LookupEnv("SCALR_TOKEN")
	if !ok {
		t.Fatalf("Required environment variable SCALR_TOKEN is not set")
	}

	_, ok = os.LookupEnv("SCALR_HOSTNAME")
	if !ok {
		t.Fatalf("Required environment variable SCALR_HOSTNAME is not set")
	}

	_, ok = os.LookupEnv("SCALR_ACCOUNT")
	if !ok {
		t.Fatalf("Required environment variable SCALR_ACCOUNT is not set")
	}

}

func Test_Compile(t *testing.T) {

	t.Log("Compile binary")

	cmd := exec.Command("go", "build", "-o", "scalr")
	err := cmd.Run()

	if err != nil {
		t.Fatalf("Failed to compile binary. Is GOLANG installed?")
	}

}

func Test_Version(t *testing.T) {

	t.Log("Try to run the scalr binary")

	cmd := exec.Command("./scalr", "-version")
	err := cmd.Run()

	if err != nil {
		t.Fatalf("Failed to run binary. Did it compile correctly?")
	}

}

func Test_Environment(t *testing.T) {

	//t.Parallel()

	t.Log("Creating environment")

	account_id, _ := os.LookupEnv("SCALR_ACCOUNT")
	name := "automated-test"

	_, output, err := run_test("create-environment", "-account-id="+account_id, "-name="+name)

	if err != nil {
		t.Fatalf(output.String())
	}

	env_id := output.Search("id").Data().(string)

	if env_id == "" {
		t.Fatalf("Failed to create environment")
	}

	t.Log("Get environment")

	message, output, err := run_test("get-environment", "-environment="+env_id)

	if err != nil {
		t.Fatalf(message)
	}

	if output.Search("name").Data().(string) != name {
		t.Fatalf("Failed to get environment")
	}

	t.Log("Update environment")

	_, output, _ = run_test("update-environment", "-environment="+env_id, "-account-id="+account_id, "-name="+name+"-2", "-cost-estimation-enabled=false")

	if output.Search("cost-estimation-enabled").Data() == true {
		t.Fatalf("Failed to update environment")
	}

	t.Log("Delete environment")

	message, _, err = run_test("delete-environment", "-environment="+env_id)

	if err != nil {
		t.Fatalf(message)
	}

	t.Log("Confirm environment deletion")

	_, _, err = run_test("get-environment", "-environment="+env_id)

	if err == nil {
		t.Fatalf("Environment still exists")
	}

}
