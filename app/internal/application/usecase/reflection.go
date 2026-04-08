package usecase

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"replikator/internal/application/dto"
	"replikator/internal/application/port"
	"replikator/internal/domain/entity"
	"replikator/internal/domain/service"
	"replikator/internal/domain/valueobject"
	"replikator/internal/metrics"
	"replikator/pkg/annotations"
	"replikator/pkg/logger"
)

type ReflectResourcesUseCase struct {
	reflectionService *service.ReflectionService
	sourceStore       port.SourceStore
	mirrorRepo        port.MirrorRepository
	namespaceLister   NamespaceLister
	logger            logger.Logger
}

type NamespaceLister interface {
	List(ctx context.Context) ([]string, error)
}

func NewReflectResourcesUseCase(
	reflectionService *service.ReflectionService,
	sourceStore port.SourceStore,
	mirrorRepo port.MirrorRepository,
	namespaceLister NamespaceLister,
	logger logger.Logger,
) *ReflectResourcesUseCase {
	return &ReflectResourcesUseCase{
		reflectionService: reflectionService,
		sourceStore:       sourceStore,
		mirrorRepo:        mirrorRepo,
		namespaceLister:   namespaceLister,
		logger:            logger,
	}
}

func (uc *ReflectResourcesUseCase) Execute(ctx context.Context, source *entity.Source) error {
	if err := uc.reflectionService.ValidateSource(source); err != nil {
		return err
	}

	if !source.IsAllowed() {
		uc.cleanupAutoMirrorsWhenDisabled(ctx, source)
		return nil
	}

	if source.IsAutoEnabled() {
		if err := uc.handleAutoMirrors(ctx, source); err != nil {
			return err
		}
	}

	uc.cleanupAutoMirrorsWhenNoLongerAllowed(ctx, source)

	mirrors, err := uc.mirrorRepo.GetBySourceID(ctx, source.ID())
	if err != nil {
		return fmt.Errorf("failed to get mirrors for source %s: %w", source.ID(), err)
	}

	for _, mirror := range mirrors {
		if err := uc.reflectToMirror(ctx, source, mirror); err != nil {
			continue
		}
	}

	return nil
}

func (uc *ReflectResourcesUseCase) handleAutoMirrors(ctx context.Context, source *entity.Source) error {
	namespaces, err := uc.namespaceLister.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	for _, ns := range namespaces {
		if ns == source.Namespace() {
			continue
		}

		if !source.CanAutoMirrorToNamespace(ns) {
			continue
		}

		if err := uc.createAutoMirrorIfNotExists(ctx, source, ns); err != nil {
			uc.logger.Warn("Failed to create auto mirror",
				"namespace", ns,
				"source", source.FullName(),
				"error", err)
			continue
		}
	}

	return nil
}

func (uc *ReflectResourcesUseCase) createAutoMirrorIfNotExists(ctx context.Context, source *entity.Source, targetNamespace string) error {
	_, err := uc.mirrorRepo.Get(ctx, targetNamespace, source.TargetName())
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing mirror: %w", err)
	}

	if err := uc.checkForConflicts(ctx, source, targetNamespace); err != nil {
		uc.logger.Warn("Skipping auto mirror creation due to conflict",
			"namespace", targetNamespace,
			"source", source.FullName(),
			"error", err)
		return nil
	}

	mirror := entity.NewAutoMirror(
		valueobject.NewMirrorID(targetNamespace, source.TargetName()),
		source.ID(),
		targetNamespace,
		source.TargetName(),
		source.ResourceType(),
	)

	if err := uc.mirrorRepo.Create(ctx, mirror); err != nil {
		metrics.RecordReflectionError(source.Namespace(), source.TargetName(), targetNamespace, "mirror_create_failed")
		return fmt.Errorf("failed to create mirror: %w", err)
	}

	metrics.RecordAutoMirrorCreated(targetNamespace)
	uc.logger.Info("Created auto mirror",
		"mirror", mirror.FullName(),
		"source", source.FullName())

	return nil
}

