package valueobject

import (
	"testing"
)

func TestSourceID(t *testing.T) {
	id := NewSourceID("default", "my-secret")

	if id.Namespace() != "default" {
		t.Errorf("Expected namespace 'default', got %q", id.Namespace())
	}

	if id.Name() != "my-secret" {
		t.Errorf("Expected name 'my-secret', got %q", id.Name())
	}

	expected := "default/my-secret"
	if id.String() != expected {
		t.Errorf("Expected String() to return %q, got %q", expected, id.String())
	}

	id2 := NewSourceID("default", "my-secret")
	if !id.Equals(id2) {
		t.Error("Expected two SourceIDs with same values to be equal")
	}

	id3 := NewSourceID("kube-system", "my-secret")
	if id.Equals(id3) {
		t.Error("Expected SourceIDs with different namespaces to not be equal")
	}
}

func TestMirrorID(t *testing.T) {
	id := NewMirrorID("namespace-1", "mirror-secret")

	if id.Namespace() != "namespace-1" {
		t.Errorf("Expected namespace 'namespace-1', got %q", id.Namespace())
	}

	if id.Name() != "mirror-secret" {
		t.Errorf("Expected name 'mirror-secret', got %q", id.Name())
	}

	expected := "namespace-1/mirror-secret"
	if id.String() != expected {
		t.Errorf("Expected String() to return %q, got %q", expected, id.String())
	}
}

func TestAllowedNamespaces_Matches(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		namespace string
		want      bool
	}{
		{
			name:      "single exact match",
			patterns:  []string{"namespace-1"},
			namespace: "namespace-1",
			want:      true,
		},
		{
			name:      "single exact no match",
			patterns:  []string{"namespace-1"},
			namespace: "namespace-2",
			want:      false,
		},
		{
			name:      "wildcard match all",
			patterns:  []string{"*"},
			namespace: "any-namespace",
			want:      true,
		},
		{
			name:      "prefix pattern",
			patterns:  []string{"namespace-1*"},
			namespace: "namespace-123",
			want:      true,
		},
		{
			name:      "prefix pattern no match",
			patterns:  []string{"namespace-1*"},
			namespace: "namespace-abc",
			want:      false,
		},
		{
			name:      "regex pattern no match",
			patterns:  []string{"namespace-[0-9]*"},
			namespace: "namespace-abc",
			want:      false,
		},
		{
			name:      "multiple patterns first matches",
			patterns:  []string{"namespace-1", "namespace-2", "namespace-3"},
			namespace: "namespace-1",
			want:      true,
		},
		{
			name:      "multiple patterns second matches",
			patterns:  []string{"namespace-1", "namespace-2", "namespace-3"},
			namespace: "namespace-2",
			want:      true,
		},
		{
			name:      "multiple patterns none match",
			patterns:  []string{"namespace-1", "namespace-2"},
			namespace: "namespace-3",
			want:      false,
		},
		{
			name:      "empty patterns none match",
			patterns:  []string{},
			namespace: "any-namespace",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			an, err := NewAllowedNamespaces(tt.patterns)
			if err != nil {
				t.Fatalf("Unexpected error creating AllowedNamespaces: %v", err)
			}

			if got := an.Matches(tt.namespace); got != tt.want {
				t.Errorf("AllowedNamespaces.Matches(%q) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestAllowedNamespaces_IsEmpty(t *testing.T) {
	an, _ := NewAllowedNamespaces([]string{})
	if !an.IsEmpty() {
		t.Error("Expected empty AllowedNamespaces to be empty")
	}

	an, _ = NewAllowedNamespaces([]string{"namespace-1"})
	if an.IsEmpty() {
		t.Error("Expected non-empty AllowedNamespaces to not be empty")
	}
}

func TestAllowedNamespaces_IsAll(t *testing.T) {
	an, _ := NewAllowedNamespaces([]string{"*"})
	if !an.IsAll() {
		t.Error("Expected wildcard AllowedNamespaces to have IsAll() = true")
	}

	an, _ = NewAllowedNamespaces([]string{"namespace-1"})
	if an.IsAll() {
		t.Error("Expected non-wildcard AllowedNamespaces to have IsAll() = false")
	}
}

func TestParseAllowedNamespacesAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{"*"},
		},
		{
			name:     "single namespace",
			input:    "namespace-1",
			expected: []string{"namespace-1"},
		},
		{
			name:     "multiple namespaces",
			input:    "namespace-1,namespace-2,namespace-3",
			expected: []string{"namespace-1", "namespace-2", "namespace-3"},
		},
		{
			name:     "multiple namespaces with spaces",
			input:    "namespace-1, namespace-2, namespace-3",
			expected: []string{"namespace-1", "namespace-2", "namespace-3"},
		},
		{
			name:     "wildcard",
			input:    "*",
			expected: []string{"*"},
		},
		{
			name:     "regex pattern",
			input:    "namespace-[0-9]*",
			expected: []string{"namespace-[0-9]*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAllowedNamespacesAnnotation(tt.input)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("Expected pattern[%d] = %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}
