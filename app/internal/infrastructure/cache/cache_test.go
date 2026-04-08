package infrastructure

import (
	"testing"
	"time"
)

func TestNotFoundCache(t *testing.T) {
	cache := NewNotFoundCache(time.Hour)

	key := "default/secret"

	if cache.IsNotFound(key) {
		t.Error("expected key to not be marked as not found initially")
	}

	cache.MarkNotFound(key)

	if !cache.IsNotFound(key) {
		t.Error("expected key to be marked as not found after MarkNotFound")
	}

	cache.MarkFound(key)

	if cache.IsNotFound(key) {
		t.Error("expected key to not be marked as not found after MarkFound")
	}
}

func TestNotFoundCacheExpiry(t *testing.T) {
	cache := NewNotFoundCache(time.Millisecond * 100)

	key := "default/secret"

	cache.MarkNotFound(key)

	if !cache.IsNotFound(key) {
		t.Error("expected key to be marked as not found immediately after MarkNotFound")
	}

	time.Sleep(time.Millisecond * 150)

	if cache.IsNotFound(key) {
		t.Error("expected key to not be marked as not found after expiry")
	}
}

func TestMirrorCache(t *testing.T) {
	cache := NewMirrorCache()

	entry := &MirrorEntry{
		MirrorID:  "ns1/secret1",
		SourceID:  "default/secret1",
		IsAuto:    true,
		CreatedAt: time.Now(),
	}

	cache.Set(entry.MirrorID, entry)

	got, ok := cache.Get(entry.MirrorID)
	if !ok {
		t.Error("expected to get entry from cache")
	}

	if got.MirrorID != entry.MirrorID {
		t.Errorf("expected MirrorID %s, got %s", entry.MirrorID, got.MirrorID)
	}

	if got.SourceID != entry.SourceID {
		t.Errorf("expected SourceID %s, got %s", entry.SourceID, got.SourceID)
	}

	if got.IsAuto != entry.IsAuto {
		t.Errorf("expected IsAuto %v, got %v", entry.IsAuto, got.IsAuto)
	}
}

func TestMirrorCacheDelete(t *testing.T) {
	cache := NewMirrorCache()

	entry := &MirrorEntry{
		MirrorID:  "ns1/secret1",
		SourceID:  "default/secret1",
		IsAuto:    true,
		CreatedAt: time.Now(),
	}

	cache.Set(entry.MirrorID, entry)
	cache.Delete(entry.MirrorID)

	if _, ok := cache.Get(entry.MirrorID); ok {
		t.Error("expected entry to be deleted from cache")
	}
}

func TestMirrorCacheListBySource(t *testing.T) {
	cache := NewMirrorCache()

	sourceID := "default/secret1"

	cache.Set("ns1/secret1", &MirrorEntry{
		MirrorID: "ns1/secret1",
		SourceID: sourceID,
		IsAuto:   true,
	})

	cache.Set("ns2/secret1", &MirrorEntry{
		MirrorID: "ns2/secret1",
		SourceID: sourceID,
		IsAuto:   false,
	})

	cache.Set("ns3/secret2", &MirrorEntry{
		MirrorID: "ns3/secret2",
		SourceID: "default/secret2",
		IsAuto:   true,
	})

	entries := cache.ListBySource(sourceID)

	if len(entries) != 2 {
		t.Errorf("expected 2 entries for source %s, got %d", sourceID, len(entries))
	}
}

func TestMirrorCacheDeleteBySource(t *testing.T) {
	cache := NewMirrorCache()

	sourceID := "default/secret1"

	cache.Set("ns1/secret1", &MirrorEntry{
		MirrorID: "ns1/secret1",
		SourceID: sourceID,
		IsAuto:   true,
	})

	cache.Set("ns2/secret1", &MirrorEntry{
		MirrorID: "ns2/secret1",
		SourceID: sourceID,
		IsAuto:   false,
	})

	cache.Set("ns3/secret2", &MirrorEntry{
		MirrorID: "ns3/secret2",
		SourceID: "default/secret2",
		IsAuto:   true,
	})

	deleted := cache.DeleteBySource(sourceID)

	if len(deleted) != 2 {
		t.Errorf("expected 2 deleted entries, got %d", len(deleted))
	}

	if _, ok := cache.Get("ns1/secret1"); ok {
		t.Error("expected ns1/secret1 to be deleted")
	}

	if _, ok := cache.Get("ns2/secret1"); ok {
		t.Error("expected ns2/secret1 to be deleted")
	}

	if _, ok := cache.Get("ns3/secret2"); !ok {
		t.Error("expected ns3/secret2 to still exist")
	}
}