func (uc *ReflectResourcesUseCase) checkForConflicts(ctx context.Context, source *entity.Source, targetNamespace string) error {
	switch source.ResourceType() {
	case entity.ResourceTypeSecret:
		secret, err := uc.sourceStore.GetSecret(ctx, targetNamespace, source.TargetName())
		if err == nil {
			if dto.IsHelmSecret(secret) {
				uc.logger.Debug("Skipping Helm secret in target namespace", "namespace", targetNamespace, "name", source.TargetName())
				return nil
			}
			return fmt.Errorf("resource %s/%s already exists", targetNamespace, source.TargetName())
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for conflicts: %w", err)
		}
	case entity.ResourceTypeConfigMap:
		cm, err := uc.sourceStore.GetConfigMap(ctx, targetNamespace, source.TargetName())
		if err == nil {
			if dto.IsHelmConfigMap(cm) {
				uc.logger.Debug("Skipping Helm configmap in target namespace", "namespace", targetNamespace, "name", source.TargetName())
				return nil
			}
			return fmt.Errorf("resource %s/%s already exists", targetNamespace, source.TargetName())
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for conflicts: %w", err)
		}
	}
	return nil
}

func (uc *ReflectResourcesUseCase) cleanupAutoMirrorsWhenDisabled(ctx context.Context, source *entity.Source) {
	mirrors, err := uc.mirrorRepo.GetBySourceID(ctx, source.ID())
	if err != nil {
		return
	}

	for _, mirror := range mirrors {
		uc.logger.Info("Deleting auto mirror because source is disabled",
			"mirror", mirror.FullName(),
			"source", source.FullName())
		_ = uc.mirrorRepo.Delete(ctx, mirror.ID())
	}
}

func (uc *ReflectResourcesUseCase) cleanupAutoMirrorsWhenNoLongerAllowed(ctx context.Context, source *entity.Source) {
	mirrors, err := uc.mirrorRepo.GetBySourceID(ctx, source.ID())
	if err != nil {
		return
	}

	for _, mirror := range mirrors {
		if !source.CanAutoMirrorToNamespace(mirror.Namespace()) {
			uc.logger.Info("Deleting auto mirror because namespace is no longer allowed",
				"mirror", mirror.FullName(),
				"source", source.FullName())
			_ = uc.mirrorRepo.Delete(ctx, mirror.ID())
		}
	}
}

func (uc *ReflectResourcesUseCase) reflectToMirror(ctx context.Context, source *entity.Source, mirror *entity.Mirror) error {
	start := time.Now()
	var sourceData map[string][]byte
	var err error
	var sourceSecretType corev1.SecretType

	switch source.ResourceType() {
	case entity.ResourceTypeSecret:
		var secret *corev1.Secret
		secret, err = uc.sourceStore.GetSecret(ctx, source.Namespace(), source.Name())
		if err != nil {
			metrics.RecordReflectionError(source.Namespace(), source.Name(), mirror.Namespace(), "source_not_found")
			return err
		}
		sourceData = secret.Data
		sourceSecretType = secret.Type

	case entity.ResourceTypeConfigMap:
		var cm *corev1.ConfigMap
		cm, err = uc.sourceStore.GetConfigMap(ctx, source.Namespace(), source.Name())
		if err != nil {
			metrics.RecordReflectionError(source.Namespace(), source.Name(), mirror.Namespace(), "source_not_found")
			return err
		}
		sourceData = make(map[string][]byte)
		for k, v := range cm.Data {
			sourceData[k] = []byte(v)
		}
		for k, v := range cm.BinaryData {
			sourceData[k] = v
		}
	}

	if err := uc.reflectionService.Reflect(source, mirror, sourceData); err != nil {
		metrics.RecordReflectionError(source.Namespace(), source.Name(), mirror.Namespace(), "reflection_validation_failed")
		return err
	}

	if err := uc.updateMirrorData(ctx, source, mirror, sourceData, sourceSecretType); err != nil {
		metrics.RecordReflectionError(source.Namespace(), source.Name(), mirror.Namespace(), "update_failed")
		return err
	}

	metrics.RecordReflectionSuccess(source.Namespace(), source.Name(), mirror.Namespace(), time.Since(start))
	return nil
}

