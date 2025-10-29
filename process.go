package subprocess

import (
	"context"
	"io"
	"os/exec"
)

type Options struct {
	Command string
	Args    []string

	reader io.ReadCloser
	writer io.WriteCloser
}

type Process struct {
	ops *Options
}

type ProcessRunner struct {
	cmd          *exec.Cmd
	readerWriter io.ReadWriteCloser
	doneCh       chan error
}

func (p *ProcessRunner) Stop() error {
	return p.cmd.Process.Kill()
}

func (p *ProcessRunner) Wait() error {
	return <-p.doneCh
}

func (p *ProcessRunner) ReaderWriter() io.ReadWriteCloser {
	return p.readerWriter
}

func NewProcess(cmd string, args []string) (*Process, error) {
	p := &Process{
		ops: &Options{
			Command: cmd,
			Args:    args,
		},
	}
	return p, nil
}

func (p *Process) Exec(ctx context.Context) (*ProcessRunner, error) {
	cmd := exec.CommandContext(ctx, p.ops.Command, p.ops.Args...)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	readerWriter := io.MultiReader(stdoutPipe, stderrPipe)

	rw := struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: readerWriter,
		Writer: stdinPipe,
		Closer: stdinPipe,
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()
	return &ProcessRunner{
		cmd:          cmd,
		doneCh:       doneCh,
		readerWriter: rw,
	}, nil
}

