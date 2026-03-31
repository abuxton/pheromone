package devops

import (
	"strings"
	"testing"
)

// TestRedactCommandForLog verifies that sensitive values are replaced with
// [REDACTED] while the command structure (program name, flags, plain args) is
// preserved.
func TestRedactCommandForLog(t *testing.T) {
	cases := []struct {
		name  string
		input string
		// wantContains is a slice of substrings that must appear in the output.
		wantContains []string
		// wantAbsent is a slice of substrings that must NOT appear in the output.
		wantAbsent []string
	}{
		{
			name:         "plain command without secrets",
			input:        "ls -la /var/log",
			wantContains: []string{"ls -la /var/log"},
		},
		{
			name:         "long flag with equals",
			input:        "curl --token=supersecret https://api.example.com",
			wantContains: []string{"curl", "--token=[REDACTED]", "https://api.example.com"},
			wantAbsent:   []string{"supersecret"},
		},
		{
			name:         "long flag with space separator",
			input:        "kubectl --password hunter2 apply -f deployment.yaml",
			wantContains: []string{"kubectl", "--password [REDACTED]", "apply"},
			wantAbsent:   []string{"hunter2"},
		},
		{
			name:         "short -p flag",
			input:        "mysql -p s3cr3t -u root",
			wantContains: []string{"mysql", "-p [REDACTED]", "-u root"},
			wantAbsent:   []string{"s3cr3t"},
		},
		{
			name:         "url with embedded credentials",
			input:        "git clone https://user:ghp_abc123@github.com/org/repo.git",
			wantContains: []string{"git clone", "https://[REDACTED]@github.com/org/repo.git"},
			wantAbsent:   []string{"ghp_abc123"},
		},
		{
			name:         "env variable assignment TOKEN=",
			input:        "TOKEN=abc123xyz helm upgrade --install app ./chart",
			wantContains: []string{"TOKEN=[REDACTED]", "helm upgrade", "--install", "app"},
			wantAbsent:   []string{"abc123xyz"},
		},
		{
			name:         "env variable assignment PASSWORD=",
			input:        "PASSWORD=hunter2 some-command",
			wantContains: []string{"PASSWORD=[REDACTED]"},
			wantAbsent:   []string{"hunter2"},
		},
		{
			name:         "authorization header in curl -H",
			input:        `curl -H "Authorization: Bearer ghp_mytoken123" https://api.github.com`,
			wantContains: []string{"curl", "Authorization: Bearer [REDACTED]"},
			wantAbsent:   []string{"ghp_mytoken123"},
		},
		{
			name:         "api-key long flag with equals",
			input:        "some-cli --api-key=topSecretKey123 --region us-east-1",
			wantContains: []string{"some-cli", "--api-key=[REDACTED]", "--region us-east-1"},
			wantAbsent:   []string{"topSecretKey123"},
		},
		{
			name:         "multiple secrets in one command",
			input:        "deploy --token=tok1 --password secret2 --region us-west-2",
			wantContains: []string{"deploy", "--token=[REDACTED]", "--password [REDACTED]", "--region us-west-2"},
			wantAbsent:   []string{"tok1", "secret2"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := redactCommandForLog(tc.input)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q\n  input: %q\n  got:   %q", want, tc.input, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("expected output NOT to contain %q\n  input: %q\n  got:   %q", absent, tc.input, got)
				}
			}
		})
	}
}
