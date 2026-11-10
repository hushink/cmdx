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

// Package cmdx is a small convenience wrapper around os/exec.
//
// It exposes three entry points over the standard library's process
// primitives:
//
//   - Run: execute a command synchronously.
//   - Start: start a process asynchronously and return a Process to wait on.
//   - Pipe: connect commands through stdin/stdout without invoking a shell.
//
// Commands, I/O streams, exit codes, and cancellation semantics come directly
// from os/exec and context. Every command accepts a context.Context, and
// WithTimeout sets a timeout for one execution. Child processes inherit the
// parent's environment by default; WithEnv replaces it completely. The library
// does not log by default; pass a Logger implementation (for example,
// slog.Default()) via WithLogger to enable logging.
package cmdx
