package clitest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	binOnce sync.Once
	binPath string
	binErr  error
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate module root")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func Binary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	binOnce.Do(func() {
		cache, err := os.MkdirTemp("", "eve-fleet-clitest-")
		if err != nil {
			binErr = err
			return
		}
		out := filepath.Join(cache, "eve-fleet")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/eve-fleet")
		cmd.Dir = root
		cmd.Env = os.Environ()
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			binErr = fmt.Errorf("go build failed: %w\n%s", err, buf.String())
			return
		}
		binPath = out
	})
	if binErr != nil {
		t.Fatal(binErr)
	}
	return binPath
}

type Result struct {
	Code   int
	Stdout string
	Stderr string
}

func Run(t *testing.T, dir string, extraEnv []string, args ...string) Result {
	t.Helper()
	cmd := exec.Command(Binary(t), args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, os.Environ()...), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run eve-fleet: %v", err)
		}
		code = ee.ExitCode()
	}
	return Result{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}
