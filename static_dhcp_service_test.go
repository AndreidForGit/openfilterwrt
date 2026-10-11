// Tests for the static DHCP service.
//
// They use the fake executor defined in test_helpers_test.go and check the exact
// uci commands that would be sent to the router.

package main

import (
	"errors"
	"testing"
)

func TestStaticDHCPServiceListsHosts(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	staticDHCPHosts, listError := staticDHCPService.ListStaticDHCPHosts()
	if listError != nil {
		testingInstance.Fatalf("unexpected list error: %v", listError)
	}

	if len(staticDHCPHosts) != 2 {
		testingInstance.Fatalf("expected 2 hosts, received %d", len(staticDHCPHosts))
	}

	firstHost := staticDHCPHosts[0]
	if firstHost.IPAddress != "192.168.1.100" {
		testingInstance.Errorf("expected first host IP 192.168.1.100, received %q", firstHost.IPAddress)
	}
}

func TestStaticDHCPServiceCreatesHost(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: "",
		ExitStatus:     0,
	}
	fakeExecutor.commandResults["uci add dhcp host"] = CommandResult{
		StandardOutput: "cfg0a1b2c\n",
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	newHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.100",
	}

	createError := staticDHCPService.CreateStaticDHCPHost(newHost)
	if createError != nil {
		testingInstance.Fatalf("unexpected create error: %v", createError)
	}

	expectedCommands := []string{
		"uci add dhcp host",
		"uci set dhcp.cfg0a1b2c.name='laptop'",
		"uci set dhcp.cfg0a1b2c.mac='AA:BB:CC:DD:EE:FF'",
		"uci set dhcp.cfg0a1b2c.ip='192.168.1.100'",
		"uci commit dhcp",
		"/etc/init.d/dnsmasq restart",
	}

	for commandIndex := 0; commandIndex < len(expectedCommands); commandIndex++ {
		expectedCommand := expectedCommands[commandIndex]
		if fakeExecutor.containsCommand(expectedCommand) == false {
			testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
		}
	}
}

func TestStaticDHCPServiceRejectsDuplicateMAC(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	newHost := StaticDHCPHost{
		Name:       "duplicate",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.200",
	}

	createError := staticDHCPService.CreateStaticDHCPHost(newHost)
	if createError == nil {
		testingInstance.Fatalf("expected duplicate MAC address to be rejected")
	}
}

func TestStaticDHCPServiceRejectsInvalidHost(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	invalidHost := StaticDHCPHost{
		Name:       "bad",
		MACAddress: "not-a-mac",
		IPAddress:  "not-an-ip",
	}

	createError := staticDHCPService.CreateStaticDHCPHost(invalidHost)
	if createError == nil {
		testingInstance.Fatalf("expected invalid host to be rejected")
	}

	var validationError *ValidationError
	validationErrorDetected := errors.As(createError, &validationError)
	if validationErrorDetected == false {
		testingInstance.Errorf("expected a ValidationError, received %T", createError)
	}

	if len(fakeExecutor.executedCommands) != 0 {
		testingInstance.Errorf("expected no commands to be executed, received %v", fakeExecutor.executedCommands)
	}
}

func TestStaticDHCPServiceDeletesHost(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	deleteError := staticDHCPService.DeleteStaticDHCPHost("AA:BB:CC:DD:EE:FF")
	if deleteError != nil {
		testingInstance.Fatalf("unexpected delete error: %v", deleteError)
	}

	if fakeExecutor.containsCommand("uci delete dhcp.@host[0]") == false {
		testingInstance.Errorf("expected section @host[0] to be deleted, executed: %v", fakeExecutor.executedCommands)
	}

	if fakeExecutor.containsCommand("uci commit dhcp") == false {
		testingInstance.Errorf("expected dhcp configuration to be committed, executed: %v", fakeExecutor.executedCommands)
	}
}

func TestStaticDHCPServiceDeleteMissingHost(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: "",
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	deleteError := staticDHCPService.DeleteStaticDHCPHost("AA:BB:CC:DD:EE:FF")
	notFoundDetected := errors.Is(deleteError, StaticDHCPHostNotFoundError)
	if notFoundDetected == false {
		testingInstance.Errorf("expected StaticDHCPHostNotFoundError, received %v", deleteError)
	}
}

func TestStaticDHCPServiceCreatesHostFromCompactMAC(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: "",
		ExitStatus:     0,
	}
	fakeExecutor.commandResults["uci add dhcp host"] = CommandResult{
		StandardOutput: "cfg0a1b2c\n",
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	newHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "aabbccddeeff",
		IPAddress:  "192.168.1.100",
	}

	createError := staticDHCPService.CreateStaticDHCPHost(newHost)
	if createError != nil {
		testingInstance.Fatalf("unexpected create error: %v", createError)
	}

	expectedCommand := "uci set dhcp.cfg0a1b2c.mac='AA:BB:CC:DD:EE:FF'"
	if fakeExecutor.containsCommand(expectedCommand) == false {
		testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
	}
}

