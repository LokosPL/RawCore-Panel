package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

const maxShellScriptBytes = 128 << 10
const maxShellOutputBytes = 512 << 10

type shellBatchResult struct {
	Output    string `json:"output"`
	ExitCode  int    `json:"exitCode"`
	Truncated bool   `json:"truncated"`
}

func validateShellScript(source string) (string, error) {
	source = strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
	if len(source) > maxShellScriptBytes {
		return "", errors.New("Skrypt przekracza limit 128 KiB")
	}
	if !utf8.ValidString(source) {
		return "", errors.New("Skrypt musi być tekstem UTF-8")
	}
	for _, c := range source {
		if (c < 32 && c != '\t' && c != '\n') || c == 127 {
			return "", errors.New("Skrypt zawiera niedozwolone znaki sterujące")
		}
	}
	if strings.TrimSpace(source) == "" {
		return "", errors.New("Wklej co najmniej jedną komendę")
	}
	if !strings.HasSuffix(source, "\n") {
		source += "\n"
	}
	return source, nil
}

type boundedShellOutput struct {
	mu        sync.Mutex
	out       bytes.Buffer
	truncated bool
}

func (b *boundedShellOutput) Write(chunk []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(chunk)
	remaining := maxShellOutputBytes - b.out.Len()
	if remaining <= 0 {
		b.truncated = true
		return original, nil
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
		b.truncated = true
	}
	_, _ = b.out.Write(chunk)
	return original, nil
}

func (b *boundedShellOutput) snapshot() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Avoid splitting the final UTF-8 rune when the output is capped.
	return strings.ToValidUTF8(b.out.String(), "�"), b.truncated
}

func runShellBatch(ctx context.Context, client *ssh.Client, directory, source string) (shellBatchResult, error) {
	script, err := validateShellScript(source)
	if err != nil {
		return shellBatchResult{}, err
	}
	session, err := client.NewSession()
	if err != nil {
		return shellBatchResult{}, fmt.Errorf("Nie udało się otworzyć sesji skryptu: %w", err)
	}
	defer session.Close()
	output := &boundedShellOutput{}
	session.Stdout = output
	session.Stderr = output
	session.Stdin = strings.NewReader(script)
	// A separate, noninteractive shell avoids terminal line buffering/truncation
	// when a user pastes many commands. It does not change the open PTY's cwd.
	command := "if [ -d " + quote(directory) + " ]; then cd -- " + quote(directory) + "; else cd; fi; exec /bin/bash -se"
	if err := session.Start(command); err != nil {
		return shellBatchResult{}, err
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	waitCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	select {
	case err = <-done:
	case <-waitCtx.Done():
		_ = session.Close()
		return shellBatchResult{}, errors.New("Przekroczono limit 120 sekund lub przerwano wykonywanie skryptu")
	}
	text, truncated := output.snapshot()
	result := shellBatchResult{Output: text, Truncated: truncated}
	if err == nil {
		return result, nil
	}
	if exit, ok := err.(*ssh.ExitError); ok {
		result.ExitCode = exit.ExitStatus()
		return result, nil
	}
	return shellBatchResult{}, fmt.Errorf("Połączenie podczas skryptu zostało przerwane: %w", err)
}
