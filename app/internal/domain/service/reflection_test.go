package service

import (
	"testing"

	"replikator/internal/domain/entity"
	"replikator/internal/domain/event"
	"replikator/internal/domain/valueobject"
)

type mockEventDispatcher struct {
	events []event.DomainEvent
}

func (m *mockEventDispatcher) Dispatch(e event.DomainEvent) error {
	m.events = append(m.events, e)
	return nil
}

func TestReflectionService_ValidateSource(t *testing.T) {
	svc := NewReflectionService(&mockEventDispatcher{})

	tests := []struct {
		name    string
		source  *entity.Source
		wantErr error
	}{
		{
			name:    "nil source",
			source:  nil,
			wantErr: ErrNilSource,
		},
		{
			name: "empty name",
			source: func() *entity.Source {
				s := entity.NewSource("default", "", entity.ResourceTypeSecret)
				return s
			}(),
			wantErr: ErrEmptySourceName,
		},
		{
			name: "empty namespace",
			source: func() *entity.Source {
				s := entity.NewSource("", "my-secret", entity.ResourceTypeSecret)
				return s
			}(),
			wantErr: ErrEmptySourceNamespace,
		},
		{
			name: "valid source",
			source: func() *entity.Source {
				s := entity.NewSource("default", "my-secret", entity.ResourceTypeSecret)
				return s
			}(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateSource(tt.source)
			if err != tt.wantErr {
				t.Errorf("ValidateSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReflectionService_ValidateMirror(t *testing.T) {
	svc := NewReflectionService(&mockEventDispatcher{})

	sourceID := valueobject.NewSourceID("default", "source-secret")
	mirrorID := valueobject.NewMirrorID("namespace-1", "mirror-secret")

	tests := []struct {
		name    string
		mirror  *entity.Mirror
		wantErr error
	}{
		{
			name:    "nil mirror",
			mirror:  nil,
			wantErr: ErrNilMirror,
		},
		{
			name: "empty name",
			mirror: func() *entity.Mirror {
				m := entity.NewMirror(valueobject.NewMirrorID("ns", ""), sourceID, "ns", "", entity.ResourceTypeSecret)
				return m
			}(),
			wantErr: ErrEmptyMirrorName,
		},
		{
			name: "empty namespace",
			mirror: func() *entity.Mirror {
				m := entity.NewMirror(valueobject.NewMirrorID("", "name"), sourceID, "", "name", entity.ResourceTypeSecret)
				return m
			}(),
			wantErr: ErrEmptyMirrorNamespace,
		},
		{
			name: "valid mirror",
			mirror: func() *entity.Mirror {
				return entity.NewMirror(mirrorID, sourceID, "namespace-1", "mirror-secret", entity.ResourceTypeSecret)
			}(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateMirror(tt.mirror)
			if err != tt.wantErr {
				t.Errorf("ValidateMirror() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReflectionService_CanCreateAutoMirror(t *testing.T) {
	svc := NewReflectionService(&mockEventDispatcher{})

	allowedNS, _ := valueobject.NewAllowedNamespaces([]string{"namespace-1", "namespace-2"})
	autoNS, _ := valueobject.NewAllowedNamespaces([]string{"namespace-1"})

	source := entity.NewSource("default", "my-secret", entity.ResourceTypeSecret)
	source.SetAllowed(true)
	source.SetAllowedNamespaces(allowedNS)
	source.SetAutoEnabled(true)
	source.SetAutoNamespaces(autoNS)

	if !svc.CanCreateAutoMirror(source, "namespace-1", "my-secret") {
		t.Error("Expected to be able to create auto mirror to namespace-1 with same name")
	}

	if svc.CanCreateAutoMirror(source, "namespace-2", "my-secret") {
		t.Error("Expected to not be able to create auto mirror to namespace-2 (not in auto namespaces)")
	}

	if svc.CanCreateAutoMirror(source, "namespace-1", "different-name") {
		t.Error("Expected to not be able to create auto mirror with different name")
	}

	source.SetAutoEnabled(false)
	if svc.CanCreateAutoMirror(source, "namespace-1", "my-secret") {
		t.Error("Expected to not be able to create auto mirror when auto is disabled")
	}
}

func TestReflectionService_ReflectionError(t *testing.T) {
	err := &ReflectionError{Message: "test error"}

	if err.Error() != "test error" {
		t.Errorf("Expected error message 'test error', got %q", err.Error())
	}
}