func TestMirrorCacheDeleteByNamespace(t *testing.T) {
	cache := NewMirrorCache()

	cache.Set("ns1/secret1", &MirrorEntry{
		MirrorID: "ns1/secret1",
		SourceID: "default/secret1",
		IsAuto:   true,
	})

	cache.Set("ns1/secret2", &MirrorEntry{
		MirrorID: "ns1/secret2",
		SourceID: "default/secret2",
		IsAuto:   false,
	})

	cache.Set("ns2/secret1", &MirrorEntry{
		MirrorID: "ns2/secret1",
		SourceID: "default/secret1",
		IsAuto:   true,
	})

	deleted := cache.DeleteByNamespace("ns1")

	if len(deleted) != 2 {
		t.Errorf("expected 2 deleted entries, got %d", len(deleted))
	}

	if _, ok := cache.Get("ns1/secret1"); ok {
		t.Error("expected ns1/secret1 to be deleted")
	}

	if _, ok := cache.Get("ns1/secret2"); ok {
		t.Error("expected ns1/secret2 to be deleted")
	}

	if _, ok := cache.Get("ns2/secret1"); !ok {
		t.Error("expected ns2/secret1 to still exist")
	}
}

func TestMirrorCacheClear(t *testing.T) {
	cache := NewMirrorCache()

	cache.Set("ns1/secret1", &MirrorEntry{
		MirrorID: "ns1/secret1",
		SourceID: "default/secret1",
		IsAuto:   true,
	})

	cache.Set("ns2/secret1", &MirrorEntry{
		MirrorID: "ns2/secret1",
		SourceID: "default/secret1",
		IsAuto:   false,
	})

	cache.Clear()

	if _, ok := cache.Get("ns1/secret1"); ok {
		t.Error("expected cache to be cleared")
	}

	if _, ok := cache.Get("ns2/secret1"); ok {
		t.Error("expected cache to be cleared")
	}
}

func TestMirrorCacheUpdateLastSyncAt(t *testing.T) {
	cache := NewMirrorCache()

	entry := &MirrorEntry{
		MirrorID:   "ns1/secret1",
		SourceID:   "default/secret1",
		IsAuto:     true,
		CreatedAt:  time.Now().Add(-time.Hour),
		LastSyncAt: time.Now().Add(-time.Hour),
	}

	cache.Set(entry.MirrorID, entry)

	originalLastSync := entry.LastSyncAt

	time.Sleep(time.Millisecond)

	entry.LastSyncAt = time.Now()
	cache.Set(entry.MirrorID, entry)

	got, _ := cache.Get(entry.MirrorID)
	if !got.LastSyncAt.After(originalLastSync) {
		t.Error("expected LastSyncAt to be updated")
	}
}

func TestPropertiesCache(t *testing.T) {
	cache := NewPropertiesCache(time.Hour)

	key := "default/secret"

	if cache.IsNotFound(key) {
		t.Error("expected key to not be marked as not found initially")
	}

	props := &SourceProperties{
		Allowed:           true,
		AllowedNamespaces: []string{"ns1", "ns2"},
		AutoEnabled:       true,
		AutoNamespaces:    []string{"ns3"},
		Version:           "v1",
	}

	cache.Set(key, props)

	got, ok := cache.Get(key)
	if !ok {
		t.Error("expected to get props from cache")
	}

	if got.Allowed != props.Allowed {
		t.Errorf("expected Allowed %v, got %v", props.Allowed, got.Allowed)
	}

	if cache.IsNotFound(key) {
		t.Error("expected key to not be marked as not found after Set")
	}
}

func TestPropertiesCacheMarkNotFound(t *testing.T) {
	cache := NewPropertiesCache(time.Hour)

	key := "default/secret"

	props := &SourceProperties{
		Allowed: true,
		Version: "v1",
	}

	cache.Set(key, props)
	cache.MarkNotFound(key)

	if _, ok := cache.Get(key); ok {
		t.Error("expected props to be deleted after MarkNotFound")
	}

	if !cache.IsNotFound(key) {
		t.Error("expected key to be marked as not found")
	}
}

func TestPropertiesCacheClear(t *testing.T) {
	cache := NewPropertiesCache(time.Hour)

	cache.Set("key1", &SourceProperties{Allowed: true})
	cache.Set("key2", &SourceProperties{Allowed: false})

	cache.Clear()

	if _, ok := cache.Get("key1"); ok {
		t.Error("expected key1 to be cleared")
	}

	if _, ok := cache.Get("key2"); ok {
		t.Error("expected key2 to be cleared")
	}
}
