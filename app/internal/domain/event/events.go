package event

import (
	"time"

	"replikator/internal/domain/entity"
	"replikator/internal/domain/valueobject"
)

type DomainEventType string

const (
	EventTypeSourceDiscovered    DomainEventType = "SourceDiscovered"
	EventTypeSourceUpdated       DomainEventType = "SourceUpdated"
	EventTypeSourceDeleted       DomainEventType = "SourceDeleted"
	EventTypeMirrorCreated       DomainEventType = "MirrorCreated"
	EventTypeMirrorUpdated       DomainEventType = "MirrorUpdated"
	EventTypeMirrorDeleted       DomainEventType = "MirrorDeleted"
	EventTypeReflectionCompleted DomainEventType = "ReflectionCompleted"
	EventTypeReflectionFailed    DomainEventType = "ReflectionFailed"
)

type DomainEvent interface {
	EventType() DomainEventType
	OccurredAt() time.Time
	AggregateID() string
}

type BaseEvent struct {
	eventType   DomainEventType
	occurredAt  time.Time
	aggregateID string
}

func (e *BaseEvent) EventType() DomainEventType {
	return e.eventType
}

func (e *BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e *BaseEvent) AggregateID() string {
	return e.aggregateID
}

func NewBaseEvent(eventType DomainEventType, aggregateID string) BaseEvent {
	return BaseEvent{
		eventType:   eventType,
		occurredAt:  time.Now().UTC(),
		aggregateID: aggregateID,
	}
}

type SourceDiscoveredEvent struct {
	BaseEvent
	Source *entity.Source
}

func NewSourceDiscoveredEvent(source *entity.Source) *SourceDiscoveredEvent {
	return &SourceDiscoveredEvent{
		BaseEvent: NewBaseEvent(EventTypeSourceDiscovered, source.ID().String()),
		Source:    source,
	}
}

type SourceUpdatedEvent struct {
	BaseEvent
	Source *entity.Source
}

func NewSourceUpdatedEvent(source *entity.Source) *SourceUpdatedEvent {
	return &SourceUpdatedEvent{
		BaseEvent: NewBaseEvent(EventTypeSourceUpdated, source.ID().String()),
		Source:    source,
	}
}

type SourceDeletedEvent struct {
	BaseEvent
	SourceID valueobject.SourceID
}

func NewSourceDeletedEvent(sourceID valueobject.SourceID) *SourceDeletedEvent {
	return &SourceDeletedEvent{
		BaseEvent: NewBaseEvent(EventTypeSourceDeleted, sourceID.String()),
		SourceID:  sourceID,
	}
}

type MirrorCreatedEvent struct {
	BaseEvent
	Mirror *entity.Mirror
}

func NewMirrorCreatedEvent(mirror *entity.Mirror) *MirrorCreatedEvent {
	return &MirrorCreatedEvent{
		BaseEvent: NewBaseEvent(EventTypeMirrorCreated, mirror.ID().String()),
		Mirror:    mirror,
	}
}

type MirrorUpdatedEvent struct {
	BaseEvent
	Mirror *entity.Mirror
}

func NewMirrorUpdatedEvent(mirror *entity.Mirror) *MirrorUpdatedEvent {
	return &MirrorUpdatedEvent{
		BaseEvent: NewBaseEvent(EventTypeMirrorUpdated, mirror.ID().String()),
		Mirror:    mirror,
	}
}

type MirrorDeletedEvent struct {
	BaseEvent
	MirrorID valueobject.MirrorID
}

func NewMirrorDeletedEvent(mirrorID valueobject.MirrorID) *MirrorDeletedEvent {
	return &MirrorDeletedEvent{
		BaseEvent: NewBaseEvent(EventTypeMirrorDeleted, mirrorID.String()),
		MirrorID:  mirrorID,
	}
}

type ReflectionCompletedEvent struct {
	BaseEvent
	SourceID      valueobject.SourceID
	MirrorID      valueobject.MirrorID
	SourceVersion string
}

func NewReflectionCompletedEvent(sourceID valueobject.SourceID, mirrorID valueobject.MirrorID, sourceVersion string) *ReflectionCompletedEvent {
	return &ReflectionCompletedEvent{
		BaseEvent:     NewBaseEvent(EventTypeReflectionCompleted, sourceID.String()),
		SourceID:      sourceID,
		MirrorID:      mirrorID,
		SourceVersion: sourceVersion,
	}
}

type ReflectionFailedEvent struct {
	BaseEvent
	SourceID valueobject.SourceID
	MirrorID valueobject.MirrorID
	Error    string
}

func NewReflectionFailedEvent(sourceID valueobject.SourceID, mirrorID valueobject.MirrorID, err error) *ReflectionFailedEvent {
	return &ReflectionFailedEvent{
		BaseEvent: NewBaseEvent(EventTypeReflectionFailed, sourceID.String()),
		SourceID:  sourceID,
		MirrorID:  mirrorID,
		Error:     err.Error(),
	}
}
