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
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRun_Success(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t), helperOpt("echo", "hello", "world"), WithStdout(&stdout))
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "hello world\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRun_ExitError(t *testing.T) {
	err := Run(context.Background(), testBinary(t), helperOpt("exit", "3"))
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run() error = %v, want *exec.ExitError", err)
	}
	if got, want := exitErr.ExitCode(), 3; got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}
}

func TestRun_Timeout(t *testing.T) {
	err := Run(context.Background(), testBinary(t),
		helperOpt("sleep", "10s"),
		WithTimeout(100*time.Millisecond),
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRun_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, testBinary(t), helperOpt("echo", "unreachable"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRun_StdinStdout(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("cat"),
		WithStdin(strings.NewReader("hello stdin")),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "hello stdin"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRun_Stderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("both"),
		WithStdout(&stdout),
		WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "out"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "err"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestRun_WithDir(t *testing.T) {
	dir := t.TempDir()

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("getwd"),
		WithDir(dir),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	got, err := os.Stat(strings.TrimSpace(stdout.String()))
	if err != nil {
		t.Fatalf("os.Stat(helper workdir) error = %v", err)
	}
	want, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", dir, err)
	}
	if !os.SameFile(got, want) {
		t.Errorf("workdir = %q, want %q", stdout.String(), dir)
	}
}

func TestRun_InheritsEnvByDefault(t *testing.T) {
	t.Setenv("CMDX_TEST_ENV", "inherited")

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("env", "CMDX_TEST_ENV"),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "inherited"; got != want {
		t.Errorf("env value = %q, want %q", got, want)
	}
}

func TestRun_WithEnvReplacesEnv(t *testing.T) {
	t.Setenv("CMDX_TEST_ENV", "inherited")

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("env", "CMDX_TEST_ENV"),
		WithEnv([]string{"CMDX_TEST_ENV=explicit"}),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "explicit"; got != want {
		t.Errorf("env value = %q, want %q", got, want)
	}
}

func TestRun_WithEmptyEnv(t *testing.T) {
	t.Setenv("CMDX_TEST_ENV", "inherited")

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("env", "CMDX_TEST_ENV"),
		WithEnv([]string{}),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("env value = %q, want empty", got)
	}
}

func TestRun_InvalidCommand(t *testing.T) {
	err := Run(context.Background(), "cmdx-command-that-does-not-exist")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

func TestRun_WithLogger(t *testing.T) {
	logger := &recordingLogger{}
	var stdout bytes.Buffer

	err := Run(context.Background(), testBinary(t),
		helperOpt("echo", "logged"),
		WithLogger(logger),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if got := logger.count("info"); got == 0 {
		t.Error("logger received no Info messages")
	}
	if got := logger.count("debug"); got == 0 {
		t.Error("logger received no Debug messages")
	}
}

func TestRun_WithNilLogger(t *testing.T) {
	err := Run(context.Background(), testBinary(t),
		helperOpt("echo", "no logger"),
		WithLogger(nil),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestStart_Success(t *testing.T) {
	var stdout bytes.Buffer
	proc, err := Start(context.Background(), testBinary(t),
		helperOpt("echo", "async"),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if proc == nil {
		t.Fatal("Start() returned nil process")
	}

	if err := proc.Wait(); err != nil {
		t.Fatalf("Process.Wait() error = %v, want nil", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), "async"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestStart_ExitError(t *testing.T) {
	proc, err := Start(context.Background(), testBinary(t), helperOpt("exit", "5"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	err = proc.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Process.Wait() error = %v, want *exec.ExitError", err)
	}
	if got, want := exitErr.ExitCode(), 5; got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}
}

func TestStart_Timeout(t *testing.T) {
	proc, err := Start(context.Background(), testBinary(t),
		helperOpt("sleep", "10s"),
		WithTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	if err := proc.Wait(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Process.Wait() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStart_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	proc, err := Start(ctx, testBinary(t), helperOpt("sleep", "10s"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	cancel()
	if err := proc.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Process.Wait() error = %v, want context.Canceled", err)
	}
}

func TestStart_WaitIsIdempotent(t *testing.T) {
	proc, err := Start(context.Background(), testBinary(t), helperOpt("echo", "once"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	if err := proc.Wait(); err != nil {
		t.Fatalf("first Wait() error = %v, want nil", err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("second Wait() error = %v, want nil", err)
	}
}

func TestStart_InvalidCommand(t *testing.T) {
	proc, err := Start(context.Background(), "cmdx-command-that-does-not-exist")
	if err == nil {
		if proc != nil {
			_ = proc.Wait()
		}
		t.Fatal("Start() error = nil, want non-nil")
	}
}

// recordingLogger records log levels to verify WithLogger.
type recordingLogger struct {
	mu     sync.Mutex
	levels []string
}

func (l *recordingLogger) record(level string) {
	l.mu.Lock()
	l.levels = append(l.levels, level)
	l.mu.Unlock()
}

func (l *recordingLogger) Debug(string, ...any) { l.record("debug") }
func (l *recordingLogger) Info(string, ...any)  { l.record("info") }
func (l *recordingLogger) Warn(string, ...any)  { l.record("warn") }
func (l *recordingLogger) Error(string, ...any) { l.record("error") }

func (l *recordingLogger) count(level string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := 0
	for _, lv := range l.levels {
		if lv == level {
			n++
		}
	}
	return n
}