func TestStaticDHCPServiceUpdatesNameAndIP(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	updatedHost := StaticDHCPHost{
		Name:       "renamed-laptop",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.200",
	}

	updateError := staticDHCPService.UpdateStaticDHCPHost("AA:BB:CC:DD:EE:FF", updatedHost)
	if updateError != nil {
		testingInstance.Fatalf("unexpected update error: %v", updateError)
	}

	expectedCommands := []string{
		"uci set dhcp.@host[0].name='renamed-laptop'",
		"uci set dhcp.@host[0].mac='AA:BB:CC:DD:EE:FF'",
		"uci set dhcp.@host[0].ip='192.168.1.200'",
		"uci commit dhcp",
		"/etc/init.d/dnsmasq restart",
	}
	for commandIndex := 0; commandIndex < len(expectedCommands); commandIndex++ {
		expectedCommand := expectedCommands[commandIndex]
		if fakeExecutor.containsCommand(expectedCommand) == false {
			testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
		}
	}

	deletedCommand := "uci delete dhcp.@host[0]"
	if fakeExecutor.containsCommand(deletedCommand) {
		testingInstance.Errorf("expected the section not to be recreated when the MAC address did not change, executed: %v", fakeExecutor.executedCommands)
	}
}

func TestStaticDHCPServiceUpdatesMACAddressByRecreatingSection(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}
	fakeExecutor.commandResults["uci add dhcp host"] = CommandResult{
		StandardOutput: "cfg0a1b2c\n",
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	updatedHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "DE:AD:BE:EF:00:01",
		IPAddress:  "192.168.1.100",
	}

	updateError := staticDHCPService.UpdateStaticDHCPHost("AA:BB:CC:DD:EE:FF", updatedHost)
	if updateError != nil {
		testingInstance.Fatalf("unexpected update error: %v", updateError)
	}

	expectedCommands := []string{
		"uci delete dhcp.@host[0]",
		"uci add dhcp host",
		"uci set dhcp.cfg0a1b2c.mac='DE:AD:BE:EF:00:01'",
		"uci commit dhcp",
		"/etc/init.d/dnsmasq restart",
	}
	for commandIndex := 0; commandIndex < len(expectedCommands); commandIndex++ {
		expectedCommand := expectedCommands[commandIndex]
		if fakeExecutor.containsCommand(expectedCommand) == false {
			testingInstance.Errorf("expected command %q to be executed, executed: %v", expectedCommand, fakeExecutor.executedCommands)
		}
	}
}

func TestStaticDHCPServiceUpdateMissingHost(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: "",
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	updatedHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.1.100",
	}

	updateError := staticDHCPService.UpdateStaticDHCPHost("AA:BB:CC:DD:EE:FF", updatedHost)
	if errors.Is(updateError, StaticDHCPHostNotFoundError) == false {
		testingInstance.Errorf("expected StaticDHCPHostNotFoundError, received %v", updateError)
	}
}

func TestStaticDHCPServiceUpdateConflictingMACAddress(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci show dhcp"] = CommandResult{
		StandardOutput: sampleUCIHostOutput,
		ExitStatus:     0,
	}

	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	updatedHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "11:22:33:44:55:66",
		IPAddress:  "192.168.1.100",
	}

	updateError := staticDHCPService.UpdateStaticDHCPHost("AA:BB:CC:DD:EE:FF", updatedHost)
	if errors.Is(updateError, StaticDHCPHostConflictError) == false {
		testingInstance.Errorf("expected StaticDHCPHostConflictError, received %v", updateError)
	}
}

func TestStaticDHCPServiceUpdateRejectsInvalidInput(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	uciClient := NewUCIClient(fakeExecutor)
	staticDHCPService := NewStaticDHCPService(uciClient, "dnsmasq")

	invalidHost := StaticDHCPHost{
		Name:       "laptop",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "not-an-ip",
	}

	updateError := staticDHCPService.UpdateStaticDHCPHost("AA:BB:CC:DD:EE:FF", invalidHost)
	var validationError *ValidationError
	if errors.As(updateError, &validationError) == false {
		testingInstance.Errorf("expected a ValidationError, received %v", updateError)
	}

	if len(fakeExecutor.executedCommands) != 0 {
		testingInstance.Errorf("expected no commands to run, executed: %v", fakeExecutor.executedCommands)
	}
}
