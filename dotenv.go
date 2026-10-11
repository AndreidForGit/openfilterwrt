// Optional support for ".env" files.
//
// A ".env" file is a plain text file with one "KEY=VALUE" pair per line. It is a
// convenient way to keep settings (including secrets) out of the source code and
// out of very long command lines.
//
// Loading the file is optional:
//
//   - If the file does not exist, nothing happens.
//   - Variables that already exist in the real environment always win, so a value
//     exported in the shell (or injected by Docker) is never overwritten by the
//     file.
//
// Everything here uses only the standard library, because the project avoids
// adding dependencies.
//
// Python equivalent: this does roughly what the "python-dotenv" package does, but
// written by hand and only for the simple cases this project needs.

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ParseEnvironmentFile turns the contents of a ".env" file into a map of
// key/value pairs. It understands:
//
//   - empty lines and lines that start with "#" (both ignored),
//   - an optional "export " prefix,
//   - values wrapped in single or double quotes,
//   - empty values such as "ROUTER_PASSWORD=".
//
// A line that is neither empty, nor a comment, nor a valid "KEY=VALUE" assignment
// is reported as an error, with the line number, so mistakes are easy to find.
func ParseEnvironmentFile(fileContents string) (map[string]string, error) {
	environmentValues := make(map[string]string)

	fileLines := strings.Split(fileContents, "\n")
	for lineIndex := 0; lineIndex < len(fileLines); lineIndex++ {
		currentLine := strings.TrimSpace(fileLines[lineIndex])

		if currentLine == "" {
			continue
		}

		if strings.HasPrefix(currentLine, "#") {
			continue
		}

		assignmentLine := strings.TrimPrefix(currentLine, "export ")
		assignmentLine = strings.TrimSpace(assignmentLine)

		equalsIndex := strings.Index(assignmentLine, "=")
		if equalsIndex < 0 {
			lineNumber := lineIndex + 1
			errorMessage := fmt.Sprintf("invalid line %d in the environment file: %q (expected KEY=VALUE)", lineNumber, currentLine)
			invalidLineError := errors.New(errorMessage)
			return nil, invalidLineError
		}

		environmentKey := strings.TrimSpace(assignmentLine[:equalsIndex])
		if environmentKey == "" {
			lineNumber := lineIndex + 1
			errorMessage := fmt.Sprintf("invalid line %d in the environment file: the key cannot be empty", lineNumber)
			emptyKeyError := errors.New(errorMessage)
			return nil, emptyKeyError
		}

		rawValue := strings.TrimSpace(assignmentLine[equalsIndex+1:])
		environmentValues[environmentKey] = unquoteEnvironmentValue(rawValue)
	}

	return environmentValues, nil
}

// unquoteEnvironmentValue removes one pair of surrounding single or double quotes.
// A value without quotes is returned unchanged.
func unquoteEnvironmentValue(rawValue string) string {
	if len(rawValue) < 2 {
		return rawValue
	}

	firstCharacter := rawValue[0]
	lastCharacter := rawValue[len(rawValue)-1]

	if firstCharacter == '"' && lastCharacter == '"' {
		return rawValue[1 : len(rawValue)-1]
	}

	if firstCharacter == '\'' && lastCharacter == '\'' {
		return rawValue[1 : len(rawValue)-1]
	}

	return rawValue
}

// LoadEnvironmentFile reads the file at the given path and returns its values. A
// missing file is not an error: it simply means the feature is not being used.
func LoadEnvironmentFile(environmentFilePath string) (map[string]string, error) {
	fileContents, readError := os.ReadFile(environmentFilePath)
	if readError != nil {
		if os.IsNotExist(readError) {
			return nil, nil
		}
		return nil, readError
	}

	environmentValues, parseError := ParseEnvironmentFile(string(fileContents))
	if parseError != nil {
		return nil, parseError
	}

	return environmentValues, nil
}

// ApplyEnvironmentValues copies the values into the process environment, but only
// for keys that are not set yet. This is what gives the real environment priority
// over the ".env" file.
func ApplyEnvironmentValues(environmentValues map[string]string) {
	for environmentKey, environmentValue := range environmentValues {
		_, keyExists := os.LookupEnv(environmentKey)
		if keyExists {
			continue
		}

		os.Setenv(environmentKey, environmentValue)
	}
}
