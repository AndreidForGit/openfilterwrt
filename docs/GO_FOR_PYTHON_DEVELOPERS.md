# Go for Python developers

If you already know a little Python, this page gives you the minimum you need to
read this project with confidence. It is not a full Go course; it focuses on the
things you will actually see in this repository.

## The 30 second version

- Go is compiled: you write `.go` files, run `go build`, and get a single
  executable file. In Python you run `python app.py`; here you run the built
  binary.
- Go has static types. You write `name string` instead of just `name`.
- Errors are values, not exceptions. Functions return an `error` and you check it.
- There are no classes. Related data lives in `struct` values, and behaviour lives
  in functions attached to them.
- Go is strict: an unused variable or import is a compile error, not a warning.

## Side by side

| Idea | Python | Go (this project) |
| :--- | :--- | :--- |
| Import | `import os` | `import "os"` |
| Variable | `count = 0` | `count := 0` or `var count int` |
| Function | `def f(x):` | `func f(x int) int {` |
| Group of data | `class` or `dict` | `struct` |
| Methods | inside the class | `func (s *Service) Method()` |
| Null value | `None` | `nil` |
| Error | `raise` / `try` `except` | `return err` / `if err != nil` |
| List | `list` | slice: `[]string` |
| Dictionary | `dict` | map: `map[string]string` |
| Text formatting | f-strings | `fmt.Sprintf("...%s", value)` |
| Booleans | `and`, `or`, `not` | `&&`, `\|\|`, `!` |
| Tests | `assert` with pytest | `func TestSomething(t *testing.T)` |

## Variables and types

You can declare a variable with `:=`, which infers the type from the right side,
or with `var` when you want to be explicit:

```go
hostName := "laptop"      // string, type inferred
var hostCount int          // int, zero value is 0
isAllowed := false         // bool
```

In Python every variable can hold anything. In Go the type is fixed. The "zero
value" of a type is what you get when you do not assign anything: `""` for
strings, `0` for numbers, `false` for booleans, `nil` for pointers, slices and
maps.

## Structs instead of classes

A `struct` groups related values. It is like a Python dataclass without methods:

```go
type StaticDHCPHost struct {
	Name       string `json:"name"`
	MACAddress string `json:"macAddress"`
	IPAddress  string `json:"ipAddress"`
}
```

The parts in backticks are struct tags. They tell the JSON encoder which names to
use in the API, so the Python field `macAddress` maps to `MACAddress` in Go.

Creating one looks like a dictionary literal:

```go
newHost := StaticDHCPHost{
	Name:       "laptop",
	MACAddress: "AA:BB:CC:DD:EE:FF",
	IPAddress:  "192.168.1.100",
}
```

## Methods: functions with a receiver

In Go you attach a function to a type by putting the receiver in parentheses:

```go
func (service *StaticDHCPService) ListStaticDHCPHosts() ([]StaticDHCPHost, error) {
	...
}
```

`service` is like `self` in Python, but you write it yourself. Inside the function
you use `service.uciClient` exactly like `self.uciClient` in Python.

## Pointers, in one paragraph

A `*` means "the address of a value" and `&` means "give me the address of this
value". You can mostly read `*StaticDHCPService` as "a reference to the service"
and `&Something{...}` as "create it and share the same object". Pointers matter
because they let a method change the original value instead of a copy, and because
they avoid copying big values around.

## Errors are values

There is no `try`/`except`. A function that can fail returns an `error` as its last
result, and you check it immediately:

```go
commandResult, commandError := uciClient.runCommand(command)
if commandError != nil {
	return commandError
}
```

Read `if err != nil { return err }` as "if something went wrong, pass the problem
up to whoever called me". The `serviceErrorStatusCode` function in
`http_handlers.go` shows how the HTTP layer turns those errors into status codes.

## Interfaces

An interface describes behaviour, not data. This one says "any type that has an
`ExecuteCommand` method can be used here":

```go
type CommandExecutor interface {
	ExecuteCommand(command string) (CommandResult, error)
}
```

That is like Python duck typing ("if it walks like a duck...") but checked by the
compiler. It is the trick that makes the project testable: the real code uses the
SSH connection, while the tests use `fakeCommandExecutor`, and neither one knows
about the other.

## Slices and maps

A slice is a growable list:

```go
hosts := make([]StaticDHCPHost, 0) // empty list
hosts = append(hosts, oneHost)     // like list.append
numberOfHosts := len(hosts)        // like len(list)
```

A map is a dictionary:

```go
sections := make(map[string]*UCISection) // keys are strings
aSection, exists := sections["default_radio0"]
if exists {
	// use aSection
}
```

Notice the `exists` boolean: reading a missing key does not raise an error, it
returns the zero value and `false`.

## Loops and conditions

Go has only one loop keyword, `for`, and it looks like Python's `for` when you use
`range`:

```go
for index := 0; index < len(hosts); index++ {
	currentHost := hosts[index]
}

for sectionIdentifier, section := range sections {
	// sectionIdentifier is the key, section is the value
}
```

Conditions do not need parentheses, but they always need curly braces:

```go
if validationError != nil {
	return validationError
}
```

## defer: run this when the function ends

`defer` schedules a call for when the function returns. It is the usual way to
release resources, and it is like a `finally` block:

```go
service.mutationLock.Lock()
defer service.mutationLock.Unlock()
```

The lock is always released, no matter where the function returns.

## Concurrency

A goroutine is a very lightweight thread, started with the `go` keyword:

```go
go startHTTPServer(httpServer, serverErrorChannel)
```

Goroutines talk to each other with channels, which are typed queues. In `main.go`
the program starts the server in a goroutine and then waits with `select` until
either a stop signal arrives or the server fails. Python equivalent: threads plus
`queue.Queue`, but much cheaper.

## Testing

A test lives in a file ending in `_test.go` and its name starts with `Test`. The
parameter is the test handle, usually called `t`:

```go
func TestValidateMACAddressAcceptsValidAddresses(testingInstance *testing.T) {
	validationError := ValidateMACAddress("AA:BB:CC:DD:EE:FF")
	if validationError != nil {
		testingInstance.Errorf("expected a valid MAC address, got %v", validationError)
	}
}
```

Run all the tests with:

```bash
go test ./...
```

In this project the tests never talk to a real router. They inject
`fakeCommandExecutor` (see `test_helpers_test.go`) and then check the commands the
service "would have sent". That idea comes straight from `unittest.mock`.

## Common gotchas

- **Unused things are errors.** If you declare a variable or import a package and
  do not use it, the program will not compile.
- **Formatting is automatic.** Run `gofmt -w .` (or let your editor do it). Go
  code always looks the same, no discussions about style.
- **No `while` loop.** Use `for` with a condition: `for keepGoing { ... }`.
- **`:=` only inside functions.** At the top level (outside any function) you must
  use `var`.
- **Copying a struct copies the value.** Use a pointer (`*Type`) when the function
  should change the original.

## Where to go next

- Read `docs/CODE_WALKTHROUGH.md` to move through the files one by one.
- The official tour is a great next step: <https://go.dev/tour/>.
- To look up a function, the standard library docs are at <https://pkg.go.dev/>.



