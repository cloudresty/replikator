package annotations

import (
	"strings"
	"testing"
	"time"

	"replikator/internal/application/dto"
)

func TestMirrorAnnotations(t *testing.T) {
	sourceID := "default/my-secret"
	version := "v1"

	annotations := MirrorAnnotations(sourceID, version)

	if annotations[dto.AnnotationReflects] != sourceID {
		t.Errorf("expected reflects annotation to be %s, got %s", sourceID, annotations[dto.AnnotationReflects])
	}

	if annotations[dto.AnnotationReflectedVersion] != version {
		t.Errorf("expected reflected-version annotation to be %s, got %s", version, annotations[dto.AnnotationReflectedVersion])
	}

	if annotations[dto.AnnotationReflectedAt] == "" {
		t.Error("expected reflected-at annotation to be set")
	}

	if _, err := time.Parse(time.RFC3339, annotations[dto.AnnotationReflectedAt]); err != nil {
		t.Errorf("expected reflected-at to be valid RFC3339 time, got error: %v", err)
	}
}

func TestAutoMirrorAnnotations(t *testing.T) {
	sourceID := "default/my-secret"
	version := "v1"

	annotations := AutoMirrorAnnotations(sourceID, version)

	if annotations[dto.AnnotationReflects] != sourceID {
		t.Errorf("expected reflects annotation to be %s, got %s", sourceID, annotations[dto.AnnotationReflects])
	}

	if annotations[dto.AnnotationReflectedVersion] != version {
		t.Errorf("expected reflected-version annotation to be %s, got %s", version, annotations[dto.AnnotationReflectedVersion])
	}

	if annotations[dto.AnnotationAutoReflects] != "true" {
		t.Errorf("expected auto-reflects annotation to be true, got %s", annotations[dto.AnnotationAutoReflects])
	}

	if annotations[dto.AnnotationReflectedAt] == "" {
		t.Error("expected reflected-at annotation to be set")
	}
}

func TestSourceAnnotations(t *testing.T) {
	tests := []struct {
		name              string
		allowed           bool
		allowedNamespaces string
		autoEnabled       bool
		autoNamespaces    string
	}{
		{
			name:              "all options set",
			allowed:           true,
			allowedNamespaces: "namespace-1,namespace-2",
			autoEnabled:       true,
			autoNamespaces:    "namespace-3,namespace-4",
		},
		{
			name:              "only allowed set",
			allowed:           true,
			allowedNamespaces: "",
			autoEnabled:       false,
			autoNamespaces:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := SourceAnnotations(tt.allowed, tt.allowedNamespaces, tt.autoEnabled, tt.autoNamespaces)

			expectedAllowed := "true"
			if !tt.allowed {
				expectedAllowed = "false"
			}
			if annotations[dto.AnnotationReplicationAllowed] != expectedAllowed {
				t.Errorf("expected replication-allowed to be %s, got %s", expectedAllowed, annotations[dto.AnnotationReplicationAllowed])
			}

			if tt.allowedNamespaces != "" {
				if annotations[dto.AnnotationReplicationAllowedNamespaces] != tt.allowedNamespaces {
					t.Errorf("expected replication-allowed-namespaces to be %s, got %s", tt.allowedNamespaces, annotations[dto.AnnotationReplicationAllowedNamespaces])
				}
			}

			expectedAutoEnabled := "true"
			if !tt.autoEnabled {
				expectedAutoEnabled = "false"
			}
			if annotations[dto.AnnotationReplicationAutoEnabled] != expectedAutoEnabled {
				t.Errorf("expected replication-auto-enabled to be %s, got %s", expectedAutoEnabled, annotations[dto.AnnotationReplicationAutoEnabled])
			}

			if tt.autoNamespaces != "" {
				if annotations[dto.AnnotationReplicationAutoNamespaces] != tt.autoNamespaces {
					t.Errorf("expected replication-auto-namespaces to be %s, got %s", tt.autoNamespaces, annotations[dto.AnnotationReplicationAutoNamespaces])
				}
			}
		})
	}
}

func TestBoolToString(t *testing.T) {
	if boolToString(true) != "true" {
		t.Error("expected boolToString(true) to return 'true'")
	}
	if boolToString(false) != "false" {
		t.Error("expected boolToString(false) to return 'false'")
	}
}

func TestMirrorAnnotationsContainsAllKeys(t *testing.T) {
	annotations := MirrorAnnotations("default/secret", "v1")

	expectedKeys := []string{
		dto.AnnotationReflects,
		dto.AnnotationReflectedVersion,
		dto.AnnotationReflectedAt,
	}

	for _, key := range expectedKeys {
		if _, ok := annotations[key]; !ok {
			t.Errorf("expected annotation key %s to be present", key)
		}
	}

	if len(annotations) != len(expectedKeys) {
		t.Errorf("expected %d annotations, got %d", len(expectedKeys), len(annotations))
	}
}

func TestAutoMirrorAnnotationsContainsAllKeys(t *testing.T) {
	annotations := AutoMirrorAnnotations("default/secret", "v1")

	expectedKeys := []string{
		dto.AnnotationReflects,
		dto.AnnotationReflectedVersion,
		dto.AnnotationReflectedAt,
		dto.AnnotationAutoReflects,
	}

	for _, key := range expectedKeys {
		if _, ok := annotations[key]; !ok {
			t.Errorf("expected annotation key %s to be present", key)
		}
	}

	if len(annotations) != len(expectedKeys) {
		t.Errorf("expected %d annotations, got %d", len(expectedKeys), len(annotations))
	}
}

func TestReflectedAtIsRecent(t *testing.T) {
	annotations := MirrorAnnotations("default/secret", "v1")

	reflectedAtStr := annotations[dto.AnnotationReflectedAt]
	reflectedAt, err := time.Parse(time.RFC3339, reflectedAtStr)
	if err != nil {
		t.Fatalf("failed to parse reflected-at: %v", err)
	}

	if time.Since(reflectedAt) > time.Second {
		t.Error("reflected-at timestamp should be within the last second")
	}
}

func TestSourceAnnotationsNamespaceHandling(t *testing.T) {
	annotations := SourceAnnotations(true, "ns-1,ns-2,ns-3", true, "ns-4,ns-5")

	allowedNS := annotations[dto.AnnotationReplicationAllowedNamespaces]
	if !strings.Contains(allowedNS, "ns-1") {
		t.Error("expected allowed-namespaces to contain ns-1")
	}
	if !strings.Contains(allowedNS, "ns-2") {
		t.Error("expected allowed-namespaces to contain ns-2")
	}
	if !strings.Contains(allowedNS, "ns-3") {
		t.Error("expected allowed-namespaces to contain ns-3")
	}

	autoNS := annotations[dto.AnnotationReplicationAutoNamespaces]
	if !strings.Contains(autoNS, "ns-4") {
		t.Error("expected auto-namespaces to contain ns-4")
	}
	if !strings.Contains(autoNS, "ns-5") {
		t.Error("expected auto-namespaces to contain ns-5")
	}
}
