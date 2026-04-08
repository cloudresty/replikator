package event

import (
	"sync"

	"replikator/pkg/logger"
)

type InMemoryDispatcher struct {
	handlers map[DomainEventType][]EventHandler
	mu       sync.RWMutex
	log      logger.Logger
}

type EventHandler func(event DomainEvent) error

func NewInMemoryDispatcher() *InMemoryDispatcher {
	return &InMemoryDispatcher{
		handlers: make(map[DomainEventType][]EventHandler),
		log:      nil,
	}
}

func (d *InMemoryDispatcher) Dispatch(event DomainEvent) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	handlers, ok := d.handlers[event.EventType()]
	if !ok {
		return nil
	}

	for _, handler := range handlers {
		if err := handler(event); err != nil {
			return err
		}
	}

	return nil
}

func (d *InMemoryDispatcher) RegisterHandler(eventType DomainEventType, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.handlers[eventType] = append(d.handlers[eventType], handler)
}
