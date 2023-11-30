package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

// Run scalr binary and capture JSON output
func run_test(t *testing.T, params ...string) (string, *gabs.Container, error) {

	t.Log(strings.Join(params, " "))
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

	hostname, ok := os.LookupEnv("SCALR_HOSTNAME")
	if !ok {
		t.Fatalf("Required environment variable SCALR_HOSTNAME is not set")
	}

	t.Log("Will run tests against host: " + hostname)

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

func Test_Tags(t *testing.T) {

	account_id, _ := os.LookupEnv("SCALR_ACCOUNT")
	name := "test-tag"

	t.Log("List all tags")

	_, output, err := run_test(t, "list-tags")

	if err != nil {
		t.Fatalf(output.String())
	}

	for _, child := range output.Children() {
		if child.Search("name").Data().(string) == name {
			t.Log("Test-tag already exists, delete")
			run_test(t, "delete-tag", "-tag="+child.Search("id").Data().(string))
		}
	}

	t.Log("Create tag")

	_, output, err = run_test(t, "create-tag", "-account-id="+account_id, "-name="+name)

	if err != nil {
		t.Fatalf(output.String())
	}

	tag_id := output.Search("id").Data().(string)

	t.Log("Get tag")

	message, output, err := run_test(t, "get-tag", "-tag="+tag_id)

	if err != nil {
		t.Fatalf(message)
	}

	if output.Search("name").Data().(string) != name {
		t.Fatalf("Failed to get tag")
	}

	t.Log("Update tag")

	message, _, err = run_test(t, "update-tag", "-tag="+tag_id, "-name="+name+"-2")

	if err != nil {
		t.Fatalf(message)
	}

	t.Log("Delete tag")

	message, _, err = run_test(t, "delete-tag", "-tag="+tag_id)

	if err != nil {
		t.Fatalf(message)
	}

	t.Log("Confirm tag deletion")

	_, _, err = run_test(t, "get-tag", "-tag="+tag_id)

	if err == nil {
		t.Fatalf("Tag still exists")
	}

}

func Test_Environment(t *testing.T) {

	//t.Parallel()

	name := "automated-test"

	t.Log("List all environments")

	_, output, err := run_test(t, "list-environments")

	if err != nil {
		t.Fatalf(output.String())
	}

	for _, child := range output.Children() {
		if child.Search("name").Data().(string) == name {
			t.Log("Environment already exists, delete")
			run_test(t, "delete-environment", "-environment="+child.Search("id").Data().(string))
		}
	}

	t.Log("Create environment")

	_, output, err = run_test(t, "create-environment", "-name="+name)

	if err != nil {
		t.Fatalf(output.String())
	}

	env_id := output.Search("id").Data().(string)

	if env_id == "" {
		t.Fatalf("Failed to create environment")
	}

	t.Log("Get environment")

	message, output, err := run_test(t, "get-environment", "-environment="+env_id)

	if err != nil {
		t.Fatalf(message)
	}

	if output.Search("name").Data().(string) != name {
		t.Fatalf("Failed to get environment")
	}

	t.Log("Update environment")

	_, output, _ = run_test(t, "update-environment", "-environment="+env_id, "-name="+name+"-2", "-cost-estimation-enabled=false")

	if output.Search("cost-estimation-enabled").Data() == true {
		t.Fatalf("Failed to update environment")
	}

	//scalr add-environment-tags -environment=env-ud2fd7tkfl3e21g -id=tag-udffivoc6efoimg
	//scalr list-environment-tags -environment=env-ud2fd7tkfl3e21g
	//scalr create-tag -account-id=acc-ud2fd7shes2mt3g -name=test-tag-second
	//scalr replace-environment-tags -environment=env-ud2fd7tkfl3e21g -id=tag-udfgbqj50gtjftg
	//scalr delete-environment-tags -environment=env-ud2fd7tkfl3e21g -id=tag-udfgbqj50gtjftg

	t.Log("Delete environment")

	message, _, err = run_test(t, "delete-environment", "-environment="+env_id)

	if err != nil {
		t.Fatalf(message)
	}

	t.Log("Confirm environment deletion")

	_, _, err = run_test(t, "get-environment", "-environment="+env_id)

	if err == nil {
		t.Fatalf("Environment still exists")
	}

}

func Test_Workspace(t *testing.T) {

	environment_name := "automated-test"
	workspace_name := "automated-test"

	t.Log("Create environment")

	_, output, err := run_test(t, "create-environment", "-name="+environment_name)

	if err != nil {
		t.Fatalf(output.String())
	}

	env_id := output.Search("id").Data().(string)

	if env_id == "" {
		t.Fatalf("Failed to create environment")
	}

	t.Log("Create workspace")

	_, output, err = run_test(t, "create-workspace", "-environment-id="+env_id, "-name="+workspace_name)

	if err != nil {
		t.Fatalf(output.String())
	}

	workspace_id := output.Search("id").Data().(string)

	if env_id == "" {
		t.Fatalf("Failed to create workspace")
	}

	t.Log("Get workspace")

	message, output, err := run_test(t, "get-workspace", "-workspace="+workspace_id)

	if err != nil {
		t.Fatalf(message)
	}

	if output.Search("name").Data().(string) != workspace_name {
		t.Fatalf("Failed to get workspace")
	}

	t.Log("Update workspace")

	_, output, err = run_test(t, "update-workspace", "-workspace="+workspace_id, "-name="+workspace_name+"-2")

	if err != nil {
		t.Fatalf(output.String())
	}

	t.Log("Lock workspace")

	_, output, err = run_test(t, "lock-workspace", "-workspace="+workspace_id)

	if err != nil {
		t.Fatalf(output.String())
	}

	t.Log("Unlock workspace")

	_, output, err = run_test(t, "unlock-workspace", "-workspace="+workspace_id)

	if err != nil {
		t.Fatalf(output.String())
	}

	//resync-workspace
	//set-schedule
	//workspace tags

	t.Log("Delete workspace")

	message, _, err = run_test(t, "delete-workspace", "-workspace="+workspace_id)

	if err != nil {
		t.Fatalf(message)
	}

	t.Log("Confirm workspace deletion")

	_, _, err = run_test(t, "get-workspace", "-workspace="+workspace_id)

	if err == nil {
		t.Fatalf("workspace still exists")
	}

	t.Log("Delete environment")

	message, _, err = run_test(t, "delete-environment", "-environment="+env_id)

	if err != nil {
		t.Fatalf(message)
	}

}
