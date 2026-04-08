package federation

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"replikator/internal/metrics"
	"replikator/pkg/logger"
)

type CrossClusterSyncService struct {
	clusterManager *RemoteClusterManager
	localClient    client.Client
	log            logger.Logger
}

func NewCrossClusterSyncService(clusterManager *RemoteClusterManager, localClient client.Client, log logger.Logger) *CrossClusterSyncService {
	return &CrossClusterSyncService{
		clusterManager: clusterManager,
		localClient:    localClient,
		log:            log,
	}
}

func (s *CrossClusterSyncService) SyncSecretToCluster(ctx context.Context, clusterName string, secret *corev1.Secret, targetNamespace string) error {
	clusterClient, ok := s.clusterManager.GetClient(clusterName)
	if !ok {
		return fmt.Errorf("cluster %s not available", clusterName)
	}

	start := time.Now()
	existing, err := clusterClient.CoreV1().Secrets(targetNamespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err == nil {
		existing.Data = secret.Data
		existing.Type = secret.Type
		existing.Labels = secret.Labels
		existing.Annotations = secret.Annotations
		_, err = clusterClient.CoreV1().Secrets(targetNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		newSecret := secret.DeepCopy()
		newSecret.Namespace = targetNamespace
		newSecret.ResourceVersion = ""
		newSecret.UID = ""
		_, err = clusterClient.CoreV1().Secrets(targetNamespace).Create(ctx, newSecret, metav1.CreateOptions{})
	}
	if err != nil {
		metrics.RecordReflectionError(secret.Namespace, secret.Name, targetNamespace, "cross_cluster_sync_failed")
		return fmt.Errorf("failed to sync secret to cluster %s: %w", clusterName, err)
	}

	s.clusterManager.UpdateLastSync(clusterName)
	metrics.RecordReflectionSuccess(secret.Namespace, secret.Name, fmt.Sprintf("%s:%s", clusterName, targetNamespace), time.Since(start))
	return nil
}

func (s *CrossClusterSyncService) SyncConfigMapToCluster(ctx context.Context, clusterName string, cm *corev1.ConfigMap, targetNamespace string) error {
	clusterClient, ok := s.clusterManager.GetClient(clusterName)
	if !ok {
		return fmt.Errorf("cluster %s not available", clusterName)
	}

	start := time.Now()
	existingCM, err := clusterClient.CoreV1().ConfigMaps(targetNamespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if err == nil {
		existingCM.Data = cm.Data
		existingCM.BinaryData = cm.BinaryData
		existingCM.Labels = cm.Labels
		existingCM.Annotations = cm.Annotations
		_, err = clusterClient.CoreV1().ConfigMaps(targetNamespace).Update(ctx, existingCM, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		newCM := cm.DeepCopy()
		newCM.Namespace = targetNamespace
		newCM.ResourceVersion = ""
		newCM.UID = ""
		_, err = clusterClient.CoreV1().ConfigMaps(targetNamespace).Create(ctx, newCM, metav1.CreateOptions{})
	}
	if err != nil {
		metrics.RecordReflectionError(cm.Namespace, cm.Name, targetNamespace, "cross_cluster_sync_failed")
		return fmt.Errorf("failed to sync configmap to cluster %s: %w", clusterName, err)
	}

	s.clusterManager.UpdateLastSync(clusterName)
	metrics.RecordReflectionSuccess(cm.Namespace, cm.Name, fmt.Sprintf("%s:%s", clusterName, targetNamespace), time.Since(start))
	return nil
}

func (s *CrossClusterSyncService) DeleteSecretFromCluster(ctx context.Context, clusterName, namespace, name string) error {
	clusterClient, ok := s.clusterManager.GetClient(clusterName)
	if !ok {
		return fmt.Errorf("cluster %s not available", clusterName)
	}

	err := clusterClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret from cluster %s: %w", clusterName, err)
	}
	return nil
}

func (s *CrossClusterSyncService) DeleteConfigMapFromCluster(ctx context.Context, clusterName, namespace, name string) error {
	clusterClient, ok := s.clusterManager.GetClient(clusterName)
	if !ok {
		return fmt.Errorf("cluster %s not available", clusterName)
	}

	err := clusterClient.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete configmap from cluster %s: %w", clusterName, err)
	}
	return nil
}

func (s *CrossClusterSyncService) HealthCheck(ctx context.Context) map[string]bool {
	return s.clusterManager.HealthCheck(ctx)
}
