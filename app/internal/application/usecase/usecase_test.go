package usecase_test

import (
	"testing"
)

func TestIsPrintable(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "printable ascii",
			data:     []byte("hello world"),
			expected: true,
		},
		{
			name:     "binary data",
			data:     []byte{0x00, 0x01, 0x02},
			expected: false,
		},
		{
			name:     "empty",
			data:     []byte{},
			expected: true,
		},
		{
			name:     "mixed",
			data:     []byte("hello\x00world"),
			expected: false,
		},
		{
			name:     "tab character",
			data:     []byte("hello\tworld"),
			expected: false,
		},
		{
			name:     "newline",
			data:     []byte("hello\nworld"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPrintable(tt.data)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}
