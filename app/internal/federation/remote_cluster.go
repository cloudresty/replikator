package federation

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"replikator/pkg/logger"
)

type RemoteClusterManager struct {
	clusters map[string]*ClusterConfig
	log      logger.Logger
}

type ClusterConfig struct {
	Client       kubernetes.Interface
	Kubeconfig   []byte
	LastSyncTime int64
}

func NewRemoteClusterManager(log logger.Logger) *RemoteClusterManager {
	return &RemoteClusterManager{
		clusters: make(map[string]*ClusterConfig),
		log:      log,
	}
}

func (m *RemoteClusterManager) AddCluster(ctx context.Context, name string, kubeconfig []byte) error {
	tmpPath, err := writeTempKubeconfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to write temp kubeconfig: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	cfg, err := clientcmd.BuildConfigFromFlags("", tmpPath)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	cl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	m.clusters[name] = &ClusterConfig{
		Client:     cl,
		Kubeconfig: kubeconfig,
	}

	if m.log != nil {
		m.log.Info("Added remote cluster", "name", name)
	}
	return nil
}

func (m *RemoteClusterManager) GetClient(name string) (kubernetes.Interface, bool) {
	cfg, ok := m.clusters[name]
	if !ok {
		return nil, false
	}
	return cfg.Client, true
}

func (m *RemoteClusterManager) RemoveCluster(name string) {
	delete(m.clusters, name)
	if m.log != nil {
		m.log.Info("Removed remote cluster", "name", name)
	}
}

func (m *RemoteClusterManager) HealthCheck(ctx context.Context) map[string]bool {
	results := make(map[string]bool)
	for name, cfg := range m.clusters {
		results[name] = cfg.Client != nil
	}
	return results
}

func (m *RemoteClusterManager) UpdateLastSync(name string) {
	if cfg, ok := m.clusters[name]; ok {
		cfg.LastSyncTime = time.Now().Unix()
	}
}

func (m *RemoteClusterManager) ListClusters() []string {
	names := make([]string, 0, len(m.clusters))
	for name := range m.clusters {
		names = append(names, name)
	}
	return names
}

func writeTempKubeconfig(kubeconfig []byte) (string, error) {
	tmpfile, err := os.CreateTemp("", "kubeconfig-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpfile.Write(kubeconfig); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpfile.Name())
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpfile.Name(), nil
}
