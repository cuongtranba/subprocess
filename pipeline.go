package subprocess

import (
	"context"
	"time"
)

// OperationType represents the type of pipeline operation
type OperationType int

const (
	OpSingle OperationType = iota // Single process execution
	OpPipe                        // | - pipe stdout to stdin
	OpAnd                         // && - run next if previous succeeds
	OpOr                          // || - run next if previous fails
	OpBackground                  // & - run in background
)

// String returns a string representation of the operation type
func (o OperationType) String() string {
	switch o {
	case OpSingle:
		return "single"
	case OpPipe:
		return "pipe"
	case OpAnd:
		return "and"
	case OpOr:
		return "or"
	case OpBackground:
		return "background"
	default:
		return "unknown"
	}
}

// Result represents the result of executing an Executable
// It uses a tree structure to capture all intermediate and final outputs
type Result struct {
	Type     OperationType // Type of operation that produced this result
	Stdout   []byte        // Captured stdout
	Stderr   []byte        // Captured stderr
	ExitCode int           // Exit code of the process/pipeline
	Error    error         // Execution error if any
	Skipped  bool          // True if this process was skipped (in && || chains)
	Children []*Result     // Child results in the execution tree

	// Background-specific errors (non-fatal, don't affect exit code)
	BackgroundErrors []error
}

// Executable is the common interface for Process and Pipeline
// It represents anything that can be executed and composed with operators
type Executable interface {
	// Run executes the Executable and returns the result
	Run(ctx context.Context) (*Result, error)

	// Pipe connects stdout of this Executable to stdin of next
	// Equivalent to: this | next
	Pipe(next Executable) Executable

	// And runs next only if this succeeds (exit code 0)
	// Equivalent to: this && next
	And(next Executable) Executable

	// Or runs next only if this fails (exit code != 0)
	// Equivalent to: this || next
	Or(next Executable) Executable

	// Background runs this Executable in the background
	// Equivalent to: this &
	Background() Executable

	// WithShutdownTimeout sets the timeout for graceful shutdown
	WithShutdownTimeout(timeout time.Duration) Executable
}
