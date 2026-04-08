package watcher

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"

	"replikator/pkg/logger"
)

type EventType string

const (
	EventAdd    EventType = "ADDED"
	EventUpdate EventType = "MODIFIED"
	EventDelete EventType = "DELETED"
)

type WatchEvent struct {
	Type      EventType
	Key       string
	Object    any
	OldObject any
}

type EventHandler interface {
	OnAdd(obj any) error
	OnUpdate(obj, oldObj any) error
	OnDelete(obj any) error
	OnError(err error) error
}

type Informer interface {
	AddEventHandler(handler cache.ResourceEventHandler)
	HasSynced() bool
	Run(stopCh <-chan struct{})
}

type Controller interface {
	Start(ctx context.Context) error
	Stop()
	HasSynced() bool
	IsRunning() bool
}

type WatchSession struct {
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
	stoppedCh    chan struct{}
	timeout      time.Duration
	sessionStart time.Time
	logger       logger.Logger
}

func NewWatchSession(timeout time.Duration, log logger.Logger) *WatchSession {
	return &WatchSession{
		timeout:   timeout,
		logger:    log,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

func (s *WatchSession) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = true
	s.sessionStart = time.Now()
	s.stoppedCh = make(chan struct{})
}

func (s *WatchSession) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
}

func (s *WatchSession) WaitForStop() {
	s.mu.RLock()
	stoppedCh := s.stoppedCh
	s.mu.RUnlock()

	<-stoppedCh
}

func (s *WatchSession) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.running
}

func (s *WatchSession) IsExpired() bool {
	if s.timeout == 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return false
	}
	return time.Since(s.sessionStart) > s.timeout
}

func (s *WatchSession) RemainingTime() time.Duration {
	if s.timeout == 0 {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	elapsed := time.Since(s.sessionStart)
	remaining := s.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

type SessionManager struct {
	mu           sync.RWMutex
	session      *WatchSession
	timeout      time.Duration
	logger       logger.Logger
	onSessionEnd []func()
}

func NewSessionManager(timeout time.Duration, log logger.Logger) *SessionManager {
	return &SessionManager{
		timeout:      timeout,
		logger:       log,
		onSessionEnd: make([]func(), 0),
	}
}

func (m *SessionManager) StartSession() *WatchSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil && m.session.IsRunning() {
		m.session.Stop()
	}

	m.session = NewWatchSession(m.timeout, m.logger)
	m.session.Start()

	return m.session
}

func (m *SessionManager) EndSession() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		m.session.Stop()
		for _, handler := range m.onSessionEnd {
			handler()
		}
	}
}

func (m *SessionManager) OnSessionEnd(handler func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onSessionEnd = append(m.onSessionEnd, handler)
}

func (m *SessionManager) CurrentSession() *WatchSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.session
}

func (m *SessionManager) IsSessionValid() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return false
	}
	return m.session.IsRunning() && !m.session.IsExpired()
}

type EventChannel struct {
	ch   chan WatchEvent
	buf  int
	mu   sync.RWMutex
	done bool
}

func NewEventChannel(bufferSize int) *EventChannel {
	return &EventChannel{
		ch:  make(chan WatchEvent, bufferSize),
		buf: bufferSize,
	}
}

func (ec *EventChannel) Send(event WatchEvent) bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if ec.done {
		return false
	}

	select {
	case ec.ch <- event:
		return true
	default:
		return false
	}
}

func (ec *EventChannel) Receive() (WatchEvent, bool) {
	event, ok := <-ec.ch
	return event, ok
}

func (ec *EventChannel) Close() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.done = true
	close(ec.ch)
}

func (ec *EventChannel) BufferSize() int {
	return ec.buf
}

func (ec *EventChannel) Pending() int {
	return len(ec.ch)
}

type EventDispatcher struct {
	channel  *EventChannel
	handlers []EventHandler
	logger   logger.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewEventDispatcher(bufferSize int, log logger.Logger) *EventDispatcher {
	return &EventDispatcher{
		channel:  NewEventChannel(bufferSize),
		handlers: make([]EventHandler, 0),
		logger:   log,
		stopCh:   make(chan struct{}),
	}
}

func (d *EventDispatcher) AddHandler(handler EventHandler) {
	d.handlers = append(d.handlers, handler)
}

func (d *EventDispatcher) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.processEvents(ctx)
}

func (d *EventDispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

func (d *EventDispatcher) processEvents(ctx context.Context) {
	defer d.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case event, ok := <-d.channel.ch:
			if !ok {
				return
			}
			d.dispatchEvent(event)
		}
	}
}

func (d *EventDispatcher) dispatchEvent(event WatchEvent) {
	for _, handler := range d.handlers {
		var err error
		switch event.Type {
		case EventAdd:
			err = handler.OnAdd(event.Object)
		case EventUpdate:
			err = handler.OnUpdate(event.Object, event.OldObject)
		case EventDelete:
			err = handler.OnDelete(event.Object)
		}

		if err != nil {
			if err := handler.OnError(err); err != nil {
				d.logger.Error("Event handler error", "error", err)
			}
		}
	}
}

type ResourceVersioner interface {
	GetResourceVersion(obj any) (string, error)
}

type DefaultResourceVersioner struct{}

func (v *DefaultResourceVersioner) GetResourceVersion(obj any) (string, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	return m.GetResourceVersion(), nil
}
