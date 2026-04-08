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

	targetSecret := &corev1.Secret{}
	_, err := clusterClient.CoreV1().Secrets(targetNamespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err == nil {
		targetSecret.Data = secret.Data
		targetSecret.Type = secret.Type
		targetSecret.Labels = secret.Labels
		targetSecret.Annotations = secret.Annotations
		targetSecret.Namespace = targetNamespace
		_, err = clusterClient.CoreV1().Secrets(targetNamespace).Update(ctx, targetSecret, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		targetSecret = secret.DeepCopy()
		targetSecret.Namespace = targetNamespace
		targetSecret.ResourceVersion = ""
		targetSecret.UID = ""
		_, err = clusterClient.CoreV1().Secrets(targetNamespace).Create(ctx, targetSecret, metav1.CreateOptions{})
	}
	if err != nil {
		metrics.RecordReflectionError(secret.Namespace, secret.Name, targetNamespace, "cross_cluster_sync_failed")
		return fmt.Errorf("failed to sync secret to cluster %s: %w", clusterName, err)
	}

	s.clusterManager.UpdateLastSync(clusterName)
	metrics.RecordReflectionSuccess(secret.Namespace, secret.Name, fmt.Sprintf("%s:%s", clusterName, targetNamespace), time.Since(time.Now()))
	return nil
}

func (s *CrossClusterSyncService) SyncConfigMapToCluster(ctx context.Context, clusterName string, cm *corev1.ConfigMap, targetNamespace string) error {
	clusterClient, ok := s.clusterManager.GetClient(clusterName)
	if !ok {
		return fmt.Errorf("cluster %s not available", clusterName)
	}

	targetCM := &corev1.ConfigMap{}
	_, err := clusterClient.CoreV1().ConfigMaps(targetNamespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if err == nil {
		targetCM.Data = cm.Data
		targetCM.BinaryData = cm.BinaryData
		targetCM.Labels = cm.Labels
		targetCM.Annotations = cm.Annotations
		targetCM.Namespace = targetNamespace
		_, err = clusterClient.CoreV1().ConfigMaps(targetNamespace).Update(ctx, targetCM, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		targetCM = cm.DeepCopy()
		targetCM.Namespace = targetNamespace
		targetCM.ResourceVersion = ""
		targetCM.UID = ""
		_, err = clusterClient.CoreV1().ConfigMaps(targetNamespace).Create(ctx, targetCM, metav1.CreateOptions{})
	}
	if err != nil {
		metrics.RecordReflectionError(cm.Namespace, cm.Name, targetNamespace, "cross_cluster_sync_failed")
		return fmt.Errorf("failed to sync configmap to cluster %s: %w", clusterName, err)
	}

	s.clusterManager.UpdateLastSync(clusterName)
	metrics.RecordReflectionSuccess(cm.Namespace, cm.Name, fmt.Sprintf("%s:%s", clusterName, targetNamespace), time.Since(time.Now()))
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
