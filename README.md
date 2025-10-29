# subprocess

A simple and elegant Go library for managing subprocesses with bidirectional I/O communication.

## Features

- **Simple API**: Create and manage subprocesses with minimal boilerplate
- **Pipeline Support**: Shell-like operators (`|`, `&&`, `||`, `&`) for composing processes
- **Streaming Pipes**: Memory-efficient real-time data flow between processes
- **Bidirectional I/O**: Read from and write to subprocess stdin/stdout/stderr through a unified interface
- **Context Support**: Full support for `context.Context` for cancellation and timeouts
- **Process Control**: Start, stop, and wait for subprocess completion
- **Graceful Shutdown**: Configurable timeouts for clean process termination
- **Result Trees**: Comprehensive execution traces with all intermediate outputs
- **Well Tested**: Comprehensive test suite with high code coverage

## Installation

```bash
go get github.com/cuongtranba/subprocess
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "io"

    "github.com/cuongtranba/subprocess"
)

func main() {
    // Create a new process
    process, err := subprocess.NewProcess("cat", []string{})
    if err != nil {
        panic(err)
    }

    // Execute the process
    ctx := context.Background()
    runner, err := process.Exec(ctx)
    if err != nil {
        panic(err)
    }

    // Get the reader/writer for I/O
    rw := runner.ReaderWriter()

    // Write to stdin
    fmt.Fprintln(rw, "Hello, subprocess!")
    rw.Close() // Close stdin to signal EOF

    // Read from stdout/stderr
    output, _ := io.ReadAll(rw)
    fmt.Printf("Output: %s\n", output)

    // Wait for process to complete
    if err := runner.Wait(); err != nil {
        panic(err)
    }
}
```

## Pipeline Support

The library supports shell-like pipeline operations with a fluent API. Pipelines use streaming I/O for memory efficiency and provide comprehensive execution results.

### Quick Pipeline Example

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "github.com/cuongtranba/subprocess"
)

