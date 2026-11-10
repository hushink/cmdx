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
	"io"
	"os/exec"
	"slices"
	"time"
)

// Option configures a single command or pipeline.
//
// Create options with WithArgs, WithEnv, WithStdin, WithStdout, WithStderr,
// WithDir, WithTimeout, and WithLogger.
type Option interface {
	apply(*config)
}

// WithLogger sets the logger. Passing nil disables logging.
func WithLogger(l Logger) Option {
	return &loggerOption{logger: l}
}

type loggerOption struct{ logger Logger }

func (o *loggerOption) apply(c *config) {
	if o.logger == nil {
		c.logger = defaultLogger
		return
	}
	c.logger = o.logger
}

// WithTimeout sets the timeout for a single command or the entire pipeline.
//
// A non-positive duration applies no additional timeout; the command is still
// bound by ctx. When the timeout expires, the process is terminated and the
// returned error wraps context.DeadlineExceeded, which can be tested with
// errors.Is.
func WithTimeout(d time.Duration) Option {
	return &timeoutOption{duration: d}
}

type timeoutOption struct{ duration time.Duration }

func (o *timeoutOption) apply(c *config) { c.timeout = o.duration }

// ====================== Configuration ======================

type config struct {
	args    []string
	env     []string
	envSet  bool
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	dir     string
	logger  Logger
	timeout time.Duration
}

func newConfig() *config {
	return &config{
		logger: defaultLogger,
	}
}

// applyToCmd applies the configuration to cmd.
//
// Standard input, output, and error that were not explicitly configured are
// left nil, in which case os/exec connects them to the null device.
func (c *config) applyToCmd(cmd *exec.Cmd) {
	if c.envSet {
		cmd.Env = c.env
		if cmd.Env == nil {
			// A nil Env instructs os/exec to inherit the parent environment;
			// an explicit empty environment requires a non-nil empty slice.
			cmd.Env = []string{}
		}
	}
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	cmd.Dir = c.dir
}

// ====================== Basic options ======================

// WithArgs sets the command arguments (excluding the command name). The
// provided slice is copied.
func WithArgs(args ...string) Option {
	return &argsOption{args: slices.Clone(args)}
}

// WithEnv sets the environment variables for the child process. The provided
// slice is copied.
//
// Without WithEnv, the child process inherits the parent's environment. When
// WithEnv is used, the child uses exactly the provided environment; nil or an
// empty slice means an empty environment.
func WithEnv(env []string) Option {
	return &envOption{env: slices.Clone(env)}
}

// WithStdin sets the command's standard input; nil reads from the null device.
func WithStdin(r io.Reader) Option {
	return &stdinOption{stdin: r}
}

// WithStdout sets the command's standard output; nil discards the output.
func WithStdout(w io.Writer) Option {
	return &stdoutOption{stdout: w}
}

// WithStderr sets the command's standard error; nil discards the output.
func WithStderr(w io.Writer) Option {
	return &stderrOption{stderr: w}
}

// WithDir sets the command's working directory; an empty string means the
// current directory.
func WithDir(dir string) Option {
	return &dirOption{dir: dir}
}

type argsOption struct{ args []string }

func (o *argsOption) apply(c *config) { c.args = o.args }

type envOption struct{ env []string }

func (o *envOption) apply(c *config) {
	c.env = o.env
	c.envSet = true
}

type stdinOption struct{ stdin io.Reader }

func (o *stdinOption) apply(c *config) { c.stdin = o.stdin }

type stdoutOption struct{ stdout io.Writer }

func (o *stdoutOption) apply(c *config) { c.stdout = o.stdout }

type stderrOption struct{ stderr io.Writer }

func (o *stderrOption) apply(c *config) { c.stderr = o.stderr }

type dirOption struct{ dir string }

func (o *dirOption) apply(c *config) { c.dir = o.dir }
