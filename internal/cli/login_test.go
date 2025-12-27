package cli

import (
	"reflect"
	"testing"
)

func TestBuildLoginArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "empty args adds format and no-store",
			args:     []string{},
			expected: []string{"login", "-format=json", "-no-store"},
		},
		{
			name:     "with method adds format and no-store",
			args:     []string{"-method=userpass", "username=admin"},
			expected: []string{"login", "-format=json", "-no-store", "-method=userpass", "username=admin"},
		},
		{
			name:     "preserves user format flag",
			args:     []string{"-format=json"},
			expected: []string{"login", "-no-store", "-format=json"},
		},
		{
			name:     "preserves user format flag with double dash",
			args:     []string{"--format=table"},
			expected: []string{"login", "-no-store", "--format=table"},
		},
		{
			name:     "preserves user no-store flag",
			args:     []string{"-no-store"},
			expected: []string{"login", "-format=json", "-no-store"},
		},
		{
			name:     "preserves user no-store flag with double dash",
			args:     []string{"--no-store"},
			expected: []string{"login", "-format=json", "--no-store"},
		},
		{
			name:     "preserves both user flags",
			args:     []string{"-format=json", "-no-store"},
			expected: []string{"login", "-format=json", "-no-store"},
		},
		{
			name:     "complex args with method and path",
			args:     []string{"-method=ldap", "-path=ldap-corp", "username=user"},
			expected: []string{"login", "-format=json", "-no-store", "-method=ldap", "-path=ldap-corp", "username=user"},
		},
		{
			name:     "oidc method",
			args:     []string{"-method=oidc"},
			expected: []string{"login", "-format=json", "-no-store", "-method=oidc"},
		},
		{
			name:     "github method with token",
			args:     []string{"-method=github", "token=ghp_xxxx"},
			expected: []string{"login", "-format=json", "-no-store", "-method=github", "token=ghp_xxxx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLoginArgs(tt.args)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildLoginArgs(%v) = %v, want %v", tt.args, result, tt.expected)
			}
		})
	}
}

func TestHasJSONFormat(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: false,
		},
		{
			name:     "no format flag",
			args:     []string{"-method=userpass"},
			expected: false,
		},
		{
			name:     "format=json with single dash",
			args:     []string{"-format=json"},
			expected: true,
		},
		{
			name:     "format=json with double dash",
			args:     []string{"--format=json"},
			expected: true,
		},
		{
			name:     "format flag alone (assumes json follows)",
			args:     []string{"-format"},
			expected: true,
		},
		{
			name:     "format flag with other args",
			args:     []string{"-method=ldap", "-format=json", "username=user"},
			expected: true,
		},
		{
			name:     "format=table is not json",
			args:     []string{"-format=table"},
			expected: false,
		},
		{
			name:     "format=yaml is not json",
			args:     []string{"--format=yaml"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasJSONFormat(tt.args)
			if result != tt.expected {
				t.Errorf("hasJSONFormat(%v) = %v, want %v", tt.args, result, tt.expected)
			}
		})
	}
}
