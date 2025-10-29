package subprocess

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// Visitor defines the interface for traversing and executing Executables
type Visitor interface {
	VisitProcess(p *ExecutableProcess) (*Result, error)
	VisitPipe(left, right Executable) (*Result, error)
	VisitAnd(left, right Executable) (*Result, error)
	VisitOr(left, right Executable) (*Result, error)
	VisitBackground(exec Executable) (*Result, error)
}

// ExecutionVisitor implements the Visitor interface for executing pipelines
type ExecutionVisitor struct {
	ctx             context.Context
	shutdownTimeout time.Duration
	backgroundJobs  []*BackgroundJob
}

// BackgroundJob tracks a process running in the background
type BackgroundJob struct {
	exec   Executable
	done   chan *Result
	cancel context.CancelFunc
}

// VisitProcess executes a single process
func (v *ExecutionVisitor) VisitProcess(ep *ExecutableProcess) (*Result, error) {
	// Start the process
	runner, err := ep.process.Exec(v.ctx)
	if err != nil {
		return &Result{
			Type:     OpSingle,
			Error:    fmt.Errorf("failed to start process: %w", err),
			ExitCode: -1,
		}, err
	}

	// Read all output from ReaderWriter (stdout+stderr combined)
	output, _ := io.ReadAll(runner.ReaderWriter())

	// Wait for completion
	err = runner.Wait()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &Result{
		Type:     OpSingle,
		Stdout:   output,
		Stderr:   nil, // Combined with stdout in ReaderWriter
		ExitCode: exitCode,
		Error:    err,
	}, err
}

// VisitPipe executes two executables with stdout piped to stdin
func (v *ExecutionVisitor) VisitPipe(left, right Executable) (*Result, error) {
	// Check context before starting
	if err := v.ctx.Err(); err != nil {
		return &Result{
			Type:  OpPipe,
			Error: err,
		}, err
	}

	// Execute left and right with streaming pipe
	leftResult, rightResult, err := v.executePipe(left, right)

	// Build result tree
	result := &Result{
		Type:     OpPipe,
		Children: []*Result{leftResult, rightResult},
	}

	if err != nil {
		result.Error = err
		// Use the exit code from whichever side failed
		if leftResult.Error != nil {
			result.ExitCode = leftResult.ExitCode
			result.Stderr = leftResult.Stderr
		} else {
			result.ExitCode = rightResult.ExitCode
			result.Stderr = rightResult.Stderr
		}
		return result, err
	}

	// Final output is from the right side
	result.Stdout = rightResult.Stdout
	result.Stderr = rightResult.Stderr
	result.ExitCode = rightResult.ExitCode

	return result, nil
}

// VisitAnd executes right only if left succeeds (exit code 0)
func (v *ExecutionVisitor) VisitAnd(left, right Executable) (*Result, error) {
	// Execute left
	leftResult, err := left.Run(v.ctx)

	// Build result structure
	result := &Result{
		Type:     OpAnd,
		Children: []*Result{leftResult},
	}

	// If left failed, skip right
	if err != nil || leftResult.ExitCode != 0 {
		// Add skipped right to children
		rightResult := &Result{
			Type:    OpSingle,
			Skipped: true,
		}
		result.Children = append(result.Children, rightResult)
		result.ExitCode = leftResult.ExitCode
		result.Error = leftResult.Error
		result.Stdout = leftResult.Stdout
		result.Stderr = leftResult.Stderr
		return result, leftResult.Error
	}

	// Left succeeded, execute right
	rightResult, err := right.Run(v.ctx)
	result.Children = append(result.Children, rightResult)

	// Final result is from right
	result.ExitCode = rightResult.ExitCode
	result.Error = rightResult.Error
	result.Stdout = rightResult.Stdout
	result.Stderr = rightResult.Stderr

	return result, err
}

// VisitOr executes right only if left fails (exit code != 0)
// Matches bash behavior: if right succeeds, overall result is success
func (v *ExecutionVisitor) VisitOr(left, right Executable) (*Result, error) {
	// Execute left
	leftResult, err := left.Run(v.ctx)

	// Build result structure
	result := &Result{
		Type:     OpOr,
		Children: []*Result{leftResult},
	}

	// If left succeeded, skip right
	if err == nil && leftResult.ExitCode == 0 {
		// Add skipped right to children
		rightResult := &Result{
			Type:    OpSingle,
			Skipped: true,
		}
		result.Children = append(result.Children, rightResult)
		result.ExitCode = leftResult.ExitCode
		result.Stdout = leftResult.Stdout
		result.Stderr = leftResult.Stderr
		return result, nil
	}

	// Left failed, execute right (bash behavior: || recovers from failure)
	rightResult, rightErr := right.Run(v.ctx)
	result.Children = append(result.Children, rightResult)

	// Final result is from right (bash semantics)
	result.ExitCode = rightResult.ExitCode
	result.Stdout = rightResult.Stdout
	result.Stderr = rightResult.Stderr

	// If right succeeded, overall succeeds (bash behavior)
	if rightErr == nil && rightResult.ExitCode == 0 {
		result.Error = nil
		return result, nil
	}

	// Right also failed
	result.Error = rightResult.Error
	return result, rightErr
}