func main() {
    ctx := context.Background()

    // Create a pipeline: echo "hello world" | grep "world"
    echo, _ := subprocess.NewExecutable("echo", "hello world")
    grep, _ := subprocess.NewExecutable("grep", "world")

    result, err := echo.Pipe(grep).Run(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Output: %s\n", strings.TrimSpace(string(result.Stdout)))
    fmt.Printf("Exit code: %d\n", result.ExitCode)
}
```

### Pipeline Operators

#### Pipe (`|`)

Connects stdout of one process to stdin of the next:

```go
// ls | grep ".go" | wc -l
ls, _ := subprocess.NewExecutable("ls")
grep, _ := subprocess.NewExecutable("grep", ".go")
wc, _ := subprocess.NewExecutable("wc", "-l")

result, _ := ls.Pipe(grep).Pipe(wc).Run(ctx)
fmt.Printf("Go files: %s\n", result.Stdout)
```

**Behavior:**
- Streaming data flow (memory efficient)
- Fail-fast: if any process fails, entire pipeline fails
- Final output is from the last process in the chain

#### And (`&&`)

Runs next process only if previous succeeds (exit code 0):

```go
// make test && make build
test, _ := subprocess.NewExecutable("make", "test")
build, _ := subprocess.NewExecutable("make", "build")

result, _ := test.And(build).Run(ctx)
// build only runs if test succeeds
```

**Behavior:**
- Sequential execution
- Short-circuit: stops on first failure
- Skipped processes are marked in result tree

#### Or (`||`)

Runs next process only if previous fails (exit code != 0):

```go
// false || echo "recovered"
false_cmd, _ := subprocess.NewExecutable("false")
echo, _ := subprocess.NewExecutable("echo", "recovered")

result, _ := false_cmd.Or(echo).Run(ctx)
// echo runs because false fails
// final exit code is 0 (bash behavior)
```

**Behavior:**
- Matches bash semantics: `||` recovers from failure
- If recovery succeeds, overall result is success
- Original error preserved in result tree

#### Background (`&`)

Runs process in the background:

```go
// long_task & quick_task
longTask, _ := subprocess.NewExecutable("sleep", "10")
quickTask, _ := subprocess.NewExecutable("echo", "done")

result, _ := longTask.Background().And(quickTask).Run(ctx)
// quickTask runs immediately, longTask runs in parallel
// Run() waits for both before returning
```

**Behavior:**
- Non-blocking: foreground processes continue immediately
- Wait-at-end: `Run()` waits for all background jobs
- Errors are collected in `result.BackgroundErrors` (don't affect exit code)

### Complex Pipeline Example

Combine operators for sophisticated workflows:

```go
// (echo "test" | grep "test") && echo "found" || echo "not found"
echo, _ := subprocess.NewExecutable("echo", "test")
grep, _ := subprocess.NewExecutable("grep", "test")
found, _ := subprocess.NewExecutable("echo", "found")
notFound, _ := subprocess.NewExecutable("echo", "not found")

pipeline := echo.Pipe(grep).And(found).Or(notFound)
result, _ := pipeline.Run(ctx)

fmt.Printf("Result: %s\n", result.Stdout) // "found"
```

### Configuration

#### Shutdown Timeout

Set graceful shutdown timeout (default: 5 seconds):

```go
process, _ := subprocess.NewExecutable("long_running_task")

result, _ := process.
    WithShutdownTimeout(10 * time.Second).
    Run(ctx)

// On context cancellation:
// 1. Send SIGTERM
// 2. Wait up to 10 seconds
// 3. Send SIGKILL if still running
```

### Result Structure

Pipeline results use a tree structure to capture all execution details:

```go
type Result struct {
    Type      OperationType  // Single, Pipe, And, Or, Background
    Stdout    []byte         // Captured stdout
    Stderr    []byte         // Captured stderr
    ExitCode  int            // Exit code
    Error     error          // Execution error if any
    Skipped   bool           // True if skipped (in && || chains)
    Children  []*Result      // Child results (nested operations)

    BackgroundErrors []error // Errors from background processes
}
```

Example result tree inspection:

```go
result, _ := echo.Pipe(grep).And(found).Run(ctx)

fmt.Printf("Type: %v\n", result.Type)           // "and"
fmt.Printf("Exit code: %d\n", result.ExitCode)  // 0
fmt.Printf("Children: %d\n", len(result.Children)) // 2

// First child is the pipe operation
pipeResult := result.Children[0]
fmt.Printf("Pipe type: %v\n", pipeResult.Type)  // "pipe"
fmt.Printf("Pipe children: %d\n", len(pipeResult.Children)) // 2 (echo and grep)

// Second child is the "found" command
foundResult := result.Children[1]
fmt.Printf("Found output: %s\n", foundResult.Stdout) // "found"
```

## API Reference

### Creating a Process

```go
process, err := subprocess.NewProcess(command string, args []string)
```

Creates a new `Process` instance with the specified command and arguments.

**Parameters:**
- `command`: The command to execute (e.g., `/bin/bash`, `echo`, `cat`)
- `args`: Slice of arguments to pass to the command

**Returns:**
- `*Process`: A new process instance
- `error`: Error if process creation fails (currently always returns nil)

### Executing a Process

```go
runner, err := process.Exec(ctx context.Context)
```

Starts the process and returns a `ProcessRunner` for managing it.

**Parameters:**
- `ctx`: Context for cancellation and timeout control

**Returns:**
- `*ProcessRunner`: Runner for managing the process
- `error`: Error if process execution fails

### ProcessRunner Methods

#### ReaderWriter()

```go
rw := runner.ReaderWriter()
```

Returns an `io.ReadWriteCloser` for bidirectional communication with the process.

- **Reading**: Reads from both stdout and stderr (combined)
- **Writing**: Writes to stdin
- **Closing**: Closes stdin (signals EOF to the process)

#### Stop()

```go
err := runner.Stop()
```

Kills the running process immediately.

**Returns:**
- `error`: Error if process termination fails

#### Wait()

```go
err := runner.Wait()
```

Blocks until the process completes and returns its exit status.

**Returns:**
- `error`: Error if process exited with non-zero status or was killed

## Usage Examples

### Interactive Command

```go
// Run an interactive bash script
process, _ := subprocess.NewProcess("/bin/bash", []string{"script.sh"})
runner, _ := process.Exec(context.Background())

rw := runner.ReaderWriter()

// Send input
fmt.Fprintln(rw, "user input")

// Read output
buf := make([]byte, 1024)
n, _ := rw.Read(buf)
fmt.Printf("Output: %s\n", buf[:n])

runner.Wait()
```

### Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

process, _ := subprocess.NewProcess("sleep", []string{"10"})
runner, _ := process.Exec(ctx)

// Process will be killed after 5 seconds due to context timeout
<-ctx.Done()
runner.Stop()
runner.Wait()
```

### Capturing Stderr

```go
process, _ := subprocess.NewProcess("sh", []string{"-c", "echo 'error' >&2"})
runner, _ := process.Exec(context.Background())

// Both stdout and stderr are available through ReaderWriter()
output, _ := io.ReadAll(runner.ReaderWriter())
fmt.Printf("Combined output: %s\n", output) // Prints: error

runner.Wait()
```

### Stopping a Long-Running Process

```go
process, _ := subprocess.NewProcess("sleep", []string{"100"})
runner, _ := process.Exec(context.Background())

// Do some work...
time.Sleep(2 * time.Second)

// Stop the process
runner.Stop()

// Wait returns an error because the process was killed
err := runner.Wait()
fmt.Printf("Process killed: %v\n", err)
```

## Example CLI Application

The repository includes a complete example CLI application in `cmd/echo/` that demonstrates:
- Running an interactive bash script
- Bidirectional I/O with a subprocess
- Graceful shutdown with signal handling
- Context cancellation

To run the example:

```bash
go build -o bin/echo ./cmd/echo
./bin/echo
```

## Testing

Run the test suite:

```bash
go test -v
```

Run with coverage:

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

The test suite includes:
- ✅ Process creation and execution
- ✅ Pipeline operators (Pipe, And, Or, Background)
- ✅ Streaming pipe connections
- ✅ Multi-stage pipelines
- ✅ Conditional execution and short-circuiting
- ✅ Fail-fast behavior
- ✅ Result tree structure
- ✅ Bidirectional I/O communication
- ✅ Process termination and cleanup
- ✅ Error handling and validation
- ✅ Context cancellation
- ✅ Graceful shutdown with timeout
- ✅ Stderr capture
- ✅ Multiple writes to stdin
- ✅ Concurrent operations

**Coverage**: Comprehensive test coverage including edge cases and error scenarios.

## Design Principles

- **Simplicity**: Minimal API surface with sensible defaults
- **Composability**: Works seamlessly with standard Go interfaces (`io.Reader`, `io.Writer`, `context.Context`)
- **Safety**: Proper resource cleanup and error handling
- **Testability**: Real behavior testing without mocks (following TDD principles)

## Use Cases

- Building CLI tools that wrap other commands
- Creating complex command pipelines (like shell scripts in Go)
- Process orchestration and workflow automation
- Executing and interacting with shell scripts
- Interactive command-line applications
- Command output processing and filtering
- Parallel task execution with background processes
- Conditional command execution with error recovery

## Requirements

- Go 1.24.3 or higher
- Unix-like operating system (Linux, macOS) for full functionality

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - feel free to use this library in your projects.

## Author

Created with ❤️ for the Go community
