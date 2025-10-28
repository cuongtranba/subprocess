# subprocess

A simple and elegant Go library for managing subprocesses with bidirectional I/O communication.

## Features

- **Simple API**: Create and manage subprocesses with minimal boilerplate
- **Bidirectional I/O**: Read from and write to subprocess stdin/stdout/stderr through a unified interface
- **Context Support**: Full support for `context.Context` for cancellation and timeouts
- **Process Control**: Start, stop, and wait for subprocess completion
- **Combined Output**: Automatically combines stdout and stderr streams
- **Well Tested**: Comprehensive test suite with 87%+ code coverage

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
- ✅ Bidirectional I/O communication
- ✅ Process termination and cleanup
- ✅ Error handling and validation
- ✅ Context cancellation
- ✅ Stderr capture
- ✅ Multiple writes to stdin
- ✅ Concurrent operations

**Coverage**: 87%+ statement coverage. The uncovered lines are error handling for system-level pipe creation failures (StdinPipe, StdoutPipe, StderrPipe errors), which are extremely rare and difficult to test without mocking system calls.

## Design Principles

- **Simplicity**: Minimal API surface with sensible defaults
- **Composability**: Works seamlessly with standard Go interfaces (`io.Reader`, `io.Writer`, `context.Context`)
- **Safety**: Proper resource cleanup and error handling
- **Testability**: Real behavior testing without mocks (following TDD principles)

## Use Cases

- Building CLI tools that wrap other commands
- Executing and interacting with shell scripts
- Process orchestration and management
- Interactive command-line applications
- Command output processing and filtering

## Requirements

- Go 1.24.3 or higher
- Unix-like operating system (Linux, macOS) for full functionality

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - feel free to use this library in your projects.

## Author

Created with ❤️ for the Go community
