// Tests for the optional ".env" file loader (see dotenv.go).

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvironmentFile(testingInstance *testing.T) {
	fileContents := "# a comment\nexport ROUTER_ADDRESS=192.168.1.1\n\nROUTER_USERNAME='root'\nROUTER_PASSWORD=\"secret with spaces\"\nAPI_BEARER_TOKEN=\n"

	environmentValues, parseError := ParseEnvironmentFile(fileContents)
	if parseError != nil {
		testingInstance.Fatalf("unexpected parse error: %v", parseError)
	}

	expectedValues := map[string]string{
		"ROUTER_ADDRESS":   "192.168.1.1",
		"ROUTER_USERNAME":  "root",
		"ROUTER_PASSWORD":  "secret with spaces",
		"API_BEARER_TOKEN": "",
	}

	for environmentKey, expectedValue := range expectedValues {
		receivedValue, keyExists := environmentValues[environmentKey]
		if keyExists == false {
			testingInstance.Errorf("expected key %q to be present", environmentKey)
			continue
		}
		if receivedValue != expectedValue {
			testingInstance.Errorf("expected %q to be %q, received %q", environmentKey, expectedValue, receivedValue)
		}
	}
}

func TestParseEnvironmentFileRejectsInvalidLine(testingInstance *testing.T) {
	_, parseError := ParseEnvironmentFile("this line has no equals sign\n")
	if parseError == nil {
		testingInstance.Fatalf("expected an error for a line without an equals sign")
	}
}

func TestLoadEnvironmentFileMissingFile(testingInstance *testing.T) {
	missingPath := filepath.Join(testingInstance.TempDir(), "does-not-exist.env")

	environmentValues, loadError := LoadEnvironmentFile(missingPath)
	if loadError != nil {
		testingInstance.Fatalf("expected a missing file to be ignored, received error: %v", loadError)
	}
	if environmentValues != nil {
		testingInstance.Errorf("expected no values for a missing file, received %v", environmentValues)
	}
}

func TestApplyEnvironmentValuesKeepsExistingValue(testingInstance *testing.T) {
	const testEnvironmentKey = "OPENWRT_CONTROLLER_TEST_EXISTING_VALUE"

	testingInstance.Setenv(testEnvironmentKey, "from-the-environment")

	environmentValues := map[string]string{
		testEnvironmentKey: "from-the-file",
	}
	ApplyEnvironmentValues(environmentValues)

	receivedValue := os.Getenv(testEnvironmentKey)
	if receivedValue != "from-the-environment" {
		testingInstance.Errorf("expected the environment to win, received %q", receivedValue)
	}
}

func TestApplyEnvironmentValuesSetsNewValue(testingInstance *testing.T) {
	const testEnvironmentKey = "OPENWRT_CONTROLLER_TEST_NEW_VALUE"

	os.Unsetenv(testEnvironmentKey)

	environmentValues := map[string]string{
		testEnvironmentKey: "from-the-file",
	}
	ApplyEnvironmentValues(environmentValues)

	receivedValue := os.Getenv(testEnvironmentKey)
	if receivedValue != "from-the-file" {
		testingInstance.Errorf("expected the value from the file, received %q", receivedValue)
	}

	os.Unsetenv(testEnvironmentKey)
}
