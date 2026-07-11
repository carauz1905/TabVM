package vbox

import "testing"

func TestIsValidVmID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{
			name:  "empty string",
			id:    "",
			valid: false,
		},
		{
			name:  "standard lowercase uuid",
			id:    "550e8400-e29b-41d4-a716-446655440000",
			valid: true,
		},
		{
			name:  "standard uppercase uuid",
			id:    "550E8400-E29B-41D4-A716-446655440000",
			valid: true,
		},
		{
			name:  "simple test id",
			id:    "uuid-1",
			valid: false,
		},
		{
			name:  "alphanumeric id",
			id:    "vm123",
			valid: false,
		},
		{
			name:  "contains space",
			id:    "550e8400-e29b-41d4-a716-44665544000 0",
			valid: false,
		},
		{
			name:  "contains quote",
			id:    `550e8400-e29b-41d4-a716-44665544000"0`,
			valid: false,
		},
		{
			name:  "contains semicolon",
			id:    "550e8400-e29b-41d4-a716-446655440000;",
			valid: false,
		},
		{
			name:  "contains brace",
			id:    "{550e8400-e29b-41d4-a716-446655440000}",
			valid: false,
		},
		{
			name:  "path traversal",
			id:    "../etc/passwd",
			valid: false,
		},
		{
			name:  "too long",
			id:    string(make([]byte, 65)),
			valid: false,
		},
		{
			name:  "missing dashes",
			id:    "550e8400e29b41d4a716446655440000",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidVmID(tt.id)
			if got != tt.valid {
				t.Fatalf("IsValidVmID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}
