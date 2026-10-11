// Shared test helpers.
//
// fakeCommandExecutor pretends to be a CommandExecutor: instead of running
// commands on a router, it returns canned answers. The tests inject it into the
// services so they can check which commands would have been sent, without needing
// a real router.
//
// Python equivalent: this is a mock/fake object, like unittest.mock.

package main

type fakeCommandExecutor struct {
	commandResults   map[string]CommandResult
	commandErrors    map[string]error
	executedCommands []string
}

func newFakeCommandExecutor() *fakeCommandExecutor {
	fakeExecutor := &fakeCommandExecutor{
		commandResults:   make(map[string]CommandResult),
		commandErrors:    make(map[string]error),
		executedCommands: make([]string, 0),
	}
	return fakeExecutor
}

func (fakeExecutor *fakeCommandExecutor) ExecuteCommand(command string) (CommandResult, error) {
	fakeExecutor.executedCommands = append(fakeExecutor.executedCommands, command)

	executionError, errorExists := fakeExecutor.commandErrors[command]
	if errorExists {
		emptyResult := CommandResult{}
		return emptyResult, executionError
	}

	commandResult, resultExists := fakeExecutor.commandResults[command]
	if resultExists {
		return commandResult, nil
	}

	defaultResult := CommandResult{ExitStatus: 0}
	return defaultResult, nil
}

func (fakeExecutor *fakeCommandExecutor) containsCommand(command string) bool {
	for commandIndex := 0; commandIndex < len(fakeExecutor.executedCommands); commandIndex++ {
		if fakeExecutor.executedCommands[commandIndex] == command {
			return true
		}
	}
	return false
}
