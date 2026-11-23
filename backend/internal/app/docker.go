package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type serverActionResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

var dockerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func mapServerAction(action string) (string, bool) {
	switch action {
	case "wipe":
		return "w", true
	case "update":
		return "u", true
	default:
		return "", false
	}
}

func dockerExecRustServer(ctx context.Context, dockerBin string, containerName string, rustServerArg string) (stdout string, stderr string, exitCode int, err error) {
	if _, lookErr := exec.LookPath(dockerBin); lookErr != nil {
		return "", "", 127, lookErr
	}

	cmd := exec.CommandContext(ctx, dockerBin, "exec", "-u", "linuxgsm", containerName, "./rustserver", rustServerArg)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = limitOutput(outBuf.String(), 64*1024)
	stderr = limitOutput(errBuf.String(), 64*1024)
	if runErr == nil {
		return stdout, stderr, 0, nil
	}

	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout, stderr, 124, runErr
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return stdout, stderr, ee.ExitCode(), runErr
	}
	return stdout, stderr, 1, runErr
}

func dockerExecRustServerInDir(ctx context.Context, dockerBin string, containerName string, lgsmDir string, rustServerArg string) (stdout string, stderr string, exitCode int, err error) {
	if strings.TrimSpace(lgsmDir) == "" {
		return dockerExecRustServer(ctx, dockerBin, containerName, rustServerArg)
	}
	if _, lookErr := exec.LookPath(dockerBin); lookErr != nil {
		return "", "", 127, lookErr
	}

	script := fmt.Sprintf("cd %s && ./rustserver %s", shellQuoteArg(strings.TrimSpace(lgsmDir)), shellQuoteArg(strings.TrimSpace(rustServerArg)))
	cmd := exec.CommandContext(ctx, dockerBin, "exec", "-u", "linuxgsm", containerName, "sh", "-lc", script)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = limitOutput(outBuf.String(), 64*1024)
	stderr = limitOutput(errBuf.String(), 64*1024)
	if runErr == nil {
		return stdout, stderr, 0, nil
	}

	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout, stderr, 124, runErr
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return stdout, stderr, ee.ExitCode(), runErr
	}
	return stdout, stderr, 1, runErr
}

func shellQuoteArg(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func newActionTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 3*time.Minute)
}
