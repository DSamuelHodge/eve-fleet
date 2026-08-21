package clitest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHeadlessProducesNoANSI(t *testing.T) {
	root := initFleet(t, "plain-ops")
	cases := []struct {
		name string
		env  []string
		args []string
	}{
		{"json", nil, []string{"doctor", "--json"}},
		{"nocolor", []string{"NO_COLOR=1"}, []string{"doctor"}},
		{"version-json", nil, []string{"version", "--json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Run(t, root, tc.env, tc.args...)
			if res.Code != 0 {
				t.Fatalf("exit %d\n%s%s", res.Code, res.Stdout, res.Stderr)
			}
			out := res.Stdout + res.Stderr
			if strings.Contains(out, "\x1b") {
				t.Fatalf("ANSI found in headless output:\n%q", out)
			}
		})
	}
	_ = filepath.Separator
}
