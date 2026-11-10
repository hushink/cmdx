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
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Run executes a single command synchronously and waits for it to exit.
//
// If the command exits with a non-zero status, the returned error is usually
// *exec.ExitError. If execution is canceled by ctx or times out, the returned
// error wraps context.Canceled or context.DeadlineExceeded, which can be
// tested with errors.Is.
func Run(ctx context.Context, name string, opts ...Option) error {
	cfg := newConfig()
	for _, opt := range opts {
		opt.apply(cfg)
	}

	ctx, cancel := withTimeout(ctx, cfg.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, cfg.args...)
	cfg.applyToCmd(cmd)

	cfg.logger.Info("running", "cmd", cmd.String())
	if err := cmd.Run(); err != nil {
		ctxErr := ctx.Err()
		if ctxErr != nil {
			return runContextError(cfg, cmd, ctxErr, err)
		}
		cfg.logger.Error("command failed", "cmd", cmd.String(), "error", err)
		return err
	}

	cfg.logger.Debug("command succeeded", "cmd", cmd.String())
	return nil
}

// Start starts a command asynchronously and returns a Process that can be
// used to wait for the process to finish.
//
// The returned Process should be waited on with Wait once it is no longer
// needed so that process resources are released. To terminate the process
// early, cancel the provided ctx.
func Start(ctx context.Context, name string, opts ...Option) (Process, error) {
	cfg := newConfig()
	for _, opt := range opts {
		opt.apply(cfg)
	}

	ctx, cancel := withTimeout(ctx, cfg.timeout)

	cmd := exec.CommandContext(ctx, name, cfg.args...)
	cfg.applyToCmd(cmd)

	cfg.logger.Info("starting", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	proc := newProcess(ctx, cancel, cmd, cfg.logger)
	proc.monitor()
	return proc, nil
}

// Process represents an asynchronous process started by Start.
type Process interface {
	// Wait waits for the process to exit and returns its exit error.
	//
	// If the process exits with a non-zero status, the returned error is
	// usually *exec.ExitError. If the process is terminated because the
	// context was canceled or timed out, the returned error wraps
	// context.Canceled or context.DeadlineExceeded.
	//
	// Wait is safe to call multiple times and returns the same result each
	// time.
	Wait() error
}

type process struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
	logger Logger

	done     chan struct{}
	err      error
	ctxErr   error
	waitOnce sync.Once
	waitErr  error
}

func newProcess(ctx context.Context, cancel context.CancelFunc, cmd *exec.Cmd, logger Logger) *process {
	return &process{
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
		logger: logger,
		done:   make(chan struct{}),
	}
}

func (p *process) Wait() error {
	p.waitOnce.Do(func() {
		<-p.done

		// ctxErr is captured after cmd.Wait returns and before the timeout
		// timer is released, so the cancellation triggered by releasing the
		// timer is not mistaken for execution being canceled.
		if p.err != nil && p.ctxErr != nil {
			p.waitErr = errors.Join(p.ctxErr, p.err)
			return
		}
		p.waitErr = p.err
	})
	return p.waitErr
}

func (p *process) monitor() {
	go func() {
		err := p.cmd.Wait()
		ctxErr := p.ctx.Err()
		p.err = err
		p.ctxErr = ctxErr

		if err != nil {
			p.logger.Error("process exited with error", "cmd", p.cmd.String(), "error", err)
		} else {
			p.logger.Debug("process exited", "cmd", p.cmd.String())
		}

		close(p.done)
		p.cancel() // release resources such as the timeout timer
	}()
}

// runContextError builds an error chain containing the context error for a
// canceled or timed-out command.
func runContextError(cfg *config, cmd *exec.Cmd, ctxErr, err error) error {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		cfg.logger.Error("command timed out", "cmd", cmd.String(), "timeout", cfg.timeout)
		return fmt.Errorf("command timed out after %s: %w", cfg.timeout, errors.Join(ctxErr, err))
	}

	cfg.logger.Error("command canceled", "cmd", cmd.String())
	return fmt.Errorf("command canceled: %w", errors.Join(ctxErr, err))
}

func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