// VisitBackground starts execution in the background and returns immediately
func (v *ExecutionVisitor) VisitBackground(exec Executable) (*Result, error) {
	// Create a cancellable context for the background job
	bgCtx, cancel := context.WithCancel(context.Background())

	// Create background job
	job := &BackgroundJob{
		exec:   exec,
		done:   make(chan *Result, 1),
		cancel: cancel,
	}

	// Start execution in background
	go func() {
		result, _ := exec.Run(bgCtx)
		job.done <- result
	}()

	// Track this job
	v.backgroundJobs = append(v.backgroundJobs, job)

	// Return immediately with placeholder result
	result := &Result{
		Type:     OpBackground,
		ExitCode: 0, // Background doesn't affect exit code immediately
	}

	return result, nil
}

// WaitForBackground waits for all background jobs and collects their results
func (v *ExecutionVisitor) WaitForBackground(result *Result) {
	if len(v.backgroundJobs) == 0 {
		return
	}

	// Wait for all background jobs
	for _, job := range v.backgroundJobs {
		select {
		case bgResult := <-job.done:
			// Collect background errors (but don't fail overall result)
			if bgResult.Error != nil {
				if result.BackgroundErrors == nil {
					result.BackgroundErrors = make([]error, 0)
				}
				result.BackgroundErrors = append(result.BackgroundErrors, bgResult.Error)
			}
		case <-v.ctx.Done():
			// Context cancelled, try graceful shutdown
			job.cancel()
			// Brief wait for graceful shutdown
			select {
			case <-job.done:
				// Completed gracefully
			case <-time.After(v.shutdownTimeout):
				// Timeout, job may be orphaned (bash behavior)
			}
		}
	}
}

// gracefulShutdown performs downstream-first sequential graceful shutdown
func (v *ExecutionVisitor) gracefulShutdown(cmds []*exec.Cmd) {
	// Shutdown in reverse order (downstream first)
	for i := len(cmds) - 1; i >= 0; i-- {
		cmd := cmds[i]
		if cmd.Process == nil {
			continue
		}

		// Send SIGTERM
		cmd.Process.Signal(syscall.SIGTERM)

		// Wait with timeout
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
			// Exited gracefully
		case <-time.After(v.shutdownTimeout):
			// Timeout: send SIGKILL
			cmd.Process.Signal(syscall.SIGKILL)
			cmd.Wait() // reap zombie
		}
	}
}

// executePipe connects two processes via their ProcessRunner.ReaderWriter()
func (v *ExecutionVisitor) executePipe(left, right Executable) (*Result, *Result, error) {
	// Start left process
	leftRunner, leftResult, err := v.startProcess(left)
	if err != nil {
		return leftResult, &Result{Type: OpSingle, ExitCode: -1}, err
	}

	// Start right process
	rightRunner, rightResult, err := v.startProcess(right)
	if err != nil {
		return leftResult, rightResult, err
	}

	// Connect left's output (stdout+stderr) to right's input (stdin)
	// Copy in a goroutine so both processes can run concurrently
	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(rightRunner.ReaderWriter(), leftRunner.ReaderWriter())
		rightRunner.ReaderWriter().Close() // Close stdin to signal EOF
		copyDone <- err
	}()

	// Read final output from right process
	output, _ := io.ReadAll(rightRunner.ReaderWriter())

	// Wait for copy to complete
	<-copyDone

	// Wait for both processes
	leftErr := leftRunner.Wait()
	rightErr := rightRunner.Wait()

	// Build results
	leftResult = &Result{
		Type:     OpSingle,
		ExitCode: v.getExitCode(leftErr),
		Error:    leftErr,
	}

	rightResult = &Result{
		Type:     OpSingle,
		Stdout:   output,
		ExitCode: v.getExitCode(rightErr),
		Error:    rightErr,
	}

	// Return first error (fail-fast)
	if leftErr != nil {
		return leftResult, rightResult, leftErr
	}
	if rightErr != nil {
		return leftResult, rightResult, rightErr
	}

	return leftResult, rightResult, nil
}

// startProcess starts an Executable and returns its ProcessRunner
func (v *ExecutionVisitor) startProcess(exec Executable) (*ProcessRunner, *Result, error) {
	if ep, ok := exec.(*ExecutableProcess); ok {
		runner, err := ep.process.Exec(v.ctx)
		if err != nil {
			return nil, &Result{
				Type:     OpSingle,
				Error:    err,
				ExitCode: -1,
			}, err
		}
		return runner, nil, nil
	}

	// For nested pipelines, check if it's a Pipe operation
	if p, ok := exec.(*Pipeline); ok {
		if p.operation == OpPipe {
			// Recursively handle nested pipes
			return v.startNestedPipe(p)
		}
	}

	// For other pipeline types (And/Or/Background), execute normally
	result, err := exec.Run(v.ctx)
	return nil, result, err
}

// startNestedPipe handles nested pipe operations recursively
func (v *ExecutionVisitor) startNestedPipe(p *Pipeline) (*ProcessRunner, *Result, error) {
	// For a nested pipe, we need to recursively connect the processes
	// This creates a chain: left | right
	leftRunner, _, err := v.startProcess(p.left)
	if err != nil {
		return nil, &Result{Type: OpPipe, Error: err, ExitCode: -1}, err
	}

	rightRunner, _, err := v.startProcess(p.right)
	if err != nil {
		return nil, &Result{Type: OpPipe, Error: err, ExitCode: -1}, err
	}

	// Connect left to right
	go func() {
		io.Copy(rightRunner.ReaderWriter(), leftRunner.ReaderWriter())
		rightRunner.ReaderWriter().Close()
	}()

	// Return the rightmost runner (final output comes from here)
	return rightRunner, nil, nil
}

// getExitCode extracts exit code from error
func (v *ExecutionVisitor) getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return -1
}
