// Thin client for the "uci" command line tool.
//
// UCI (Unified Configuration Interface) stores the OpenWrt settings in text files
// under /etc/config (for example /etc/config/dhcp and /etc/config/wireless). The
// "uci" command reads and writes those files.
//
// This file does two things:
//
//  1. Runs uci commands (show, set, add_list, del_list, delete, commit) over SSH.
//  2. Parses the text printed by "uci show" into an easy to use structure.
//
// A detail that matters a lot: "uci show" prints list options as several quoted
// values on a single line, for example:
//
//	maclist='AA:BB:CC:DD:EE:FF' '11:22:33:44:55:66'
//
// parseUCIValueTokens is the helper that splits that line into separate values.
//
// Python equivalent: ParseUCIShowOutput is a small text parser, similar to
// reading a file line by line with readlines() and splitting each line.

package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// UCISection represents one configuration section parsed from `uci show` output.
type UCISection struct {
	Identifier  string
	SectionType string
	Options     map[string][]string
}

// UCIClient offers high level UCI operations on top of a CommandExecutor.
type UCIClient struct {
	executor CommandExecutor
}

// NewUCIClient creates a UCIClient backed by the provided CommandExecutor.
func NewUCIClient(executor CommandExecutor) *UCIClient {
	uciClient := &UCIClient{
		executor: executor,
	}
	return uciClient
}

// runCommand executes a command and turns a non-zero exit status into a Go error,
// so the caller only has to check the error. This is how the program learns that
// a uci command failed on the router.
func (uciClient *UCIClient) runCommand(command string) (CommandResult, error) {
	commandResult, executionError := uciClient.executor.ExecuteCommand(command)
	if executionError != nil {
		return commandResult, executionError
	}

	if commandResult.ExitStatus != 0 {
		trimmedStandardError := strings.TrimSpace(commandResult.StandardError)
		errorMessage := fmt.Sprintf("command %q failed with exit status %d: %s", command, commandResult.ExitStatus, trimmedStandardError)
		commandFailureError := errors.New(errorMessage)
		return commandResult, commandFailureError
	}

	return commandResult, nil
}

// ShowConfiguration returns every section of the named configuration.
func (uciClient *UCIClient) ShowConfiguration(configurationName string) (map[string]*UCISection, error) {
	command := fmt.Sprintf("uci show %s", configurationName)
	commandResult, commandError := uciClient.runCommand(command)
	if commandError != nil {
		return nil, commandError
	}

	sections := ParseUCIShowOutput(commandResult.StandardOutput)
	return sections, nil
}

// AddSection creates a new anonymous section and returns its identifier.
func (uciClient *UCIClient) AddSection(configurationName string, sectionType string) (string, error) {
	command := fmt.Sprintf("uci add %s %s", configurationName, sectionType)
	commandResult, commandError := uciClient.runCommand(command)
	if commandError != nil {
		return "", commandError
	}

	sectionIdentifier := strings.TrimSpace(commandResult.StandardOutput)
	return sectionIdentifier, nil
}

