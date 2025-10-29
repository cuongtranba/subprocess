package subprocess

import (
	"context"
	"time"
)

// Pipeline represents a composition of Executables
// It stores the structure using a flexible representation that can be traversed with the Visitor pattern
type Pipeline struct {
	operation       OperationType
	left            Executable
	right           Executable      // nil for Background operation
	shutdownTimeout time.Duration
}

// Run executes the pipeline using the visitor pattern
func (p *Pipeline) Run(ctx context.Context) (*Result, error) {
	visitor := &ExecutionVisitor{
		ctx:             ctx,
		shutdownTimeout: p.shutdownTimeout,
		backgroundJobs:  make([]*BackgroundJob, 0),
	}

	var result *Result
	var err error

	// Use visitor pattern to execute based on operation type
	switch p.operation {
	case OpPipe:
		result, err = visitor.VisitPipe(p.left, p.right)
	case OpAnd:
		result, err = visitor.VisitAnd(p.left, p.right)
	case OpOr:
		result, err = visitor.VisitOr(p.left, p.right)
	case OpBackground:
		result, err = visitor.VisitBackground(p.left)
	default:
		panic("unknown operation type")
	}

	// Wait for any background jobs before returning
	if err == nil {
		visitor.WaitForBackground(result)
	}

	return result, err
}

// Pipe creates a new pipeline that pipes output to the next executable
func (p *Pipeline) Pipe(next Executable) Executable {
	return &Pipeline{
		operation:       OpPipe,
		left:            p,
		right:           next,
		shutdownTimeout: p.shutdownTimeout,
	}
}

// And creates a new pipeline that runs next only if this succeeds
func (p *Pipeline) And(next Executable) Executable {
	return &Pipeline{
		operation:       OpAnd,
		left:            p,
		right:           next,
		shutdownTimeout: p.shutdownTimeout,
	}
}

// Or creates a new pipeline that runs next only if this fails
func (p *Pipeline) Or(next Executable) Executable {
	return &Pipeline{
		operation:       OpOr,
		left:            p,
		right:           next,
		shutdownTimeout: p.shutdownTimeout,
	}
}

// Background creates a pipeline that runs this in the background
func (p *Pipeline) Background() Executable {
	return &Pipeline{
		operation:       OpBackground,
		left:            p,
		right:           nil,
		shutdownTimeout: p.shutdownTimeout,
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout
func (p *Pipeline) WithShutdownTimeout(timeout time.Duration) Executable {
	p.shutdownTimeout = timeout
	return p
}
