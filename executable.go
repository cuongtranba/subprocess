package subprocess

import (
	"context"
	"time"
)

// ExecutableProcess wraps a Process to implement the Executable interface
// This adapter pattern keeps the Process type simple while enabling composition
type ExecutableProcess struct {
	process         *Process
	shutdownTimeout time.Duration
}

// NewExecutable creates an Executable from a Process
func NewExecutable(cmd string, args ...string) (Executable, error) {
	process, err := NewProcess(cmd, args)
	if err != nil {
		return nil, err
	}
	return &ExecutableProcess{
		process:         process,
		shutdownTimeout: 5 * time.Second, // default timeout
	}, nil
}

// Run executes the single process
func (e *ExecutableProcess) Run(ctx context.Context) (*Result, error) {
	// Create a visitor to execute this process
	visitor := &ExecutionVisitor{
		ctx:             ctx,
		shutdownTimeout: e.shutdownTimeout,
		backgroundJobs:  make([]*BackgroundJob, 0),
	}
	return visitor.VisitProcess(e)
}

// Pipe creates a pipeline that pipes output to the next executable
func (e *ExecutableProcess) Pipe(next Executable) Executable {
	return &Pipeline{
		operation:       OpPipe,
		left:            e,
		right:           next,
		shutdownTimeout: e.shutdownTimeout,
	}
}

// And creates a pipeline that runs next only if this succeeds
func (e *ExecutableProcess) And(next Executable) Executable {
	return &Pipeline{
		operation:       OpAnd,
		left:            e,
		right:           next,
		shutdownTimeout: e.shutdownTimeout,
	}
}

// Or creates a pipeline that runs next only if this fails
func (e *ExecutableProcess) Or(next Executable) Executable {
	return &Pipeline{
		operation:       OpOr,
		left:            e,
		right:           next,
		shutdownTimeout: e.shutdownTimeout,
	}
}

// Background creates a pipeline that runs this in the background
func (e *ExecutableProcess) Background() Executable {
	return &Pipeline{
		operation:       OpBackground,
		left:            e,
		right:           nil, // background has no right side
		shutdownTimeout: e.shutdownTimeout,
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout
func (e *ExecutableProcess) WithShutdownTimeout(timeout time.Duration) Executable {
	e.shutdownTimeout = timeout
	return e
}