// SetOption assigns a single value to an option of a section.
func (uciClient *UCIClient) SetOption(configurationName string, sectionIdentifier string, optionName string, optionValue string) error {
	quotedOptionValue := BuildShellQuotedValue(optionValue)
	command := fmt.Sprintf("uci set %s.%s.%s=%s", configurationName, sectionIdentifier, optionName, quotedOptionValue)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// AddListValue appends a value to a list option of a section.
func (uciClient *UCIClient) AddListValue(configurationName string, sectionIdentifier string, optionName string, optionValue string) error {
	quotedOptionValue := BuildShellQuotedValue(optionValue)
	command := fmt.Sprintf("uci add_list %s.%s.%s=%s", configurationName, sectionIdentifier, optionName, quotedOptionValue)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// DeleteListValue removes a value from a list option of a section.
func (uciClient *UCIClient) DeleteListValue(configurationName string, sectionIdentifier string, optionName string, optionValue string) error {
	quotedOptionValue := BuildShellQuotedValue(optionValue)
	command := fmt.Sprintf("uci del_list %s.%s.%s=%s", configurationName, sectionIdentifier, optionName, quotedOptionValue)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// DeleteSection removes a whole configuration section.
func (uciClient *UCIClient) DeleteSection(configurationName string, sectionIdentifier string) error {
	command := fmt.Sprintf("uci delete %s.%s", configurationName, sectionIdentifier)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// Commit persists the in-memory changes of a configuration to disk.
func (uciClient *UCIClient) Commit(configurationName string) error {
	command := fmt.Sprintf("uci commit %s", configurationName)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// RestartService restarts a service managed by the init system.
func (uciClient *UCIClient) RestartService(serviceName string) error {
	command := fmt.Sprintf("/etc/init.d/%s restart", serviceName)
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// ReloadWireless applies wireless configuration changes.
func (uciClient *UCIClient) ReloadWireless() error {
	command := "wifi reload"
	_, commandError := uciClient.runCommand(command)
	return commandError
}

// ParseUCIShowOutput converts the textual output of `uci show` into a map of
// sections keyed by their identifier.
func ParseUCIShowOutput(showOutput string) map[string]*UCISection {
	sectionMap := make(map[string]*UCISection)

	outputLines := strings.Split(showOutput, "\n")
	for lineIndex := 0; lineIndex < len(outputLines); lineIndex++ {
		currentLine := strings.TrimSpace(outputLines[lineIndex])
		if currentLine == "" {
			continue
		}

		equalsIndex := strings.Index(currentLine, "=")
		if equalsIndex < 0 {
			continue
		}

		keyPart := currentLine[:equalsIndex]
		valuePart := currentLine[equalsIndex+1:]

		firstDotIndex := strings.Index(keyPart, ".")
		if firstDotIndex < 0 {
			continue
		}

		sectionAndOption := keyPart[firstDotIndex+1:]

		sectionIdentifier := sectionAndOption
		optionName := ""
		lastDotIndex := strings.LastIndex(sectionAndOption, ".")
		if lastDotIndex >= 0 {
			sectionIdentifier = sectionAndOption[:lastDotIndex]
			optionName = sectionAndOption[lastDotIndex+1:]
		}

		existingSection, sectionExists := sectionMap[sectionIdentifier]
		if sectionExists == false {
			createdSection := &UCISection{
				Identifier:  sectionIdentifier,
				SectionType: "",
				Options:     make(map[string][]string),
			}
			sectionMap[sectionIdentifier] = createdSection
			existingSection = createdSection
		}

		decodedValues := parseUCIValueTokens(valuePart)

		if optionName == "" {
			if len(decodedValues) > 0 {
				existingSection.SectionType = decodedValues[0]
			}
			continue
		}

		existingOptionValues, optionExists := existingSection.Options[optionName]
		if optionExists == false {
			existingOptionValues = make([]string, 0)
		}
		for valueIndex := 0; valueIndex < len(decodedValues); valueIndex++ {
			existingOptionValues = append(existingOptionValues, decodedValues[valueIndex])
		}
		existingSection.Options[optionName] = existingOptionValues
	}

	return sectionMap
}

// firstOptionValue returns the first stored value of an option, or an empty
// string when the option is absent.
func firstOptionValue(section *UCISection, optionName string) string {
	optionValues, optionExists := section.Options[optionName]
	if optionExists == false {
		return ""
	}

	if len(optionValues) == 0 {
		return ""
	}

	return optionValues[0]
}

// BuildShellQuotedValue wraps a value in single quotes and escapes embedded
// single quotes so it is safe to pass through the remote shell.
func BuildShellQuotedValue(rawValue string) string {
	escapedValue := strings.ReplaceAll(rawValue, "'", "'\\''")
	quotedValue := "'" + escapedValue + "'"
	return quotedValue
}

// parseUCIValueTokens splits a `uci show` value into its individual tokens. UCI
// prints list options as several space separated, single quoted values on a
// single line, for example:
//
//	maclist='AA:BB:CC:DD:EE:FF' '11:22:33:44:55:66'
//
// Single value options are printed as one token. Sequences of the form '\”
// represent a literal single quote inside a quoted value.
func parseUCIValueTokens(rawValue string) []string {
	valueTokens := make([]string, 0)
	valueCharacters := []rune(strings.TrimSpace(rawValue))

	var currentToken strings.Builder
	tokenStarted := false
	position := 0

	for position < len(valueCharacters) {
		currentCharacter := valueCharacters[position]

		if currentCharacter == '\'' {
			tokenStarted = true
			position = position + 1
			for position < len(valueCharacters) {
				quotedCharacter := valueCharacters[position]
				if quotedCharacter == '\'' {
					escapedQuoteDetected := false
					if position+3 < len(valueCharacters) {
						if valueCharacters[position+1] == '\\' {
							if valueCharacters[position+2] == '\'' {
								if valueCharacters[position+3] == '\'' {
									escapedQuoteDetected = true
								}
							}
						}
					}

					if escapedQuoteDetected {
						currentToken.WriteRune('\'')
						position = position + 4
						continue
					}

					position = position + 1
					break
				}

				currentToken.WriteRune(quotedCharacter)
				position = position + 1
			}
			continue
		}

		if unicode.IsSpace(currentCharacter) {
			if tokenStarted {
				valueTokens = append(valueTokens, currentToken.String())
				currentToken.Reset()
				tokenStarted = false
			}
			position = position + 1
			continue
		}

		currentToken.WriteRune(currentCharacter)
		tokenStarted = true
		position = position + 1
	}

	if tokenStarted {
		valueTokens = append(valueTokens, currentToken.String())
	}

	return valueTokens
}
