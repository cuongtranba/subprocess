package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuongtranba/subprocess"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Pipeline Examples ===\n")

	// Example 1: Simple Pipe
	fmt.Println("1. Simple Pipe: echo 'hello world' | grep 'world'")
	echo, _ := subprocess.NewExecutable("echo", "hello world")
	grep, _ := subprocess.NewExecutable("grep", "world")

	result, err := echo.Pipe(grep).Run(ctx)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Output: %s\n", strings.TrimSpace(string(result.Stdout)))
		fmt.Printf("   Exit code: %d\n\n", result.ExitCode)
	}

	// Example 2: Multi-stage Pipe
	fmt.Println("2. Multi-stage Pipe: printf 'foo\\nbar\\nfoo\\n' | grep 'foo' | wc -l")
	printf, _ := subprocess.NewExecutable("printf", "foo\\nbar\\nfoo\\n")
	grepFoo, _ := subprocess.NewExecutable("grep", "foo")
	wc, _ := subprocess.NewExecutable("wc", "-l")

	result, err = printf.Pipe(grepFoo).Pipe(wc).Run(ctx)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Lines matching 'foo': %s\n\n", strings.TrimSpace(string(result.Stdout)))
	}

	// Example 3: And Operator (success case)
	fmt.Println("3. And Operator: true && echo 'success'")
	trueCmd, _ := subprocess.NewExecutable("true")
	echoSuccess, _ := subprocess.NewExecutable("echo", "success")

	result, err = trueCmd.And(echoSuccess).Run(ctx)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Output: %s\n", strings.TrimSpace(string(result.Stdout)))
		fmt.Printf("   Second command ran: yes\n\n")
	}

	// Example 4: And Operator (failure case)
	fmt.Println("4. And Operator: false && echo 'should not run'")
	falseCmd, _ := subprocess.NewExecutable("false")
	echoSkipped, _ := subprocess.NewExecutable("echo", "should not run")

	result, err = falseCmd.And(echoSkipped).Run(ctx)
	if err != nil {
		fmt.Printf("   First command failed (expected)\n")
		fmt.Printf("   Second command skipped: %v\n\n", result.Children[1].Skipped)
	}

	// Example 5: Or Operator (recovery)
	fmt.Println("5. Or Operator: false || echo 'recovered'")
	falseCmd2, _ := subprocess.NewExecutable("false")
	echoRecovered, _ := subprocess.NewExecutable("echo", "recovered")

	result, err = falseCmd2.Or(echoRecovered).Run(ctx)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Output: %s\n", strings.TrimSpace(string(result.Stdout)))
		fmt.Printf("   Exit code: %d (recovered from failure)\n\n", result.ExitCode)
	}

	// Example 6: Complex Pipeline
	fmt.Println("6. Complex: (echo 'test' | grep 'test') && echo 'found' || echo 'not found'")
	echoTest, _ := subprocess.NewExecutable("echo", "test")
	grepTest, _ := subprocess.NewExecutable("grep", "test")
	echoFound, _ := subprocess.NewExecutable("echo", "found")
	echoNotFound, _ := subprocess.NewExecutable("echo", "not found")

	pipeline := echoTest.Pipe(grepTest).And(echoFound).Or(echoNotFound)
	result, _ = pipeline.Run(ctx)
	fmt.Printf("   Output: %s\n", strings.TrimSpace(string(result.Stdout)))
	fmt.Printf("   Result tree depth: %d levels\n\n", countTreeDepth(result))

	// Example 7: Background Execution
	fmt.Println("7. Background: sleep 0.1 & echo 'immediate'")
	sleep, _ := subprocess.NewExecutable("sleep", "0.1")
	echoImmediate, _ := subprocess.NewExecutable("echo", "immediate")

	start := time.Now()
	result, _ = sleep.Background().And(echoImmediate).Run(ctx)
	duration := time.Since(start)

	fmt.Printf("   Output: %s\n", strings.TrimSpace(string(result.Stdout)))
	fmt.Printf("   Completed in: %v (waited for background job)\n\n", duration)

	// Example 8: Graceful Shutdown
	fmt.Println("8. Graceful Shutdown with timeout")
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	longSleep, _ := subprocess.NewExecutable("sleep", "5")
	pipeline = longSleep.WithShutdownTimeout(50 * time.Millisecond)

	start = time.Now()
	result, err = pipeline.Run(ctxWithTimeout)
	duration = time.Since(start)

	if err != nil {
		fmt.Printf("   Cancelled as expected\n")
		fmt.Printf("   Cleanup time: %v (should be < 200ms)\n\n", duration)
	}

	fmt.Println("=== All examples completed ===")
}

func countTreeDepth(r *subprocess.Result) int {
	if r == nil || len(r.Children) == 0 {
		return 1
	}

	maxDepth := 0
	for _, child := range r.Children {
		depth := countTreeDepth(child)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth + 1
}
