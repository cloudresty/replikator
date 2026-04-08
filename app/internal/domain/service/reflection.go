package service

import (
	"replikator/internal/domain/entity"
	"replikator/internal/domain/event"
)

type ReflectionService struct {
	eventDispatcher EventDispatcher
}

type EventDispatcher interface {
	Dispatch(event event.DomainEvent) error
}

func NewReflectionService(dispatcher EventDispatcher) *ReflectionService {
	return &ReflectionService{
		eventDispatcher: dispatcher,
	}
}

func (s *ReflectionService) Reflect(source *entity.Source, mirror *entity.Mirror, sourceData map[string][]byte) error {
	if !source.IsAllowed() {
		return ErrSourceNotAllowed
	}

	if !source.CanReflectToNamespace(mirror.Namespace()) {
		return ErrNamespaceNotAllowed
	}

	if mirror.Version() == source.Version() {
		return ErrNoUpdateNeeded
	}

	mirror.SetVersion(source.Version())

	if err := s.eventDispatcher.Dispatch(event.NewReflectionCompletedEvent(
		source.ID(),
		mirror.ID(),
		source.Version(),
	)); err != nil {
		return err
	}

	return nil
}

func (s *ReflectionService) CanCreateAutoMirror(source *entity.Source, targetNamespace, targetName string) bool {
	if !source.CanAutoMirrorToNamespace(targetNamespace) {
		return false
	}

	if source.Name() != targetName {
		return false
	}

	return true
}

func (s *ReflectionService) ShouldDeleteAutoMirror(source *entity.Source, mirror *entity.Mirror) bool {
	if !mirror.SourceID().Equals(source.ID()) {
		return true
	}

	if !source.IsEnabled() && mirror.IsEnabled() {
		return true
	}

	if source.IsAutoEnabled() && !source.CanAutoMirrorToNamespace(mirror.Namespace()) {
		return true
	}

	return false
}

func (s *ReflectionService) ValidateSource(source *entity.Source) error {
	if source == nil {
		return ErrNilSource
	}
	if source.Name() == "" {
		return ErrEmptySourceName
	}
	if source.Namespace() == "" {
		return ErrEmptySourceNamespace
	}
	return nil
}

func (s *ReflectionService) ValidateMirror(mirror *entity.Mirror) error {
	if mirror == nil {
		return ErrNilMirror
	}
	if mirror.Name() == "" {
		return ErrEmptyMirrorName
	}
	if mirror.Namespace() == "" {
		return ErrEmptyMirrorNamespace
	}
	if mirror.SourceID().Namespace() == "" {
		return ErrMirrorSourceNotSet
	}
	return nil
}

var (
	ErrSourceNotAllowed     = &ReflectionError{Message: "source does not allow reflection"}
	ErrNamespaceNotAllowed  = &ReflectionError{Message: "target namespace is not in allowed list"}
	ErrNoUpdateNeeded       = &ReflectionError{Message: "mirror is already at current version"}
	ErrNilSource            = &ReflectionError{Message: "source cannot be nil"}
	ErrEmptySourceName      = &ReflectionError{Message: "source name cannot be empty"}
	ErrEmptySourceNamespace = &ReflectionError{Message: "source namespace cannot be empty"}
	ErrNilMirror            = &ReflectionError{Message: "mirror cannot be nil"}
	ErrEmptyMirrorName      = &ReflectionError{Message: "mirror name cannot be empty"}
	ErrEmptyMirrorNamespace = &ReflectionError{Message: "mirror namespace cannot be empty"}
	ErrMirrorSourceNotSet   = &ReflectionError{Message: "mirror source is not set"}
)

type ReflectionError struct {
	Message string
}

func (e *ReflectionError) Error() string {
	return e.Message
}
