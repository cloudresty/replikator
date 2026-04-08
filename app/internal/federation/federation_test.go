package federation_test

import (
	"context"
	"testing"

	"replikator/internal/federation"
)

func TestNewRemoteClusterManager(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)
	if manager == nil {
		t.Error("expected non-nil manager")
	}

	clusters := manager.ListClusters()
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestRemoteClusterManager_GetClient_NotFound(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)

	_, ok := manager.GetClient("non-existent")
	if ok {
		t.Error("expected cluster to not exist")
	}
}

func TestRemoteClusterManager_ListClusters_Empty(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)

	clusters := manager.ListClusters()
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestRemoteClusterManager_RemoveCluster_NotFound(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)

	manager.RemoveCluster("non-existent")
}

func TestRemoteClusterManager_UpdateLastSync_NotFound(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)

	manager.UpdateLastSync("non-existent")
}

func TestRemoteClusterManager_HealthCheck_Empty(t *testing.T) {
	manager := federation.NewRemoteClusterManager(nil)

	results := manager.HealthCheck(context.TODO())
	if len(results) != 0 {
		t.Errorf("expected 0 clusters in health check, got %d", len(results))
	}
}
