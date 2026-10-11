// Tests for the wireless MAC address allow list service.
//
// The helper sectionContainsMACAddress (in mac_whitelist_service.go) is what stops
// the same MAC address from being added twice, and these tests check that.

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

func TestMACWhitelistServiceSkipsDuplicateMAC(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIWirelessOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	addError := macWhitelistService.AddMACWhitelistEntry("AA:BB:CC:DD:EE:FF")
	if addError != nil {
		testingInstance.Fatalf("unexpected add error: %v", addError)
	}

	duplicateCommand := "uci add_list wireless.default_radio0.maclist='AA:BB:CC:DD:EE:FF'"
	if fakeExecutor.containsCommand(duplicateCommand) {
		testingInstance.Errorf("expected the duplicate MAC address not to be added again, executed: %v", fakeExecutor.executedCommands)
	}

	if fakeExecutor.containsCommand("uci commit wireless") == false {
		testingInstance.Errorf("expected the wireless configuration to be committed, executed: %v", fakeExecutor.executedCommands)
	}
}

func TestMACWhitelistServiceListsRealWorldOutput(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIMultipleMACListOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	whitelistEntries, listError := macWhitelistService.ListMACWhitelistEntries()
	if listError != nil {
		testingInstance.Fatalf("unexpected list error: %v", listError)
	}

	if len(whitelistEntries) != 6 {
		testingInstance.Fatalf("expected 6 whitelist entries, received %d: %v", len(whitelistEntries), whitelistEntries)
	}
}

func TestMACWhitelistServiceRemovesFromRealWorldOutput(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show wireless"] = CommandResult{
		StandardOutput: sampleUCIMultipleMACListOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	macWhitelistService := NewMACWhitelistService(uciClient, nil)

	removeError := macWhitelistService.RemoveMACWhitelistEntry("AA:BB:CC:DD:EE:02")
	if removeError != nil {
		testingInstance.Fatalf("unexpected remove error: %v", removeError)
	}

	expectedCommand := "uci del_list wireless.default_radio0.maclist='AA:BB:CC:DD:EE:02'"
	if fakeExecutor.containsCommand(expectedCommand) == false {
		testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
	}
}