func (uc *ReflectResourcesUseCase) updateMirrorData(ctx context.Context, source *entity.Source, mirror *entity.Mirror, sourceData map[string][]byte, sourceSecretType corev1.SecretType) error {
	annotationsMap := annotations.MirrorAnnotations(source.ID().String(), source.Version())
	if mirror.IsAutoCreated() {
		annotationsMap = annotations.AutoMirrorAnnotations(source.ID().String(), source.Version())
	}

	switch mirror.ResourceType() {
	case entity.ResourceTypeSecret:
		secret, err := uc.sourceStore.GetSecret(ctx, mirror.Namespace(), mirror.Name())
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		if secret == nil {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        mirror.Name(),
					Namespace:   mirror.Namespace(),
					Annotations: annotationsMap,
				},
				Data: sourceData,
				Type: sourceSecretType,
			}
			return uc.sourceStore.CreateSecret(ctx, secret)
		}

		secret.Data = sourceData
		secret.Annotations = annotationsMap
		secret.Type = sourceSecretType
		return uc.sourceStore.UpdateSecret(ctx, secret)

	case entity.ResourceTypeConfigMap:
		cm, err := uc.sourceStore.GetConfigMap(ctx, mirror.Namespace(), mirror.Name())
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		data := make(map[string]string)
		binaryData := make(map[string][]byte)
		for k, v := range sourceData {
			if len(v) > 0 && !isPrintable(v) {
				binaryData[k] = v
			} else {
				data[k] = string(v)
			}
		}

		if cm == nil {
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        mirror.Name(),
					Namespace:   mirror.Namespace(),
					Annotations: annotationsMap,
				},
				Data:       data,
				BinaryData: binaryData,
			}
			return uc.sourceStore.CreateConfigMap(ctx, cm)
		}

		cm.Data = data
		cm.BinaryData = binaryData
		cm.Annotations = annotationsMap
		return uc.sourceStore.UpdateConfigMap(ctx, cm)
	}

	return nil
}

func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}

type HandleSourceDeletionUseCase struct {
	mirrorRepo     port.MirrorRepository
	sourceStore    port.SourceStore
	deleteOrphaned bool
}

func NewHandleSourceDeletionUseCase(
	mirrorRepo port.MirrorRepository,
	sourceStore port.SourceStore,
	deleteOrphaned bool,
) *HandleSourceDeletionUseCase {
	return &HandleSourceDeletionUseCase{
		mirrorRepo:     mirrorRepo,
		sourceStore:    sourceStore,
		deleteOrphaned: deleteOrphaned,
	}
}

func (uc *HandleSourceDeletionUseCase) Execute(ctx context.Context, sourceID valueobject.SourceID, resourceType entity.ResourceType) error {
	mirrors, err := uc.mirrorRepo.GetBySourceID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get mirrors: %w", err)
	}

	for _, mirror := range mirrors {
		if uc.deleteOrphaned {
			switch resourceType {
			case entity.ResourceTypeSecret:
				_ = uc.sourceStore.DeleteSecret(ctx, mirror.Namespace(), mirror.Name())
			case entity.ResourceTypeConfigMap:
				_ = uc.sourceStore.DeleteConfigMap(ctx, mirror.Namespace(), mirror.Name())
			}
		}

		if err := uc.mirrorRepo.Delete(ctx, mirror.ID()); err != nil {
			continue
		}
	}

	return nil
}

type CreateMirrorUseCase struct {
	mirrorRepo        port.MirrorRepository
	reflectionService *service.ReflectionService
}

func NewCreateMirrorUseCase(
	mirrorRepo port.MirrorRepository,
	reflectionService *service.ReflectionService,
) *CreateMirrorUseCase {
	return &CreateMirrorUseCase{
		mirrorRepo:        mirrorRepo,
		reflectionService: reflectionService,
	}
}

func (uc *CreateMirrorUseCase) Execute(ctx context.Context, sourceID valueobject.SourceID, targetNamespace, targetName string, resourceType entity.ResourceType) error {
	mirror := entity.NewMirror(
		valueobject.NewMirrorID(targetNamespace, targetName),
		sourceID,
		targetNamespace,
		targetName,
		resourceType,
	)

	return uc.mirrorRepo.Create(ctx, mirror)
}

type DeleteMirrorUseCase struct {
	mirrorRepo     port.MirrorRepository
	sourceStore    port.SourceStore
	deleteOrphaned bool
}

func NewDeleteMirrorUseCase(
	mirrorRepo port.MirrorRepository,
	sourceStore port.SourceStore,
	deleteOrphaned bool,
) *DeleteMirrorUseCase {
	return &DeleteMirrorUseCase{
		mirrorRepo:     mirrorRepo,
		sourceStore:    sourceStore,
		deleteOrphaned: deleteOrphaned,
	}
}

func (uc *DeleteMirrorUseCase) Execute(ctx context.Context, mirrorID valueobject.MirrorID, resourceType entity.ResourceType) error {
	if uc.deleteOrphaned {
		switch resourceType {
		case entity.ResourceTypeSecret:
			_ = uc.sourceStore.DeleteSecret(ctx, mirrorID.Namespace(), mirrorID.Name())
		case entity.ResourceTypeConfigMap:
			_ = uc.sourceStore.DeleteConfigMap(ctx, mirrorID.Namespace(), mirrorID.Name())
		}
	}

	return uc.mirrorRepo.Delete(ctx, mirrorID)
}
