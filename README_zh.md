# cmdx

[English](README.md) | [简体中文](README_zh.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/hushink/cmdx.svg)](https://pkg.go.dev/github.com/hushink/cmdx)
[![Go Report Card](https://goreportcard.com/badge/github.com/hushink/cmdx)](https://goreportcard.com/report/github.com/hushink/cmdx)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![Go Version](https://img.shields.io/badge/Go-1.26.1+-00ADD8?style=flat&logo=go)
![Test Coverage](https://img.shields.io/badge/Coverage-97.4%25-brightgreen)

**cmdx** 是对 Go 标准库 [`os/exec`](https://pkg.go.dev/os/exec) 的一层轻量封装，
零第三方依赖。它把三种常见用法——同步执行命令、异步启动并等待进程、类 shell
的管道——整理成一组紧凑的函数式选项。它不是 shell、不是进程守护工具，也不是
新的进程模型：命令、I/O、退出码和取消语义全部直接来自 `os/exec` 与 `context`。

## ✨ 特性

- 🪶 **保持轻量** - 只做 `os/exec` 之上的便捷封装，不模拟 shell，不引入自定义进程模型
- 🔒 **上下文驱动** - 直接使用 `context.Context` 控制取消和超时
- 🔄 **同步与异步** - `Run` 同步阻塞执行，`Start` + `Wait` 管理进程生命周期
- 🔗 **小型管道** - 通过标准输入输出连接命令，不启动 shell
- 📝 **可选日志** - 默认静默，`Logger` 接口兼容 `log/slog`
- 🌍 **跨平台** - 跟随 `os/exec`：Windows、Linux、macOS
- 🧪 **测试完善** - 测试使用测试二进制作为子进程，不依赖外部命令
- 🎯 **零第三方依赖** - 仅使用 Go 标准库

## 🎯 设计范围

cmdx 有意保持精简。它**不**做以下事情：

- 不调用 shell，也不解释 `|`、重定向、通配符、引号等 shell 语法；
- 不负责进程守护、自动重启或常驻进程管理；
- 不包装也不重新解释退出码——非零退出仍然是 `*exec.ExitError`。

需要更多能力时，请直接使用 `os/exec`；cmdx 不会改变其类型和错误语义。

## 📦 安装

```bash
go get github.com/hushink/cmdx
```

要求 Go 1.26.1 或更高版本。

## 🚀 快速开始

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

### 捕获输出

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

### 超时控制

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

### 同步执行命令 - `Run`

同步执行命令，等待其退出后返回。

```go
func Run(ctx context.Context, name string, opts ...Option) error
```

- 命令以非零状态退出时，错误通常为 `*exec.ExitError`。
- 执行被 `ctx` 取消或超时时，错误会包装 `context.Canceled` 或 `context.DeadlineExceeded`，可用 `errors.Is` 判断。

```go
err := cmdx.Run(ctx, "git",
    cmdx.WithArgs("status"),
    cmdx.WithDir("/path/to/repo"),
    cmdx.WithStdout(os.Stdout),
)
```

### 异步执行命令 - `Start`

异步启动命令，返回 `Process` 用于等待进程结束。

```go
func Start(ctx context.Context, name string, opts ...Option) (Process, error)

type Process interface {
    Wait() error
}
```

`Wait` 会等待进程真正退出并回收资源，可以安全地多次调用。取消传入的 `ctx` 会终止进程。

```go
proc, err := cmdx.Start(ctx, "server", cmdx.WithArgs("--port", "8080"))
if err != nil {
    log.Fatal(err)
}

// 在后台等待进程完成
go func() {
    if err := proc.Wait(); err != nil {
        log.Println("Process error:", err)
    }
}()
```

### 命令管道 - `Pipe`

`Pipe` 创建管道，前一条命令的 stdout 会连接到后一条命令的 stdin。

```go
func Pipe(ctx context.Context, name string, opts ...Option) *Pipeline
```

**示例：**

```go
// 等价于: cat file.txt | grep "error" | wc -l
output, err := cmdx.Pipe(ctx, "cat", cmdx.WithArgs("file.txt")).
    Then("grep", cmdx.WithArgs("error")).
    Then("wc", cmdx.WithArgs("-l")).
    Output()

if err != nil {
    log.Fatal(err)
}
fmt.Println("Error count:", string(output))
```

**Pipeline 方法：**

| 方法 | 说明 |
| --- | --- |
| `Then(name string, opts ...Option) *Pipeline` | 追加一条命令，连接前一条命令的 stdout |
| `WithStdout(w io.Writer) *Pipeline` | 设置最后一条命令的 stdout（捕获时同时写入捕获缓冲区） |
| `WithStderr(w io.Writer) *Pipeline` | 设置最后一条命令的 stderr（捕获时同时写入捕获缓冲区） |
| `Run() error` | 执行管道，不捕获输出 |
| `Output() ([]byte, error)` | 执行管道并捕获最后一条命令的 stdout |
| `CombinedOutput() ([]byte, error)` | 执行管道并合并最后一条命令的 stdout 与 stderr |

注意事项：

- 管道由 `os/exec` 的管道直接拼装，不启动 shell，因此 shell 语法和内建命令不可用。
- 管道中任一命令失败都会使整个管道返回错误（类似 shell 的 `pipefail`），多个错误通过 `errors.Join` 合并。
- 管道级超时由 `Pipe` 的 `WithTimeout` 设置；`Then` 中的 `WithTimeout` 和 `WithLogger` 会被忽略。
- 非最后一条命令不能设置 stdout，否则执行时会返回明确的错误。
- `Pipeline` 可以重复执行，但方法本身不是并发安全的。

### 配置选项

```go
// 设置命令参数（不包含命令名）
func WithArgs(args ...string) Option

// 设置环境变量（完全替换；未设置时继承父进程环境）
func WithEnv(env []string) Option

// 设置标准输入/输出/错误
func WithStdin(r io.Reader) Option
func WithStdout(w io.Writer) Option
func WithStderr(w io.Writer) Option

// 设置工作目录
func WithDir(dir string) Option

// 设置超时（d <= 0 表示不额外设置超时）
func WithTimeout(d time.Duration) Option

// 设置日志器（nil 表示不输出日志）
func WithLogger(l Logger) Option
```

`WithArgs`、`WithEnv` 会复制传入的切片；同一选项重复传入时，后传入的生效。

#### 环境变量

默认情况下，子进程继承父进程的环境变量：

```go
cmdx.Run(ctx, "env") // 可以看到父进程的环境变量
```

`WithEnv` **完全替换**子进程环境；传入空切片表示空环境：

```go
cmdx.Run(ctx, "env", cmdx.WithEnv([]string{
    "PATH=/usr/local/bin",
    "HOME=/root",
}))

// 空环境
cmdx.Run(ctx, "env", cmdx.WithEnv([]string{}))
```

#### 超时

`WithTimeout` 作用于单次 `Run`/`Start` 或整个 `Pipeline`。超时后进程会被终止，返回的错误包装 `context.DeadlineExceeded`：

```go
err := cmdx.Run(ctx, "sleep", cmdx.WithArgs("60"), cmdx.WithTimeout(5*time.Second))
if errors.Is(err, context.DeadlineExceeded) {
    // 超时
}
```

`d <= 0` 表示不额外设置超时，命令仍受 `ctx` 控制。

### 错误处理

```go
err := cmdx.Run(ctx, "git", cmdx.WithArgs("status"))

var exitErr *exec.ExitError
switch {
case errors.Is(err, context.DeadlineExceeded):
    // 超时
case errors.Is(err, context.Canceled):
    // 被取消
case errors.As(err, &exitErr):
    // 命令以非零状态退出
    fmt.Println("exit code:", exitErr.ExitCode())
case err != nil:
    // 启动失败等其他错误
}
```

### 自定义日志器

`Logger` 接口与 `*slog.Logger` 兼容：

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

**默认不输出日志**，避免库调用方产生意外的日志噪音。需要日志时显式传入：

```go
// 使用标准库默认日志器
cmdx.Run(ctx, "git", cmdx.WithArgs("status"), cmdx.WithLogger(slog.Default()))

// 使用自定义日志器
cmdx.Run(ctx, "git", cmdx.WithArgs("status"), cmdx.WithLogger(&MyLogger{}))
```

自定义日志器示例：

```go
type MyLogger struct{}

func (l *MyLogger) Debug(msg string, args ...any) {}
func (l *MyLogger) Info(msg string, args ...any)  { log.Printf("INFO: %s %v", msg, args) }
func (l *MyLogger) Warn(msg string, args ...any)  { log.Printf("WARN: %s %v", msg, args) }
func (l *MyLogger) Error(msg string, args ...any) { log.Printf("ERROR: %s %v", msg, args) }
```

## 💡 高级示例

### 执行命令并处理输出

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

### 并行执行多个命令

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

### 使用管道处理数据流

```go
// 等价于: ps aux | grep nginx | awk '{print $2}'
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

### 长时间运行的进程管理

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

## 🧪 测试

```bash
go test -v ./...

# 覆盖率
go test -cover ./...

# 竞态检测（需要 CGO）
CGO_ENABLED=1 go test -race ./...
```

测试通过当前测试二进制派生子进程来模拟命令，不依赖 `echo`、`sleep` 等平台相关的外部命令，因此在 Windows、Linux 和 macOS 上行为一致。

**当前测试覆盖率：97.4%**

## 📋 要求

- Go 1.26.1 或更高版本
- 支持的操作系统：Windows、Linux、macOS

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 Apache-2.0 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🔗 相关链接

- [Go 文档](https://golang.org/doc/)
- [os/exec 包文档](https://pkg.go.dev/os/exec)
- [context 包文档](https://pkg.go.dev/context)
- [log/slog 包文档](https://pkg.go.dev/log/slog)

## 💬 支持

如果有任何问题或建议，请：

- 提交 [GitHub Issue](https://github.com/hushink/cmdx/issues)
- 查看 [GoDoc 文档](https://pkg.go.dev/github.com/hushink/cmdx)

---

**Made with ❤️ using Go**
