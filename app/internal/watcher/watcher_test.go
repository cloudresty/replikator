package watcher

import (
	"testing"
	"time"
)

func TestWatchSession(t *testing.T) {
	session := NewWatchSession(time.Hour, nil)

	if session.IsRunning() {
		t.Error("expected session to not be running initially")
	}

	session.Start()

	if !session.IsRunning() {
		t.Error("expected session to be running after Start")
	}

	if session.IsExpired() {
		t.Error("expected session to not be expired immediately after Start")
	}

	session.Stop()

	if session.IsRunning() {
		t.Error("expected session to not be running after Stop")
	}
}

func TestWatchSessionExpiry(t *testing.T) {
	session := NewWatchSession(time.Millisecond*100, nil)

	session.Start()

	if session.IsExpired() {
		t.Error("expected session to not be expired immediately after Start")
	}

	time.Sleep(time.Millisecond * 150)

	if !session.IsExpired() {
		t.Error("expected session to be expired after timeout")
	}
}

func TestWatchSessionRemainingTime(t *testing.T) {
	session := NewWatchSession(time.Hour, nil)

	session.Start()
	defer session.Stop()

	remaining := session.RemainingTime()
	if remaining <= 0 {
		t.Error("expected remaining time to be positive")
	}

	if remaining > time.Hour {
		t.Error("expected remaining time to be less than initial timeout")
	}
}

func TestWatchSessionRemainingTimeExpired(t *testing.T) {
	session := NewWatchSession(time.Millisecond*100, nil)

	session.Start()

	time.Sleep(time.Millisecond * 150)

	if session.RemainingTime() != 0 {
		t.Error("expected remaining time to be 0 for expired session")
	}
}

func TestSessionManager(t *testing.T) {
	manager := NewSessionManager(time.Hour, nil)

	if manager.IsSessionValid() {
		t.Error("expected no valid session initially")
	}

	session := manager.StartSession()

	if !manager.IsSessionValid() {
		t.Error("expected session to be valid after StartSession")
	}

	if session == nil {
		t.Error("expected session to be returned from StartSession")
	}

	manager.EndSession()

	if manager.IsSessionValid() {
		t.Error("expected session to not be valid after EndSession")
	}
}

func TestSessionManagerRestartSession(t *testing.T) {
	manager := NewSessionManager(time.Hour, nil)

	session1 := manager.StartSession()
	session1.Start()

	time.Sleep(time.Millisecond)

	session2 := manager.StartSession()

	if session1 == session2 {
		t.Error("expected new session to be different from old session")
	}
}

func TestSessionManagerOnSessionEnd(t *testing.T) {
	manager := NewSessionManager(time.Hour, nil)

	called := false
	manager.OnSessionEnd(func() {
		called = true
	})

	manager.StartSession()
	manager.EndSession()

	if !called {
		t.Error("expected session end handler to be called")
	}
}

func TestEventChannel(t *testing.T) {
	ch := NewEventChannel(10)

	event := WatchEvent{
		Type: EventAdd,
		Key:  "default/secret",
	}

	if !ch.Send(event) {
		t.Error("expected Send to return true")
	}

	if ch.Pending() != 1 {
		t.Error("expected 1 pending event")
	}

	received, ok := ch.Receive()
	if !ok {
		t.Error("expected to receive event")
	}

	if received.Key != event.Key {
		t.Errorf("expected key %s, got %s", event.Key, received.Key)
	}

	if ch.Pending() != 0 {
		t.Error("expected no pending events after Receive")
	}
}

func TestEventChannelFull(t *testing.T) {
	ch := NewEventChannel(1)

	event := WatchEvent{Type: EventAdd, Key: "key1"}

	if !ch.Send(event) {
		t.Error("expected first Send to succeed")
	}

	event2 := WatchEvent{Type: EventAdd, Key: "key2"}
	if ch.Send(event2) {
		t.Error("expected second Send to fail (buffer full)")
	}

	received, ok := ch.Receive()
	if !ok {
		t.Error("expected to receive event")
	}

	if received.Key != event.Key {
		t.Errorf("expected key %s, got %s", event.Key, received.Key)
	}
}

func TestEventChannelClose(t *testing.T) {
	ch := NewEventChannel(10)

	ch.Close()

	if ch.Send(WatchEvent{Type: EventAdd, Key: "key"}) {
		t.Error("expected Send to fail after Close")
	}
}
