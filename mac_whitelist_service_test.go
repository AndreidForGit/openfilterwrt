package main

import (
	"errors"
	"testing"
)

func TestMACWhitelistServiceListsEntries(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	whitelistEntries, listError := macWhitelistService.ListMACWhitelistEntries()
	if listError != nil {
		testingInstance.Fatalf("unexpected list error: %v", listError)
	}

	if len(whitelistEntries) != 2 {
		testingInstance.Fatalf("expected 2 whitelist entries, received %d", len(whitelistEntries))
	}
}

func TestMACWhitelistServiceAddsEntry(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	addError := macWhitelistService.AddMACWhitelistEntry("99:88:77:66:55:44")
	if addError != nil {
		testingInstance.Fatalf("unexpected add error: %v", addError)
	}

	expectedCommands := []string{
		"uci set wireless.default_radio0.macfilter='allow'",
		"uci add_list wireless.default_radio0.maclist='99:88:77:66:55:44'",
		"uci commit wireless",
		"wifi reload",
	}

	for commandIndex := 0; commandIndex < len(expectedCommands); commandIndex++ {
		expectedCommand := expectedCommands[commandIndex]
		if fakeExecutor.containsCommand(expectedCommand) == false {
			testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
		}
	}
}

func TestMACWhitelistServiceRemovesEntry(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	removeError := macWhitelistService.RemoveMACWhitelistEntry("11:22:33:44:55:66")
	if removeError != nil {
		testingInstance.Fatalf("unexpected remove error: %v", removeError)
	}

	expectedCommand := "uci del_list wireless.default_radio0.maclist='11:22:33:44:55:66'"
	if fakeExecutor.containsCommand(expectedCommand) == false {
		testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
	}
}

func TestMACWhitelistServiceRemoveMissingEntry(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	removeError := macWhitelistService.RemoveMACWhitelistEntry("DE:AD:BE:EF:00:01")
	notFoundDetected := errors.Is(removeError, MACWhitelistEntryNotFoundError)
	if notFoundDetected == false {
		testingInstance.Errorf("expected MACWhitelistEntryNotFoundError, received %v", removeError)
	}
}

func TestMACWhitelistServiceAddsCompactMAC(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	addError := macWhitelistService.AddMACWhitelistEntry("998877665544")
	if addError != nil {
		testingInstance.Fatalf("unexpected add error: %v", addError)
	}

	expectedCommand := "uci add_list wireless.default_radio0.maclist='99:88:77:66:55:44'"
	if fakeExecutor.containsCommand(expectedCommand) == false {
		testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
	}
}
