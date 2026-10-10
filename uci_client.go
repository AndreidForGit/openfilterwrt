package main

import (
	"errors"
	"fmt"
	"strings"
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

		decodedValue := unquoteUCIValue(valuePart)

		if optionName == "" {
			existingSection.SectionType = decodedValue
			continue
		}

		existingOptionValues, optionExists := existingSection.Options[optionName]
		if optionExists == false {
			existingOptionValues = make([]string, 0)
		}
		existingOptionValues = append(existingOptionValues, decodedValue)
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

func unquoteUCIValue(rawValue string) string {
	trimmedValue := strings.TrimSpace(rawValue)
	if len(trimmedValue) < 2 {
		return trimmedValue
	}

	firstCharacter := trimmedValue[0]
	lastCharacter := trimmedValue[len(trimmedValue)-1]
	if firstCharacter == '\'' && lastCharacter == '\'' {
		innerValue := trimmedValue[1 : len(trimmedValue)-1]
		unescapedValue := strings.ReplaceAll(innerValue, "'\\''", "'")
		return unescapedValue
	}

	return trimmedValue
}
