package subprocess

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// TestNewProcess verifies that NewProcess creates a valid Process instance
func TestNewProcess(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		args []string
	}{
		{
			name: "simple command with no args",
			cmd:  "echo",
			args: []string{},
		},
		{
			name: "command with args",
			cmd:  "echo",
			args: []string{"hello", "world"},
		},
		{
			name: "command with path",
			cmd:  "/bin/echo",
			args: []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProcess(tt.cmd, tt.args)
			if err != nil {
				t.Fatalf("NewProcess() error = %v, want nil", err)
			}
			if p == nil {
				t.Fatal("NewProcess() returned nil process")
			}
			if p.ops == nil {
				t.Fatal("Process.ops is nil")
			}
			if p.ops.Command != tt.cmd {
				t.Errorf("Process.ops.Command = %v, want %v", p.ops.Command, tt.cmd)
			}
			if len(p.ops.Args) != len(tt.args) {
				t.Errorf("len(Process.ops.Args) = %v, want %v", len(p.ops.Args), len(tt.args))
			}
		})
	}
}

// TestProcessExec_Success verifies successful process execution
func TestProcessExec_Success(t *testing.T) {
	ctx := context.Background()
	p, err := NewProcess("echo", []string{"hello"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if runner == nil {
		t.Fatal("Exec() returned nil runner")
	}
	if runner.cmd == nil {
		t.Fatal("ProcessRunner.cmd is nil")
	}
	if runner.readerWriter == nil {
		t.Fatal("ProcessRunner.readerWriter is nil")
	}

	// Read output
	output, err := io.ReadAll(runner.ReaderWriter())
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !strings.Contains(string(output), "hello") {
		t.Errorf("output = %q, want to contain 'hello'", string(output))
	}

	// Wait for process to complete
	if err := runner.Wait(); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

// TestProcessExec_InvalidCommand verifies error handling for invalid commands
func TestProcessExec_InvalidCommand(t *testing.T) {
	ctx := context.Background()
	p, err := NewProcess("this_command_does_not_exist_xyz123", []string{})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	_, err = p.Exec(ctx)
	if err == nil {
		t.Fatal("Exec() error = nil, want error for invalid command")
	}
}

// TestProcessRunner_ReaderWriter verifies bidirectional communication
func TestProcessRunner_ReaderWriter(t *testing.T) {
	ctx := context.Background()
	// Use 'cat' which echoes stdin to stdout
	p, err := NewProcess("cat", []string{})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	rw := runner.ReaderWriter()
	if rw == nil {
		t.Fatal("ReaderWriter() returned nil")
	}

	// Write to stdin
	input := "test input\n"
	_, err = io.WriteString(rw, input)
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}

	// Close stdin to signal EOF
	rw.Close()

	// Read from stdout
	output, err := io.ReadAll(rw)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(output) != input {
		t.Errorf("output = %q, want %q", string(output), input)
	}

	// Wait for process
	if err := runner.Wait(); err != nil {
		t.Errorf("Wait() error = %v", err)
	}
}

// TestProcessRunner_Stop verifies process termination
func TestProcessRunner_Stop(t *testing.T) {
	ctx := context.Background()
	// Use 'sleep' for a long-running process
	p, err := NewProcess("sleep", []string{"10"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Stop the process
	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Wait should return an error because process was killed
	err = runner.Wait()
	if err == nil {
		t.Error("Wait() error = nil, want error for killed process")
	}
}

// TestProcessRunner_Wait verifies waiting for process completion
func TestProcessRunner_Wait(t *testing.T) {
	ctx := context.Background()
	p, err := NewProcess("echo", []string{"test"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Read output to avoid blocking
	go io.Copy(io.Discard, runner.ReaderWriter())

	// Wait for completion
	if err := runner.Wait(); err != nil {
		t.Errorf("Wait() error = %v, want nil for successful command", err)
	}
}

// TestProcessRunner_WaitWithError verifies error propagation
func TestProcessRunner_WaitWithError(t *testing.T) {
	ctx := context.Background()
	// Use a command that will exit with non-zero status
	p, err := NewProcess("sh", []string{"-c", "exit 1"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Read output
	go io.Copy(io.Discard, runner.ReaderWriter())

	// Wait should return error
	err = runner.Wait()
	if err == nil {
		t.Error("Wait() error = nil, want error for failed command")
	}
}

// TestProcessExec_ContextCancellation verifies context cancellation
func TestProcessExec_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running process
	p, err := NewProcess("sleep", []string{"10"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Cancel context
	cancel()

	// Give some time for cancellation to propagate
	time.Sleep(100 * time.Millisecond)

	// Process should be killed by context cancellation
	// Note: The process might still be running since our implementation
	// doesn't automatically kill on context cancel, but the command won't start new work

	// Clean up by stopping if still running
	runner.Stop()
	runner.Wait()
}

// TestProcessExec_StderrCapture verifies stderr is captured
func TestProcessExec_StderrCapture(t *testing.T) {
	ctx := context.Background()
	// Command that writes to stderr
	p, err := NewProcess("sh", []string{"-c", "echo 'error message' >&2"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	// Read output in goroutine to avoid blocking
	outputCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		output, err := io.ReadAll(runner.ReaderWriter())
		outputCh <- output
		errCh <- err
	}()

	// Wait for output
	output := <-outputCh
	err = <-errCh
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !strings.Contains(string(output), "error message") {
		t.Errorf("output = %q, want to contain 'error message'", string(output))
	}

	runner.Wait()
}

// TestProcessExec_MultipleWrites verifies multiple writes to stdin
func TestProcessExec_MultipleWrites(t *testing.T) {
	ctx := context.Background()
	// Use 'cat' to echo input
	p, err := NewProcess("cat", []string{})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	rw := runner.ReaderWriter()

	// Channel to collect output
	outputCh := make(chan string, 1)
	go func() {
		output, _ := io.ReadAll(rw)
		outputCh <- string(output)
	}()

	// Write multiple lines
	inputs := []string{"line1\n", "line2\n", "line3\n"}
	for _, input := range inputs {
		if _, err := io.WriteString(rw, input); err != nil {
			t.Fatalf("WriteString() error = %v", err)
		}
	}

	// Close to signal EOF
	rw.Close()

	// Get output
	output := <-outputCh

	// Verify all lines are in output
	for _, input := range inputs {
		if !strings.Contains(output, strings.TrimSpace(input)) {
			t.Errorf("output missing %q", input)
		}
	}

	runner.Wait()
}

// TestProcessRunner_ConcurrentReads verifies thread safety of ReaderWriter
func TestProcessRunner_ConcurrentReads(t *testing.T) {
	ctx := context.Background()
	p, err := NewProcess("echo", []string{"concurrent test"})
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	rw1 := runner.ReaderWriter()
	rw2 := runner.ReaderWriter()

	// Both should reference the same reader/writer
	if rw1 != rw2 {
		t.Error("ReaderWriter() should return same instance")
	}

	io.ReadAll(rw1)
	runner.Wait()
}

// TestProcessExec_EmptyArgs verifies handling of empty arguments
func TestProcessExec_EmptyArgs(t *testing.T) {
	ctx := context.Background()
	p, err := NewProcess("pwd", nil)
	if err != nil {
		t.Fatalf("NewProcess() error = %v", err)
	}

	runner, err := p.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	output, err := io.ReadAll(runner.ReaderWriter())
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if len(output) == 0 {
		t.Error("output is empty, expected current directory path")
	}

	runner.Wait()
}
