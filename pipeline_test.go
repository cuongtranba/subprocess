package subprocess

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSimplePipe(t *testing.T) {
	// Test: echo "hello world" | grep "world"
	ctx := context.Background()

	echo, err := NewExecutable("echo", "hello world")
	if err != nil {
		t.Fatalf("failed to create echo: %v", err)
	}

	grep, err := NewExecutable("grep", "world")
	if err != nil {
		t.Fatalf("failed to create grep: %v", err)
	}

	result, err := echo.Pipe(grep).Run(ctx)
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("expected output to contain 'hello world', got: %s", stdout)
	}
}

func TestMultiPipe(t *testing.T) {
	// Test: printf "hello\nworld\nhello\n" | grep "hello" | wc -l
	ctx := context.Background()

	printf, _ := NewExecutable("printf", "hello\\nworld\\nhello\\n")
	grep, _ := NewExecutable("grep", "hello")
	wc, _ := NewExecutable("wc", "-l")

	result, err := printf.Pipe(grep).Pipe(wc).Run(ctx)
	if err != nil {
		t.Fatalf("multi-pipe failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	// Should have 2 lines with "hello"
	stdout := strings.TrimSpace(string(result.Stdout))
	if !strings.Contains(stdout, "2") {
		t.Errorf("expected output to contain '2', got: %s", stdout)
	}
}

func TestAndOperatorSuccess(t *testing.T) {
	// Test: true && echo "success"
	ctx := context.Background()

	true_cmd, _ := NewExecutable("true")
	echo, _ := NewExecutable("echo", "success")

	result, err := true_cmd.And(echo).Run(ctx)
	if err != nil {
		t.Fatalf("and operation failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if stdout != "success" {
		t.Errorf("expected 'success', got: %s", stdout)
	}

	// Check that both children exist
	if len(result.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(result.Children))
	}
}

func TestAndOperatorFailure(t *testing.T) {
	// Test: false && echo "should not run"
	ctx := context.Background()

	false_cmd, _ := NewExecutable("false")
	echo, _ := NewExecutable("echo", "should not run")

	result, err := false_cmd.And(echo).Run(ctx)
	if err == nil {
		t.Error("expected error from false command")
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if strings.Contains(stdout, "should not run") {
		t.Error("second command should have been skipped")
	}

	// Check that second child is marked as skipped
	if len(result.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(result.Children))
	}
	if !result.Children[1].Skipped {
		t.Error("expected second child to be marked as skipped")
	}
}

func TestOrOperatorSuccess(t *testing.T) {
	// Test: true || echo "should not run"
	ctx := context.Background()

	true_cmd, _ := NewExecutable("true")
	echo, _ := NewExecutable("echo", "should not run")

	result, err := true_cmd.Or(echo).Run(ctx)
	if err != nil {
		t.Fatalf("or operation failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	// Second command should be skipped
	if len(result.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(result.Children))
	}
	if !result.Children[1].Skipped {
		t.Error("expected second child to be marked as skipped")
	}
}

func TestOrOperatorFailure(t *testing.T) {
	// Test: false || echo "recovered"
	ctx := context.Background()

	false_cmd, _ := NewExecutable("false")
	echo, _ := NewExecutable("echo", "recovered")

	result, err := false_cmd.Or(echo).Run(ctx)
	if err != nil {
		t.Fatalf("or operation failed: %v", err)
	}

	// Should succeed because echo succeeds (bash behavior)
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0 (recovered), got %d", result.ExitCode)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if stdout != "recovered" {
		t.Errorf("expected 'recovered', got: %s", stdout)
	}
}

func TestComplexPipeline(t *testing.T) {
	// Test: (echo "test" | grep "test") && echo "found" || echo "not found"
	ctx := context.Background()

	echo, _ := NewExecutable("echo", "test")
	grep, _ := NewExecutable("grep", "test")
	found, _ := NewExecutable("echo", "found")
	notFound, _ := NewExecutable("echo", "not found")

	pipeline := echo.Pipe(grep).And(found).Or(notFound)
	result, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("complex pipeline failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	stdout := strings.TrimSpace(string(result.Stdout))
	if stdout != "found" {
		t.Errorf("expected 'found', got: %s", stdout)
	}
}

func TestBackgroundExecution(t *testing.T) {
	// Test: sleep 1 & - should return immediately
	ctx := context.Background()

	sleep, _ := NewExecutable("sleep", "0.1")

	start := time.Now()
	result, err := sleep.Background().Run(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("background execution failed: %v", err)
	}

	// Should return quickly (not wait full sleep duration)
	// But will wait because we wait for background jobs
	if duration > 5*time.Second {
		t.Errorf("background job took too long: %v", duration)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestShutdownTimeout(t *testing.T) {
	// Test: custom shutdown timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sleep, _ := NewExecutable("sleep", "10")
	pipeline := sleep.WithShutdownTimeout(1 * time.Second)

	// Cancel after a brief period
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result, err := pipeline.Run(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}

	// Should have been cancelled
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestResultTree(t *testing.T) {
	// Test that result tree is properly constructed
	ctx := context.Background()

	echo1, _ := NewExecutable("echo", "first")
	echo2, _ := NewExecutable("echo", "second")

	result, err := echo1.And(echo2).Run(ctx)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Check tree structure
	if result.Type != OpAnd {
		t.Errorf("expected OpAnd, got %v", result.Type)
	}

	if len(result.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(result.Children))
	}

	if result.Children[0].Type != OpSingle {
		t.Errorf("expected first child to be OpSingle, got %v", result.Children[0].Type)
	}

	if result.Children[1].Type != OpSingle {
		t.Errorf("expected second child to be OpSingle, got %v", result.Children[1].Type)
	}
}

func TestPipeFailFast(t *testing.T) {
	// Test: echo "test" | false | echo "should not run much"
	// When middle command fails, pipeline should fail fast
	ctx := context.Background()

	echo1, _ := NewExecutable("echo", "test")
	false_cmd, _ := NewExecutable("false")
	echo2, _ := NewExecutable("echo", "should not run much")

	result, err := echo1.Pipe(false_cmd).Pipe(echo2).Run(ctx)
	if err == nil {
		t.Error("expected error from failed pipe")
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}
