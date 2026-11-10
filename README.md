# cmdx

[English](README.md) | [简体中文](README_zh.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/hushink/cmdx.svg)](https://pkg.go.dev/github.com/hushink/cmdx)
[![Go Report Card](https://goreportcard.com/badge/github.com/hushink/cmdx)](https://goreportcard.com/report/github.com/hushink/cmdx)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![Go Version](https://img.shields.io/badge/Go-1.26.1+-00ADD8?style=flat&logo=go)
![Test Coverage](https://img.shields.io/badge/Coverage-97.4%25-brightgreen)

**cmdx** is a small, dependency-free convenience wrapper around Go's standard
[`os/exec`](https://pkg.go.dev/os/exec) package. It packages three common
patterns — synchronous command execution, asynchronous start/wait, and
shell-like pipelines — into a compact set of functional options. It is not a
shell, a process supervisor, or a new process model: commands, I/O, exit codes,
and cancellation semantics come directly from `os/exec` and `context`.

## ✨ Features

- 🪶 **Thin by design** - A focused convenience layer over `os/exec`; no shell emulation or custom process model
- 🔒 **Context-aware** - Uses `context.Context` directly for cancellation and deadlines
- 🔄 **Sync and async** - `Run` for blocking calls, `Start` + `Wait` for process lifetimes
- 🔗 **Small pipelines** - Connects command standard I/O without invoking a shell
- 📝 **Optional logging** - Silent by default; the `Logger` interface is compatible with `log/slog`
- 🌍 **Cross-platform** - Anywhere `os/exec` works: Windows, Linux, and macOS
- 🧪 **Well tested** - Tests use the test binary as a child process and need no external commands
- 🎯 **Zero third-party dependencies** - Standard library only

## 🎯 Scope

cmdx intentionally stays small. It does **not**:

- invoke a shell or interpret shell syntax such as `|`, redirection, globbing, or quoting;
- supervise processes, restart them, or manage daemon lifecycles;
- wrap or reinterpret exit codes — a non-zero exit is still `*exec.ExitError`.

When you need more than a thin convenience layer, use `os/exec` directly; its
types and errors pass through unchanged.

## 📦 Installation

```bash
go get github.com/hushink/cmdx
```

Requires Go 1.26.1 or later.

## 🚀 Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/hushink/cmdx"
)

func main() {
    ctx := context.Background()

    err := cmdx.Run(ctx, "git", cmdx.WithArgs("status"))
    if err != nil {
        fmt.Println("Error:", err)
    }
}
```

### Capturing output

```go
var buf bytes.Buffer
err := cmdx.Run(ctx, "go",
    cmdx.WithArgs("version"),
    cmdx.WithStdout(&buf),
)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Output:", buf.String())
```

### Timeout control

```go
err := cmdx.Run(ctx, "sleep",
    cmdx.WithArgs("10"),
    cmdx.WithTimeout(5*time.Second),
)
if errors.Is(err, context.DeadlineExceeded) {
    fmt.Println("command timed out")
}
```

## 📖 API

### Synchronous execution - `Run`

Executes a command synchronously and returns after it exits.

```go
func Run(ctx context.Context, name string, opts ...Option) error
```

- If the command exits with a non-zero status, the error is usually `*exec.ExitError`.
- If execution is canceled or times out, the error wraps `context.Canceled` or `context.DeadlineExceeded`, which can be tested with `errors.Is`.

```go
err := cmdx.Run(ctx, "git",
    cmdx.WithArgs("status"),
    cmdx.WithDir("/path/to/repo"),
    cmdx.WithStdout(os.Stdout),
)
```

### Asynchronous execution - `Start`

Starts a command asynchronously and returns a `Process` used to wait for the process.

```go
func Start(ctx context.Context, name string, opts ...Option) (Process, error)

type Process interface {
    Wait() error
}
```

`Wait` waits for the process to actually exit and reclaims its resources. It is safe to call multiple times. Canceling the provided `ctx` terminates the process.

```go
proc, err := cmdx.Start(ctx, "server", cmdx.WithArgs("--port", "8080"))
if err != nil {
    log.Fatal(err)
}

// Wait for the process in the background
go func() {
    if err := proc.Wait(); err != nil {
        log.Println("Process error:", err)
    }
}()
```

### Command pipelines - `Pipe`

`Pipe` creates a pipeline. The stdout of each command is connected to the stdin of the next.

```go
func Pipe(ctx context.Context, name string, opts ...Option) *Pipeline
```

**Example:**

```go
// Equivalent to: cat file.txt | grep "error" | wc -l
output, err := cmdx.Pipe(ctx, "cat", cmdx.WithArgs("file.txt")).
    Then("grep", cmdx.WithArgs("error")).
    Then("wc", cmdx.WithArgs("-l")).
    Output()

if err != nil {
    log.Fatal(err)
}
fmt.Println("Error count:", string(output))
```

**Pipeline methods:**

| Method | Description |
| --- | --- |
| `Then(name string, opts ...Option) *Pipeline` | Appends a command and connects the previous command's stdout |
| `WithStdout(w io.Writer) *Pipeline` | Sets the last command's stdout (also written to the capture buffer when capturing) |
| `WithStderr(w io.Writer) *Pipeline` | Sets the last command's stderr (also written to the capture buffer when capturing) |
| `Run() error` | Executes the pipeline without capturing output |
| `Output() ([]byte, error)` | Executes the pipeline and captures the last command's stdout |
| `CombinedOutput() ([]byte, error)` | Executes the pipeline and merges the last command's stdout and stderr |

Notes:

- The pipeline is assembled directly from `os/exec` pipes; no shell is invoked, so shell syntax and built-ins are not available.
- Any command failure fails the whole pipeline (similar to shell `pipefail`); multiple errors are joined with `errors.Join`.
- The pipeline timeout is set with `WithTimeout` on `Pipe`; `WithTimeout` and `WithLogger` passed to `Then` are ignored.
- A command that is not the last one must not set stdout, otherwise execution returns a clear error.
- A `Pipeline` can be executed repeatedly, but its methods are not safe for concurrent use.

### Options

```go
// Set command arguments (excluding the command name)
func WithArgs(args ...string) Option

// Set environment variables (replaces the environment; inherits by default)
func WithEnv(env []string) Option

// Set standard input/output/error
func WithStdin(r io.Reader) Option
func WithStdout(w io.Writer) Option
func WithStderr(w io.Writer) Option

// Set the working directory
func WithDir(dir string) Option

// Set a timeout (a non-positive duration applies no additional timeout)
func WithTimeout(d time.Duration) Option

// Set the logger (nil disables logging)
func WithLogger(l Logger) Option
```

`WithArgs` and `WithEnv` copy the provided slices. When the same option is passed multiple times, the last one wins.

#### Environment variables

By default, child processes inherit the parent's environment:

```go
cmdx.Run(ctx, "env") // sees the parent process environment
```

`WithEnv` **completely replaces** the child environment; an empty slice means an empty environment:

```go
cmdx.Run(ctx, "env", cmdx.WithEnv([]string{
    "PATH=/usr/local/bin",
    "HOME=/root",
}))

// Empty environment
cmdx.Run(ctx, "env", cmdx.WithEnv([]string{}))
```

#### Timeout

`WithTimeout` applies to a single `Run`/`Start` or an entire `Pipeline`. When the timeout expires, the process is terminated and the returned error wraps `context.DeadlineExceeded`:

```go
err := cmdx.Run(ctx, "sleep", cmdx.WithArgs("60"), cmdx.WithTimeout(5*time.Second))
if errors.Is(err, context.DeadlineExceeded) {
    // timed out
}
```

A non-positive duration applies no additional timeout; the command is still bound by `ctx`.

### Error handling

```go
err := cmdx.Run(ctx, "git", cmdx.WithArgs("status"))

var exitErr *exec.ExitError
switch {
case errors.Is(err, context.DeadlineExceeded):
    // timed out
case errors.Is(err, context.Canceled):
    // canceled
case errors.As(err, &exitErr):
    // command exited with a non-zero status
    fmt.Println("exit code:", exitErr.ExitCode())
case err != nil:
    // other errors such as a failure to start
}
```

### Custom logger

The `Logger` interface is compatible with `*slog.Logger`:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

**Logging is disabled by default** to avoid unexpected log noise for library callers. Pass a logger explicitly when needed:

```go
// Use the standard library default logger
cmdx.Run(ctx, "git", cmdx.WithArgs("status"), cmdx.WithLogger(slog.Default()))

// Use a custom logger
cmdx.Run(ctx, "git", cmdx.WithArgs("status"), cmdx.WithLogger(&MyLogger{}))
```

Custom logger example:

```go
type MyLogger struct{}

func (l *MyLogger) Debug(msg string, args ...any) {}
func (l *MyLogger) Info(msg string, args ...any)  { log.Printf("INFO: %s %v", msg, args) }
func (l *MyLogger) Warn(msg string, args ...any)  { log.Printf("WARN: %s %v", msg, args) }
func (l *MyLogger) Error(msg string, args ...any) { log.Printf("ERROR: %s %v", msg, args) }
```

## 💡 Advanced Examples

### Execute a command and process its output

```go
var stdout, stderr bytes.Buffer

err := cmdx.Run(ctx, "go",
    cmdx.WithArgs("test", "./..."),
    cmdx.WithDir("/path/to/project"),
    cmdx.WithStdout(&stdout),
    cmdx.WithStderr(&stderr),
    cmdx.WithTimeout(5*time.Minute),
)

if err != nil {
    fmt.Println("Test failed:")
    fmt.Println(stderr.String())
} else {
    fmt.Println("Tests passed!")
    fmt.Println(stdout.String())
}
```

### Run multiple commands in parallel

```go
commands := []struct {
    name string
    args []string
}{
    {"git", []string{"status"}},
    {"go", []string{"version"}},
    {"ls", []string{"-la"}},
}

var wg sync.WaitGroup
for _, c := range commands {
    wg.Add(1)
    go func(name string, args []string) {
        defer wg.Done()

        if err := cmdx.Run(ctx, name,
            cmdx.WithArgs(args...),
            cmdx.WithTimeout(10*time.Second),
        ); err != nil {
            fmt.Printf("Command %s failed: %v\n", name, err)
        }
    }(c.name, c.args)
}
wg.Wait()
```

### Process a data stream with a pipeline

```go
// Equivalent to: ps aux | grep nginx | awk '{print $2}'
output, err := cmdx.Pipe(ctx, "ps", cmdx.WithArgs("aux")).
    Then("grep", cmdx.WithArgs("nginx")).
    Then("awk", cmdx.WithArgs("{print $2}")).
    Output()

if err != nil {
    fmt.Println("Error:", err)
    return
}
fmt.Println("Nginx PIDs:")
fmt.Println(string(output))
```

### Managing a long-running process

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    fmt.Println("\nReceived signal, shutting down...")
    cancel()
}()

proc, err := cmdx.Start(ctx, "python",
    cmdx.WithArgs("-m", "http.server", "8080"),
    cmdx.WithTimeout(1*time.Hour),
)
if err != nil {
    fmt.Println("Failed to start:", err)
    return
}

fmt.Println("Server started, press Ctrl+C to stop")
if err := proc.Wait(); err != nil {
    fmt.Println("Process error:", err)
}
```

## 🧪 Testing

```bash
go test -v ./...

# Coverage
go test -cover ./...

# Race detector (requires CGO)
CGO_ENABLED=1 go test -race ./...
```

Tests derive child processes from the current test binary to emulate commands. They do not depend on platform-specific external commands such as `echo` or `sleep`, so they behave consistently on Windows, Linux, and macOS.

**Current test coverage: 97.4%**

## 📋 Requirements

- Go 1.26.1 or later
- Supported operating systems: Windows, Linux, macOS

## 🤝 Contributing

Issues and pull requests are welcome!

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a pull request

## 📄 License

This project is licensed under the Apache-2.0 License. See the [LICENSE](LICENSE) file for details.

## 🔗 Related Links

- [Go documentation](https://golang.org/doc/)
- [os/exec package documentation](https://pkg.go.dev/os/exec)
- [context package documentation](https://pkg.go.dev/context)
- [log/slog package documentation](https://pkg.go.dev/log/slog)

## 💬 Support

If you have any questions or suggestions:

- Open a [GitHub issue](https://github.com/hushink/cmdx/issues)
- Read the [GoDoc](https://pkg.go.dev/github.com/hushink/cmdx)

---

**Made with ❤️ using Go**
