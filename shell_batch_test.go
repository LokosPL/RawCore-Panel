package main

import (
	"strings"
	"testing"
)

func TestValidateShellScript(t *testing.T) {
	testCases := []struct {
		name, input, output string
		valid               bool
	}{
		{"multiple commands", "pwd\r\nwhoami\r\nuptime", "pwd\nwhoami\nuptime\n", true},
		{"unicode", "echo 'Zażółć gęślą'", "echo 'Zażółć gęślą'\n", true},
		{"empty", " \n \t", "", false},
		{"nul", "echo hi\x00", "", false},
		{"ansi escape", "\x1b[31mecho hi", "", false},
		{"too large", strings.Repeat("a", maxShellScriptBytes+1), "", false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := validateShellScript(tc.input)
			if (err == nil) != tc.valid || (tc.valid && actual != tc.output) {
				t.Fatalf("got %q, err=%v", actual, err)
			}
		})
	}
}

func TestShellBatchOutputIsBounded(t *testing.T) {
	out := &boundedShellOutput{}
	block := []byte(strings.Repeat("x", 4096))
	for i := 0; i < 150; i++ {
		if n, err := out.Write(block); n != len(block) || err != nil {
			t.Fatal(n, err)
		}
	}
	got, truncated := out.snapshot()
	if len(got) != maxShellOutputBytes || !truncated {
		t.Fatalf("size=%d truncated=%v", len(got), truncated)
	}
}
