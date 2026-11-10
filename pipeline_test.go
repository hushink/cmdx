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
	"strings"
	"testing"
	"time"
)

func TestPipe_SingleCommandOutput(t *testing.T) {
	out, err := Pipe(context.Background(), testBinary(t), helperOpt("echo", "hello")).Output()
	if err != nil {
		t.Fatalf("Output() error = %v, want nil", err)
	}
	if got, want := string(out), "hello\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestPipe_ThenConnectsStdoutToStdin(t *testing.T) {
	out, err := Pipe(context.Background(), testBinary(t),
		helperOpt("cat"),
		WithStdin(strings.NewReader("hello")),
	).
		Then(testBinary(t), helperOpt("upper")).
		Output()
	if err != nil {
		t.Fatalf("Output() error = %v, want nil", err)
	}
	if got, want := string(out), "HELLO"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestPipe_MultipleStages(t *testing.T) {
	out, err := Pipe(context.Background(), testBinary(t),
		helperOpt("cat"),
		WithStdin(strings.NewReader("foo\nbar\nbaz\n")),
	).
		Then(testBinary(t), helperOpt("upper")).
		Then(testBinary(t), helperOpt("grep", "BAR")).
		Output()
	if err != nil {
		t.Fatalf("Output() error = %v, want nil", err)
	}
	if got, want := string(out), "BAR\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestPipe_CombinedOutput(t *testing.T) {
	out, err := Pipe(context.Background(), testBinary(t), helperOpt("both")).CombinedOutput()
	if err != nil {
		t.Fatalf("CombinedOutput() error = %v, want nil", err)
	}

	got := string(out)
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Errorf("combined output = %q, want it to contain both %q and %q", got, "out", "err")
	}
}

func TestPipe_CombinedOutputWithStdout(t *testing.T) {
	var extra bytes.Buffer
	out, err := Pipe(context.Background(), testBinary(t), helperOpt("both")).
		WithStdout(&extra).
		CombinedOutput()
	if err != nil {
		t.Fatalf("CombinedOutput() error = %v, want nil", err)
	}

	if got, want := extra.String(), "out"; got != want {
		t.Errorf("extra stdout = %q, want %q", got, want)
	}
	got := string(out)
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Errorf("combined output = %q, want it to contain both %q and %q", got, "out", "err")
	}
}

func TestPipe_RunWithStdout(t *testing.T) {
	var stdout bytes.Buffer
	err := Pipe(context.Background(), testBinary(t), helperOpt("echo", "run")).
		WithStdout(&stdout).
		Run()
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "run\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestPipe_RunWithStderr(t *testing.T) {
	var stderr bytes.Buffer
	err := Pipe(context.Background(), testBinary(t), helperOpt("stderr", "boom")).
		WithStderr(&stderr).
		Run()
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stderr.String(), "boom"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestPipe_Timeout(t *testing.T) {
	err := Pipe(context.Background(), testBinary(t),
		helperOpt("sleep", "10s"),
		WithTimeout(100*time.Millisecond),
	).Run()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPipe_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Pipe(ctx, testBinary(t), helperOpt("echo", "unreachable")).Run()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestPipe_InvalidCommand(t *testing.T) {
	err := Pipe(context.Background(), "cmdx-command-that-does-not-exist").Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

func TestPipe_ThenInvalidCommand(t *testing.T) {
	err := Pipe(context.Background(), testBinary(t), helperOpt("cat")).
		Then("cmdx-command-that-does-not-exist").
		Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

func TestPipe_FirstCommandStartFailureCleansUp(t *testing.T) {
	// Commands are started from last to first: the last command starts
	// successfully and then the first command fails to start. This test covers
	// reaping started processes and cleaning up pipes for commands that were
	// never started.
	err := Pipe(context.Background(), "cmdx-command-that-does-not-exist").
		Then(testBinary(t), helperOpt("cat")).
		Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "start command 0") {
		t.Errorf("Run() error = %v, want it to mention start command 0", err)
	}
}

func TestPipe_MiddleCommandStdoutRejected(t *testing.T) {
	var stdout bytes.Buffer
	err := Pipe(context.Background(), testBinary(t),
		helperOpt("cat"),
		WithStdout(&stdout),
	).
		Then(testBinary(t), helperOpt("upper")).
		Run()
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "must not set stdout") {
		t.Errorf("Run() error = %v, want it to mention stdout", err)
	}
}

func TestPipe_ZeroValue(t *testing.T) {
	var p Pipeline
	p.WithStdout(&bytes.Buffer{})
	p.WithStderr(&bytes.Buffer{})

	if err := p.Run(); err != nil {
		t.Errorf("zero Pipeline Run() error = %v, want nil", err)
	}
	out, err := p.Output()
	if err != nil {
		t.Errorf("zero Pipeline Output() error = %v, want nil", err)
	}
	if len(out) != 0 {
		t.Errorf("zero Pipeline Output() = %q, want empty", out)
	}
}

func TestPipe_Reusable(t *testing.T) {
	p := Pipe(context.Background(), testBinary(t), helperOpt("echo", "reuse"))

	for i := 0; i < 2; i++ {
		out, err := p.Output()
		if err != nil {
			t.Fatalf("Output() #%d error = %v, want nil", i+1, err)
		}
		if got, want := string(out), "reuse\n"; got != want {
			t.Errorf("output #%d = %q, want %q", i+1, got, want)
		}
	}
}
