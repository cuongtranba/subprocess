package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cuongtranba/subprocess"
)

func main() {
	// Get the path to prints.sh (same directory as this main.go)
	execDir, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}
	scriptPath := filepath.Join(filepath.Dir(execDir), "prints.sh")

	// For development, use relative path if executable path doesn't work
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = "./cmd/echo/prints.sh"
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Create and start the process
	process, err := subprocess.NewProcess("/bin/bash", []string{scriptPath})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating process: %v\n", err)
		os.Exit(1)
	}

	runner, err := process.Exec(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting process: %v\n", err)
		os.Exit(1)
	}

	rw := runner.ReaderWriter()

	// Channel to signal completion
	done := make(chan struct{})

	// Goroutine to copy from process stdout to our stdout
	go func() {
		io.Copy(os.Stdout, rw)
		rw.Close()
	}()

	// Goroutine to copy from our stdin to process stdin
	go func() {
		defer rw.Close()
		defer close(done)

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if _, err := fmt.Fprintln(rw, line); err != nil {
				return
			}
		}
	}()

	// Wait for either stdin to close or process to exit or context cancellation
	select {
	case <-done:
		// stdin closed, normal exit
	case <-ctx.Done():
		// Interrupted, stop the process
		runner.Stop()
	}

	// Wait for process to finish
	if err := runner.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Process exited with error: %v\n", err)
		os.Exit(1)
	}
}
