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
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	helperMarker = "cmdx-helper"
	helperRun    = "-test.run=^TestHelperProcess$"
)

// TestHelperProcess is not a real test. It is the child-process entry point
// used by this package's tests. When the test binary is launched with
// "-- cmdx-helper <mode> ...", it emulates a command and exits; otherwise it
// returns immediately.
func TestHelperProcess(_ *testing.T) {
	args := argsAfterDoubleDash(os.Args)
	if len(args) < 2 || args[0] != helperMarker {
		return
	}

	switch args[1] {
	case "echo":
		fmt.Println(strings.Join(args[2:], " "))
	case "exit":
		code := 1
		if len(args) > 2 {
			code, _ = strconv.Atoi(args[2])
		}
		os.Exit(code)
	case "sleep":
		d, err := time.ParseDuration(args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		time.Sleep(d)
	case "env":
		fmt.Print(os.Getenv(args[2]))
	case "getwd":
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Print(wd)
	case "cat":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "upper":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Print(strings.ToUpper(string(data)))
	case "grep":
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), args[2]) {
				fmt.Println(scanner.Text())
			}
		}
	case "stderr":
		fmt.Fprint(os.Stderr, strings.Join(args[2:], " "))
	case "both":
		fmt.Print("out")
		fmt.Fprint(os.Stderr, "err")
	default:
		fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", args[1])
		os.Exit(2)
	}

	os.Exit(0)
}

func argsAfterDoubleDash(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}

// testBinary returns the absolute path of the current test binary.
func testBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return exe
}

// helperArgs returns the arguments needed to run the helper child process in
// the given mode.
func helperArgs(mode string, args ...string) []string {
	all := make([]string, 0, 4+len(args))
	all = append(all, helperRun, "--", helperMarker, mode)
	all = append(all, args...)
	return all
}

// helperOpt returns an Option that runs the helper child process in the given
// mode.
func helperOpt(mode string, args ...string) Option {
	return WithArgs(helperArgs(mode, args...)...)
}
