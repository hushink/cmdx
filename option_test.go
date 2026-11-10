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
	"testing"
	"time"
)

func TestWithArgsCopiesSlice(t *testing.T) {
	rawArgs := helperArgs("echo", "copied")
	opt := WithArgs(rawArgs...)

	// Mutating the original slice must not affect an Option that has already
	// been created.
	rawArgs[0] = "-test.run=^NoSuchTest$"

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t), opt, WithStdout(&stdout))
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "copied\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestWithEnvCopiesSlice(t *testing.T) {
	rawEnv := []string{"CMDX_TEST_ENV=copied"}
	opt := WithEnv(rawEnv)

	// Mutating the original slice must not affect an Option that has already
	// been created.
	rawEnv[0] = "CMDX_TEST_ENV=mutated"

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("env", "CMDX_TEST_ENV"),
		opt,
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "copied"; got != want {
		t.Errorf("env value = %q, want %q", got, want)
	}
}

func TestWithEnvNil(t *testing.T) {
	t.Setenv("CMDX_TEST_ENV", "inherited")

	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("env", "CMDX_TEST_ENV"),
		WithEnv(nil),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got := stdout.String(); got != "" {
		t.Errorf("env value = %q, want empty", got)
	}
}

func TestWithTimeoutZeroAndNegative(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		var stdout bytes.Buffer
		err := Run(context.Background(), testBinary(t),
			helperOpt("echo", "no timeout"),
			WithTimeout(d),
			WithStdout(&stdout),
		)
		if err != nil {
			t.Errorf("Run(timeout=%v) error = %v, want nil", d, err)
		}
	}
}

func TestOptionsAppliedInOrder(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), testBinary(t),
		helperOpt("echo", "first"),
		WithArgs(helperArgs("echo", "second")...),
		WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got, want := stdout.String(), "second\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
