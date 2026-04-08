package entity

import (
	"testing"

	"replikator/internal/domain/valueobject"
)

func TestSource(t *testing.T) {
	source := NewSource("default", "my-secret", ResourceTypeSecret)

	if source.Namespace() != "default" {
		t.Errorf("Expected namespace 'default', got %q", source.Namespace())
	}

	if source.Name() != "my-secret" {
		t.Errorf("Expected name 'my-secret', got %q", source.Name())
	}

	if source.ResourceType() != ResourceTypeSecret {
		t.Errorf("Expected resource type 'Secret', got %q", source.ResourceType())
	}

	if source.IsAllowed() {
		t.Error("Expected source to not be allowed by default")
	}

	if !source.IsEnabled() {
		t.Error("Expected source to be enabled by default after creation")
	}
}

func TestSource_AllowedNamespaces(t *testing.T) {
	source := NewSource("default", "my-secret", ResourceTypeSecret)

	allowedNS, _ := valueobject.NewAllowedNamespaces([]string{"namespace-1", "namespace-2"})
	source.SetAllowedNamespaces(allowedNS)
	source.SetAllowed(true)

	if !source.CanReflectToNamespace("namespace-1") {
		t.Error("Expected source to be able to reflect to namespace-1")
	}

	if !source.CanReflectToNamespace("namespace-2") {
		t.Error("Expected source to be able to reflect to namespace-2")
	}

	if source.CanReflectToNamespace("namespace-3") {
		t.Error("Expected source to not be able to reflect to namespace-3")
	}
}

func TestSource_AutoMirror(t *testing.T) {
	source := NewSource("default", "my-secret", ResourceTypeSecret)

	allowedNS, _ := valueobject.NewAllowedNamespaces([]string{"*"})
	source.SetAllowedNamespaces(allowedNS)
	source.SetAllowed(true)
	source.SetAutoEnabled(true)

	autoNS, _ := valueobject.NewAllowedNamespaces([]string{"namespace-1", "namespace-2"})
	source.SetAutoNamespaces(autoNS)

	if !source.CanAutoMirrorToNamespace("namespace-1") {
		t.Error("Expected source to be able to auto-mirror to namespace-1")
	}

	if source.CanAutoMirrorToNamespace("namespace-3") {
		t.Error("Expected source to not be able to auto-mirror to namespace-3")
	}
}

func TestMirror(t *testing.T) {
	sourceID := valueobject.NewSourceID("default", "my-secret")
	mirrorID := valueobject.NewMirrorID("namespace-1", "my-secret")

	mirror := NewMirror(mirrorID, sourceID, "namespace-1", "my-secret", ResourceTypeSecret)

	if mirror.Namespace() != "namespace-1" {
		t.Errorf("Expected namespace 'namespace-1', got %q", mirror.Namespace())
	}

	if mirror.Name() != "my-secret" {
		t.Errorf("Expected name 'my-secret', got %q", mirror.Name())
	}

	if mirror.ResourceType() != ResourceTypeSecret {
		t.Errorf("Expected resource type 'Secret', got %q", mirror.ResourceType())
	}

	if !mirror.SourceID().Equals(sourceID) {
		t.Error("Expected SourceID to match")
	}

	if !mirror.IsEnabled() {
		t.Error("Expected mirror to be enabled by default")
	}
}

func TestMirror_Version(t *testing.T) {
	sourceID := valueobject.NewSourceID("default", "my-secret")
	mirrorID := valueobject.NewMirrorID("namespace-1", "my-secret")

	mirror := NewMirror(mirrorID, sourceID, "namespace-1", "my-secret", ResourceTypeSecret)

	if mirror.Version() != "" {
		t.Error("Expected version to be empty by default")
	}

	mirror.SetVersion("12345")

	if mirror.Version() != "12345" {
		t.Errorf("Expected version '12345', got %q", mirror.Version())
	}
}

func TestMirror_EnableDisable(t *testing.T) {
	sourceID := valueobject.NewSourceID("default", "my-secret")
	mirrorID := valueobject.NewMirrorID("namespace-1", "my-secret")

	mirror := NewMirror(mirrorID, sourceID, "namespace-1", "my-secret", ResourceTypeSecret)

	if !mirror.IsEnabled() {
		t.Error("Expected mirror to be enabled by default")
	}

	mirror.Disable()
	if mirror.IsEnabled() {
		t.Error("Expected mirror to be disabled after Disable()")
	}

	mirror.Enable()
	if !mirror.IsEnabled() {
		t.Error("Expected mirror to be enabled after Enable()")
	}
}

func TestResourceTypes(t *testing.T) {
	if ResourceTypeSecret != "Secret" {
		t.Errorf("Expected ResourceTypeSecret to be 'Secret', got %q", ResourceTypeSecret)
	}

	if ResourceTypeConfigMap != "ConfigMap" {
		t.Errorf("Expected ResourceTypeConfigMap to be 'ConfigMap', got %q", ResourceTypeConfigMap)
	}
}

func TestSource_TargetName(t *testing.T) {
	source := NewSource("default", "my-secret", ResourceTypeSecret)

	if source.TargetName() != "my-secret" {
		t.Errorf("Expected TargetName to equal Name when not set, got %q", source.TargetName())
	}

	source.SetTargetName("transformed-secret")
	if source.TargetName() != "transformed-secret" {
		t.Errorf("Expected TargetName to be 'transformed-secret', got %q", source.TargetName())
	}

	if source.Name() != "my-secret" {
		t.Errorf("Expected Name to remain unchanged, got %q", source.Name())
	}
}

func TestSource_TargetName_FallbackToName(t *testing.T) {
	source := NewSource("default", "original-secret", ResourceTypeSecret)

	source.SetTargetName("")
	if source.TargetName() != "original-secret" {
		t.Errorf("Expected TargetName to fallback to Name when empty, got %q", source.TargetName())
	}
}
