// Copyright 2026 hushink
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"sync"
	"time"
)

// Pipeline represents a group of commands connected through standard input
// and output, similar to a shell pipeline:
//
//	cmd1 | cmd2 | cmd3
//
// A Pipeline is assembled directly from os/exec pipes; no shell is invoked, so
// shell syntax and built-ins are not available. Create one with Pipe and
// append commands with Then. Pipeline methods are not safe for concurrent use;
// do not modify a Pipeline while another goroutine is executing it.
type Pipeline struct {
	ctx     context.Context
	logger  Logger
	timeout time.Duration
	mu      sync.Mutex
	cmds    []pipelineCommand
}

type pipelineCommand struct {
	name string
	cfg  *config
}

// Pipe creates a pipeline whose first command is name.
//
// ctx controls the lifetime of the whole pipeline, and WithTimeout controls
// the timeout of the whole pipeline.
func Pipe(ctx context.Context, name string, opts ...Option) *Pipeline {
	cfg := newConfig()
	for _, opt := range opts {
		opt.apply(cfg)
	}

	return &Pipeline{
		ctx:     ctx,
		logger:  cfg.logger,
		timeout: cfg.timeout,
		cmds:    []pipelineCommand{{name: name, cfg: cfg}},
	}
}

// Then appends a command to the end of the pipeline. The standard output of
// the previous command is connected to its standard input.
//
// The provided options can configure arguments, environment, working
// directory, and standard error per command. WithTimeout and WithLogger are
// ignored here; configure them once on Pipe.
//
// A command that is not the last one must not set standard output, otherwise
// execution returns an error.
func (p *Pipeline) Then(name string, opts ...Option) *Pipeline {
	cfg := newConfig()
	for _, opt := range opts {
		opt.apply(cfg)
	}

	p.mu.Lock()
	p.cmds = append(p.cmds, pipelineCommand{name: name, cfg: cfg})
	p.mu.Unlock()
	return p
}

// WithStdout sets the standard output of the last command in the pipeline.
//
// If Output or CombinedOutput is called afterwards, the output is written to
// both w and the capture buffer.
func (p *Pipeline) WithStdout(w io.Writer) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.cmds); n > 0 {
		p.cmds[n-1].cfg.stdout = w
	}
	return p
}

// WithStderr sets the standard error of the last command in the pipeline.
//
// If CombinedOutput is called afterwards, standard error is written to both w
// and the capture buffer.
func (p *Pipeline) WithStderr(w io.Writer) *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.cmds); n > 0 {
		p.cmds[n-1].cfg.stderr = w
	}
	return p
}

// Run executes the pipeline without capturing output.
//
// Any command failure fails the whole pipeline; multiple errors are joined
// with errors.Join. If commands are terminated because the context is
// canceled or times out, the returned error wraps context.Canceled or
// context.DeadlineExceeded.
func (p *Pipeline) Run() error {
	return p.run(nil, nil)
}

// Output executes the pipeline and returns the standard output of the last
// command.
//
// The returned byte slice remains valid after the run completes and can be
// used safely by the caller.
func (p *Pipeline) Output() ([]byte, error) {
	var buf bytes.Buffer
	err := p.run(&buf, nil)
	return buf.Bytes(), err
}

// CombinedOutput executes the pipeline and returns the combined standard
// output and standard error of the last command.
func (p *Pipeline) CombinedOutput() ([]byte, error) {
	var buf bytes.Buffer
	w := &syncWriter{w: &buf}
	err := p.run(w, w)
	return buf.Bytes(), err
}

func (p *Pipeline) run(stdout, stderr io.Writer) error {
	p.mu.Lock()
	specs := slices.Clone(p.cmds)
	baseCtx := p.ctx
	logger := p.logger
	timeout := p.timeout
	p.mu.Unlock()

	if len(specs) == 0 {
		return nil
	}
	if logger == nil {
		logger = defaultLogger
	}
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx, cancel := withTimeout(baseCtx, timeout)
	defer cancel()

	logger.Info("starting pipeline", "commands", len(specs), "timeout", timeout)

	// Build the commands. The standard output of intermediate commands is
	// owned by the pipeline and therefore must not be set in advance.
	cmds := make([]*exec.Cmd, len(specs))
	for i, spec := range specs {
		if i < len(specs)-1 && spec.cfg.stdout != nil {
			return fmt.Errorf("command %d (%q) must not set stdout: it feeds command %d", i, spec.name, i+1)
		}
		cmd := exec.CommandContext(ctx, spec.name, spec.cfg.args...)
		spec.cfg.applyToCmd(cmd)
		cmds[i] = cmd
	}

	// Connect adjacent commands: the stdout of one becomes the stdin of the
	// next.
	pipes := make([]io.Closer, 0, len(cmds)-1)
	for i := 0; i < len(cmds)-1; i++ {
		pipe, err := cmds[i].StdoutPipe()
		if err != nil {
			closeWriteEnds(cmds, i)
			closeAll(pipes)
			return fmt.Errorf("connect %q to %q: %w", specs[i].name, specs[i+1].name, err)
		}
		pipes = append(pipes, pipe)
		cmds[i+1].Stdin = pipe
	}

	// Capture the output of the last command.
	attachCapture(&cmds[len(cmds)-1].Stdout, stdout)
	attachCapture(&cmds[len(cmds)-1].Stderr, stderr)

	// Start commands from last to first so that the write end of each
	// command's input pipe already exists when it starts.
	for i := len(cmds) - 1; i >= 0; i-- {
		if err := cmds[i].Start(); err != nil {
			logger.Error("failed to start pipeline command", "index", i, "cmd", cmds[i].String(), "error", err)
			// Commands that were not started will never run: close their pipe
			// write ends so downstream commands see EOF.
			closeWriteEnds(cmds, i+1)
			cancel()
			// Reap started processes to avoid leaving zombies and unclosed
			// pipes behind.
			for j := len(cmds) - 1; j > i; j-- {
				_ = cmds[j].Wait()
			}
			closeAll(pipes)
			return fmt.Errorf("start command %d (%q): %w", i, specs[i].name, err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(cmds))
	for i, cmd := range cmds {
		wg.Add(1)
		go func(idx int, c *exec.Cmd, name string) {
			defer wg.Done()
			if err := c.Wait(); err != nil {
				logger.Error("pipeline command failed", "index", idx, "cmd", c.String(), "error", err)
				errCh <- fmt.Errorf("command %d (%q): %w", idx, name, err)
				return
			}
			logger.Debug("pipeline command succeeded", "index", idx, "cmd", c.String())
		}(i, cmd, specs[i].name)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		errs = append(errs, ctxErr)
	}
	return errors.Join(errs...)
}

// attachCapture attaches the capture writer to the command's output target.
// If the caller already configured an output target, both are written.
func attachCapture(dst *io.Writer, capture io.Writer) {
	if capture == nil {
		return
	}
	if *dst == nil {
		*dst = capture
		return
	}
	*dst = io.MultiWriter(*dst, capture)
}

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		_ = c.Close()
	}
}

// closeWriteEnds closes the write ends created by StdoutPipe on the first end
// commands.
//
// Only commands that were never started need explicit closing: os/exec closes
// the write ends of started commands, and the stdout of non-last commands is
// constrained to a pipe write end during the build phase.
func closeWriteEnds(cmds []*exec.Cmd, end int) {
	if end > len(cmds)-1 {
		end = len(cmds) - 1
	}
	for i := 0; i < end; i++ {
		if c, ok := cmds[i].Stdout.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

// syncWriter allows multiple goroutines to write to the same writer safely.
// It is used by CombinedOutput when merging stdout and stderr.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}
