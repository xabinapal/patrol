package cli

import (
	"reflect"
	"testing"
)

func TestBuildLoginArgs(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		args        []string
		expected    []string
		expectError bool
	}{
		{
			name:        "empty args",
			method:      "",
			path:        "",
			args:        []string{},
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "with method only",
			method:      "userpass",
			path:        "",
			args:        []string{"username=admin"},
			expected:    []string{"-method=userpass", "username=admin"},
			expectError: false,
		},
		{
			name:        "with method and path",
			method:      "ldap",
			path:        "ldap-corp",
			args:        []string{"username=user"},
			expected:    []string{"-method=ldap", "-path=ldap-corp", "username=user"},
			expectError: false,
		},
		{
			name:        "oidc method",
			method:      "oidc",
			path:        "",
			args:        []string{},
			expected:    []string{"-method=oidc"},
			expectError: false,
		},
		{
			name:        "github method with token",
			method:      "github",
			path:        "",
			args:        []string{"token=ghp_xxxx"},
			expected:    []string{"-method=github", "token=ghp_xxxx"},
			expectError: false,
		},
		{
			name:        "rejects invalid flag in args",
			method:      "",
			path:        "",
			args:        []string{"-invalid-flag"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "rejects non-K=V argument",
			method:      "",
			path:        "",
			args:        []string{"invalid"},
			expected:    nil,
			expectError: true,
		},
		{
			name:        "accepts multiple K=V pairs",
			method:      "userpass",
			path:        "",
			args:        []string{"username=admin", "password=secret"},
			expected:    []string{"-method=userpass", "username=admin", "password=secret"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildLoginArgs(tt.method, tt.path, tt.args)
			if tt.expectError {
				if err == nil {
					t.Errorf("buildLoginArgs(%q, %q, %v) expected error, got nil", tt.method, tt.path, tt.args)
				}
				return
			}
			if err != nil {
				t.Errorf("buildLoginArgs(%q, %q, %v) unexpected error: %v", tt.method, tt.path, tt.args, err)
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildLoginArgs(%q, %q, %v) = %v, want %v", tt.method, tt.path, tt.args, result, tt.expected)
			}
		})
	}
}

func TestBuildVaultLoginArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{"login", "-token-only", "-no-store"},
		},
		{
			name:     "with method and K=V",
			args:     []string{"-method=userpass", "username=admin"},
			expected: []string{"login", "-token-only", "-no-store", "-method=userpass", "username=admin"},
		},
		{
			name:     "with method, path and K=V",
			args:     []string{"-method=ldap", "-path=ldap-corp", "username=user"},
			expected: []string{"login", "-token-only", "-no-store", "-method=ldap", "-path=ldap-corp", "username=user"},
		},
		{
			name:     "only K=V pairs",
			args:     []string{"username=admin", "password=secret"},
			expected: []string{"login", "-token-only", "-no-store", "username=admin", "password=secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildVaultLoginArgs(tt.args)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildVaultLoginArgs(%v) = %v, want %v", tt.args, result, tt.expected)
			}
		})
	}
}

func TestParseLoginFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		currentMethod  string
		currentPath    string
		expectedMethod string
		expectedPath   string
		expectedRemain []string
		expectError    bool
	}{
		{
			name:           "empty args",
			args:           []string{},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "",
			expectedPath:   "",
			expectedRemain: nil,
			expectError:    false,
		},
		{
			name:           "method with equals",
			args:           []string{"-method=userpass", "username=admin"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "userpass",
			expectedPath:   "",
			expectedRemain: []string{"username=admin"},
			expectError:    false,
		},
		{
			name:           "method with double dash",
			args:           []string{"--method=ldap", "username=user"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "ldap",
			expectedPath:   "",
			expectedRemain: []string{"username=user"},
			expectError:    false,
		},
		{
			name:           "path with equals",
			args:           []string{"-path=ldap-corp", "username=user"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "",
			expectedPath:   "ldap-corp",
			expectedRemain: []string{"username=user"},
			expectError:    false,
		},
		{
			name:           "method and path",
			args:           []string{"-method=userpass", "-path=custom", "username=admin"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "userpass",
			expectedPath:   "custom",
			expectedRemain: []string{"username=admin"},
			expectError:    false,
		},
		{
			name:           "method space-separated",
			args:           []string{"-method", "userpass", "username=admin"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "userpass",
			expectedPath:   "",
			expectedRemain: []string{"username=admin"},
			expectError:    false,
		},
		{
			name:           "path space-separated",
			args:           []string{"-path", "ldap-corp", "username=user"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "",
			expectedPath:   "ldap-corp",
			expectedRemain: []string{"username=user"},
			expectError:    false,
		},
		{
			name:           "method space-separated missing value",
			args:           []string{"-method"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "",
			expectedPath:   "",
			expectedRemain: nil,
			expectError:    true,
		},
		{
			name:           "rejects invalid flag",
			args:           []string{"-invalid-flag"},
			currentMethod:  "",
			currentPath:    "",
			expectedMethod: "",
			expectedPath:   "",
			expectedRemain: nil,
			expectError:    true,
		},
		{
			name:           "preserves current method and path",
			args:           []string{"username=admin"},
			currentMethod:  "userpass",
			currentPath:    "custom",
			expectedMethod: "userpass",
			expectedPath:   "custom",
			expectedRemain: []string{"username=admin"},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, remain, err := parseLoginFlags(tt.args, tt.currentMethod, tt.currentPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("parseLoginFlags(%v, %q, %q) expected error, got nil", tt.args, tt.currentMethod, tt.currentPath)
				}
				return
			}
			if err != nil {
				t.Errorf("parseLoginFlags(%v, %q, %q) unexpected error: %v", tt.args, tt.currentMethod, tt.currentPath, err)
				return
			}
			if method != tt.expectedMethod {
				t.Errorf("parseLoginFlags() method = %q, want %q", method, tt.expectedMethod)
			}
			if path != tt.expectedPath {
				t.Errorf("parseLoginFlags() path = %q, want %q", path, tt.expectedPath)
			}
			if !reflect.DeepEqual(remain, tt.expectedRemain) {
				t.Errorf("parseLoginFlags() remain = %v, want %v", remain, tt.expectedRemain)
			}
		})
	}
}

func TestExtractTokenFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "token only",
			output:   "root-token",
			expected: "root-token",
		},
		{
			name:     "token with newline",
			output:   "root-token\n",
			expected: "root-token",
		},
		{
			name:     "token with prompt interleaved",
			output:   "Token (will be hidden): \nroot-token",
			expected: "root-token",
		},
		{
			name:     "token on last line with multiple lines",
			output:   "Token (will be hidden): \nroot-token\n",
			expected: "root-token",
		},
		{
			name:     "token with stderr prompt",
			output:   "Token (will be hidden): \nsome-warning\nroot-token",
			expected: "root-token",
		},
		{
			name:     "token with carriage return",
			output:   "Token (will be hidden): \r\nroot-token\r\n",
			expected: "root-token",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "only whitespace",
			output:   "   \n  \n  ",
			expected: "",
		},
		{
			name:     "token with leading/trailing spaces",
			output:   "  root-token  \n",
			expected: "root-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTokenFromOutput(tt.output)
			if result != tt.expected {
				t.Errorf("extractTokenFromOutput(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}
