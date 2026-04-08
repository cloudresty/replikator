package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"replikator/internal/application/dto"
	"replikator/internal/application/usecase"
	"replikator/internal/domain/entity"
	"replikator/internal/domain/service"
	"replikator/internal/domain/valueobject"
	"replikator/internal/infrastructure/adapter/k8s"
	infcache "replikator/internal/infrastructure/cache"
	"replikator/internal/infrastructure/config"
	"replikator/internal/metrics"
	"replikator/internal/watcher"
	"replikator/pkg/logger"
)

type SourceReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	reflectUseCase  *usecase.ReflectResourcesUseCase
	deleteUseCase   *usecase.HandleSourceDeletionUseCase
	reflectionSvc   *service.ReflectionService
	namespaceLister *k8s.NamespaceListerAdapter
	propertiesCache *infcache.PropertiesCache
	notFoundCache   *infcache.NotFoundCache
	log             logger.Logger
	watcherConfig   config.WatcherConfig
}

func NewSourceReconciler(
	c client.Client,
	recorder record.EventRecorder,
	reflectUC *usecase.ReflectResourcesUseCase,
	deleteUC *usecase.HandleSourceDeletionUseCase,
	reflectionSvc *service.ReflectionService,
	namespaceLister *k8s.NamespaceListerAdapter,
	propsCache *infcache.PropertiesCache,
	notFoundCache *infcache.NotFoundCache,
	log logger.Logger,
	watcherCfg config.WatcherConfig,
) *SourceReconciler {
	return &SourceReconciler{
		client:          c,
		recorder:        recorder,
		reflectUseCase:  reflectUC,
		deleteUseCase:   deleteUC,
		reflectionSvc:   reflectionSvc,
		namespaceLister: namespaceLister,
		propertiesCache: propsCache,
		notFoundCache:   notFoundCache,
		log:             log,
		watcherConfig:   watcherCfg,
	}
}

func getResultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func (r *SourceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("namespace", req.Namespace, "name", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("source", getResultLabel(reconcileErr), time.Since(start))
	}()

	cacheKey := fmt.Sprintf("%s/%s", req.Namespace, req.Name)
	if r.notFoundCache.IsNotFound(cacheKey) {
		log.Debug("Source previously not found, skipping")
		return reconcile.Result{}, nil
	}

	source, err := r.fetchSource(ctx, req.Namespace, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Source not found, caching result")
			r.notFoundCache.MarkNotFound(cacheKey)
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to fetch source: %w", err)
	}

	if source == nil {
		log.Debug("Source does not have replication annotations, caching as not a source")
		r.notFoundCache.MarkNotFound(cacheKey)
		return reconcile.Result{}, nil
	}

	r.notFoundCache.MarkFound(cacheKey)

	if err := r.reflectUseCase.Execute(ctx, source); err != nil {
		log.Error("Failed to replicate resources", "error", err)
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to replicate: %w", err)
	}

	log.Debug("Source reconciled successfully")
	return reconcile.Result{}, nil
}

func (r *SourceReconciler) fetchSource(ctx context.Context, namespace, name string) (*entity.Source, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err == nil {
		if dto.IsHelmSecret(secret) {
			r.log.Debug("Skipping Helm secret", "namespace", namespace, "name", name)
			return nil, nil
		}
		return r.secretToSource(secret), nil
	}
	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err == nil {
		if dto.IsHelmConfigMap(cm) {
			r.log.Debug("Skipping Helm configmap", "namespace", namespace, "name", name)
			return nil, nil
		}
		return r.configMapToSource(cm), nil
	}
	if !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get configmap: %w", err)
	}

	return nil, errors.NewNotFound(corev1.Resource("source"), name)
}

func (r *SourceReconciler) secretToSource(secret *corev1.Secret) *entity.Source {
	if secret.Annotations == nil {
		return nil
	}

	allowedStr := secret.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(secret.Namespace, secret.Name, entity.ResourceTypeSecret)

	allowedNamespacesStr := secret.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := secret.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	autoNamespacesStr := secret.Annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(true)
	source.SetVersion(secret.ResourceVersion)

	return source
}

func (r *SourceReconciler) configMapToSource(cm *corev1.ConfigMap) *entity.Source {
	if cm.Annotations == nil {
		return nil
	}

	allowedStr := cm.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(cm.Namespace, cm.Name, entity.ResourceTypeConfigMap)

	allowedNamespacesStr := cm.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := cm.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	autoNamespacesStr := cm.Annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(true)
	source.SetVersion(cm.ResourceVersion)

	return source
}

func (r *SourceReconciler) SetupWithManager(mgr any) error {
	return nil
}

func (r *SourceReconciler) InjectUpdateEventHandler() {
}

type SourcePredicate struct {
	notFoundCache *infcache.NotFoundCache
}

func NewSourcePredicate(cache *infcache.NotFoundCache) *SourcePredicate {
	return &SourcePredicate{notFoundCache: cache}
}

func (p *SourcePredicate) Create(e event.CreateEvent) bool {
	return p.hasReplicationAnnotation(e.Object)
}

func (p *SourcePredicate) Update(e event.UpdateEvent) bool {
	cacheKey := fmt.Sprintf("%s/%s", e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
	p.notFoundCache.MarkFound(cacheKey)
	return p.hasReplicationAnnotation(e.ObjectNew) || p.hasReplicationAnnotation(e.ObjectOld)
}

func (p *SourcePredicate) Delete(e event.DeleteEvent) bool {
	cacheKey := fmt.Sprintf("%s/%s", e.Object.GetNamespace(), e.Object.GetName())
	p.notFoundCache.MarkFound(cacheKey)
	return p.hasReplicationAnnotation(e.Object)
}

func (p *SourcePredicate) Generic(e event.GenericEvent) bool {
	return p.hasReplicationAnnotation(e.Object)
}

func (p *SourcePredicate) hasReplicationAnnotation(obj client.Object) bool {
	if obj == nil {
		return false
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, exists := annotations[dto.AnnotationReplicationAllowed]
	return exists && annotations[dto.AnnotationReplicationAllowed] == "true"
}

type MirrorPredicate struct {
}

func NewMirrorPredicate() *MirrorPredicate {
	return &MirrorPredicate{}
}

func (p *MirrorPredicate) Create(e event.CreateEvent) bool {
	return p.hasReflectsAnnotation(e.Object)
}

func (p *MirrorPredicate) Update(e event.UpdateEvent) bool {
	return p.hasReflectsAnnotation(e.ObjectNew)
}

func (p *MirrorPredicate) Delete(e event.DeleteEvent) bool {
	return p.hasReflectsAnnotation(e.Object)
}

func (p *MirrorPredicate) Generic(e event.GenericEvent) bool {
	return p.hasReflectsAnnotation(e.Object)
}

func (p *MirrorPredicate) hasReflectsAnnotation(obj client.Object) bool {
	if obj == nil {
		return false
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	reflects, exists := annotations[dto.AnnotationReflects]
	return exists && reflects != ""
}

type MirrorReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	createMirrorUC  *usecase.CreateMirrorUseCase
	deleteMirrorUC  *usecase.DeleteMirrorUseCase
	reflectUseCase  *usecase.ReflectResourcesUseCase
	reflectionSvc   *service.ReflectionService
	namespaceLister *k8s.NamespaceListerAdapter
	mirrorStore     *k8s.InMemoryMirrorStore
	mirrorCache     *infcache.MirrorCache
	log             logger.Logger
}

func NewMirrorReconciler(
	c client.Client,
	recorder record.EventRecorder,
	createMirrorUC *usecase.CreateMirrorUseCase,
	deleteMirrorUC *usecase.DeleteMirrorUseCase,
	reflectUC *usecase.ReflectResourcesUseCase,
	reflectionSvc *service.ReflectionService,
	namespaceLister *k8s.NamespaceListerAdapter,
	mirrorStore *k8s.InMemoryMirrorStore,
	mirrorCache *infcache.MirrorCache,
	log logger.Logger,
) *MirrorReconciler {
	return &MirrorReconciler{
		client:          c,
		recorder:        recorder,
		createMirrorUC:  createMirrorUC,
		deleteMirrorUC:  deleteMirrorUC,
		reflectUseCase:  reflectUC,
		reflectionSvc:   reflectionSvc,
		namespaceLister: namespaceLister,
		mirrorStore:     mirrorStore,
		mirrorCache:     mirrorCache,
		log:             log,
	}
}

func (r *MirrorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("namespace", req.Namespace, "name", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("mirror", getResultLabel(reconcileErr), time.Since(start))
	}()

	obj, err := r.fetchMirror(ctx, req.Namespace, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Mirror not found, skipping")
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to fetch mirror: %w", err)
	}

	if obj == nil {
		log.Debug("Object does not have reflects annotation, skipping")
		return reconcile.Result{}, nil
	}

	parts := r.splitReflectsAnnotation(obj.Annotations[dto.AnnotationReflects])
	if len(parts) != 2 {
		log.Info("Invalid reflects annotation format")
		return reconcile.Result{}, nil
	}

	sourceNamespace := parts[0]
	sourceName := parts[1]
	isAuto := obj.Annotations[dto.AnnotationAutoReflects] == "true"
	sourceID := valueobject.NewSourceID(sourceNamespace, sourceName)
	mirrorID := valueobject.NewMirrorID(req.Namespace, req.Name)

	source, err := r.fetchSource(ctx, sourceNamespace, sourceName)
	if err != nil {
		log.Error("Failed to fetch source", "error", err)
		reconcileErr = err
		return reconcile.Result{}, nil
	}

	if source == nil {
		if isAuto {
			// Auto-mirrors are driven by ClusterReplicationRule; the source resource is
			// not required to carry replikator.cloudresty.io/replicate="true". Register
			// the mirror in the store so CRR reconciles can find it. Do NOT delete it —
			// auto-mirror lifecycle is managed exclusively by the CRR controller.
			mirror := entity.NewMirror(mirrorID, sourceID, req.Namespace, req.Name, entity.ResourceTypeSecret)
			mirror.SetAutoCreated(true)
			if reflectedAt := obj.Annotations[dto.AnnotationReflectedAt]; reflectedAt != "" {
				mirror.SetReflectedAt(reflectedAt)
			}
			r.mirrorStore.RegisterMirror(mirror)
			r.mirrorCache.Set(mirrorID.String(), &infcache.MirrorEntry{
				MirrorID:   mirrorID.String(),
				SourceID:   sourceID.String(),
				IsAuto:     true,
				CreatedAt:  time.Now(),
				LastSyncAt: time.Now(),
			})
			return reconcile.Result{}, nil
		}
		log.Info("Source no longer exists")
		if err := r.deleteMirrorIfOrphaned(ctx, req.Namespace, req.Name); err != nil {
			reconcileErr = err
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	mirror := entity.NewMirror(mirrorID, sourceID, req.Namespace, req.Name, source.ResourceType())
	mirror.SetAutoCreated(isAuto)

	if reflectedAt := obj.Annotations[dto.AnnotationReflectedAt]; reflectedAt != "" {
		mirror.SetReflectedAt(reflectedAt)
	}

	r.mirrorStore.RegisterMirror(mirror)

	r.mirrorCache.Set(mirrorID.String(), &infcache.MirrorEntry{
		MirrorID:   mirrorID.String(),
		SourceID:   sourceID.String(),
		IsAuto:     isAuto,
		CreatedAt:  time.Now(),
		LastSyncAt: time.Now(),
	})

	mirrors, _ := r.mirrorStore.GetBySourceID(ctx, sourceID)
	for range mirrors {
		if err := r.reflectUseCase.Execute(ctx, source); err != nil {
			log.Error("Failed to reflect", "error", err)
		}
	}

	log.Debug("Mirror reconciled successfully")
	return reconcile.Result{}, nil
}

func (r *MirrorReconciler) fetchMirror(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err == nil {
		if dto.IsHelmSecret(secret) {
			r.log.Debug("Skipping Helm secret for mirror", "namespace", namespace, "name", name)
			return nil, nil
		}
		if secret.Annotations != nil && secret.Annotations[dto.AnnotationReflects] != "" {
			return secret, nil
		}
		return nil, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err == nil {
		if dto.IsHelmConfigMap(cm) {
			r.log.Debug("Skipping Helm configmap for mirror", "namespace", namespace, "name", name)
			return nil, nil
		}
		if cm.Annotations != nil && cm.Annotations[dto.AnnotationReflects] != "" {
			return nil, fmt.Errorf("configmap cannot be used as mirror target")
		}
		return nil, nil
	}

	return nil, errors.NewNotFound(corev1.Resource("mirror"), name)
}

func (r *MirrorReconciler) fetchSource(ctx context.Context, namespace, name string) (*entity.Source, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err == nil {
		if dto.IsHelmSecret(secret) {
			r.log.Debug("Skipping Helm secret as source", "namespace", namespace, "name", name)
			return nil, nil
		}
		return r.secretToSource(secret), nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err == nil {
		return r.configMapToSource(cm), nil
	}

	return nil, errors.NewNotFound(corev1.Resource("source"), name)
}

func (r *MirrorReconciler) secretToSource(secret *corev1.Secret) *entity.Source {
	if secret.Annotations == nil {
		return nil
	}

	allowedStr := secret.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(secret.Namespace, secret.Name, entity.ResourceTypeSecret)

	allowedNamespacesStr := secret.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := secret.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	source.SetAllowed(true)
	source.SetVersion(secret.ResourceVersion)

	return source
}

func (r *MirrorReconciler) configMapToSource(cm *corev1.ConfigMap) *entity.Source {
	if cm.Annotations == nil {
		return nil
	}

	if dto.IsHelmConfigMap(cm) {
		r.log.Debug("Skipping Helm configmap as source", "namespace", cm.Namespace, "name", cm.Name)
		return nil
	}

	allowedStr := cm.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(cm.Namespace, cm.Name, entity.ResourceTypeConfigMap)

	allowedNamespacesStr := cm.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := cm.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	source.SetAllowed(true)
	source.SetVersion(cm.ResourceVersion)

	return source
}

func (r *MirrorReconciler) deleteMirrorIfOrphaned(ctx context.Context, namespace, name string) error {
	return r.deleteMirrorUC.Execute(ctx, valueobject.NewMirrorID(namespace, name), entity.ResourceTypeSecret)
}

func (r *MirrorReconciler) splitReflectsAnnotation(value string) []string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return []string{"", value}
}

func (r *MirrorReconciler) SetupWithManager(mgr any) error {
	return nil
}

type NamespaceReconciler struct {
	client          client.Client
	recorder        record.EventRecorder
	reflectUseCase  *usecase.ReflectResourcesUseCase
	namespaceLister *k8s.NamespaceListerAdapter
	mirrorCache     *infcache.MirrorCache
	mirrorStore     *k8s.InMemoryMirrorStore
	log             logger.Logger
	sessionManager  *watcher.SessionManager
}

func NewNamespaceReconciler(
	c client.Client,
	recorder record.EventRecorder,
	reflectUC *usecase.ReflectResourcesUseCase,
	namespaceLister *k8s.NamespaceListerAdapter,
	mirrorCache *infcache.MirrorCache,
	mirrorStore *k8s.InMemoryMirrorStore,
	log logger.Logger,
	sessionManager *watcher.SessionManager,
) *NamespaceReconciler {
	return &NamespaceReconciler{
		client:          c,
		recorder:        recorder,
		reflectUseCase:  reflectUC,
		namespaceLister: namespaceLister,
		mirrorCache:     mirrorCache,
		mirrorStore:     mirrorStore,
		log:             log,
		sessionManager:  sessionManager,
	}
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("namespace", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("namespace", getResultLabel(reconcileErr), time.Since(start))
	}()

	if r.sessionManager != nil && !r.sessionManager.IsSessionValid() {
		log.Info("Watch session expired or invalid, clearing caches")
		r.mirrorCache.Clear()
		r.sessionManager.StartSession()
		metrics.RecordWatchSessionRestart()
	}

	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, req.NamespacedName, ns)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Namespace deleted, cleaning up mirrors")
			r.handleNamespaceDeletion(req.Name)
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get namespace: %w", err)
	}

	log.Debug("Namespace event received", "phase", ns.Status.Phase)

	return reconcile.Result{}, nil
}

func (r *NamespaceReconciler) handleNamespaceDeletion(namespace string) {
	deleted := r.mirrorCache.DeleteByNamespace(namespace)
	for _, mirrorKey := range deleted {
		r.log.Info("Deleted mirror from cache due to namespace deletion", "mirror", mirrorKey)
	}

	mirrors, _ := r.mirrorStore.ListByNamespace(namespace)
	for _, mirror := range mirrors {
		if mirror.IsAutoCreated() {
			r.mirrorStore.UnregisterMirror(mirror.ID())
			r.log.Info("Deleted auto-created mirror due to namespace deletion",
				"namespace", namespace,
				"mirror", mirror.FullName())
		}
	}
}

func (r *NamespaceReconciler) SetupWithManager(mgr any) error {
	return nil
}

type CertificateReconciler struct {
	client         client.Client
	scheme         *runtime.Scheme
	recorder       record.EventRecorder
	reflectUseCase *usecase.ReflectResourcesUseCase
	log            logger.Logger
}

func NewCertificateReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	reflectUC *usecase.ReflectResourcesUseCase,
	log logger.Logger,
) *CertificateReconciler {
	return &CertificateReconciler{
		client:         c,
		scheme:         scheme,
		recorder:       recorder,
		reflectUseCase: reflectUC,
		log:            log,
	}
}

func (r *CertificateReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("namespace", req.Namespace, "name", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("certificate", getResultLabel(reconcileErr), time.Since(start))
	}()

	cert := &certmanagerv1.Certificate{}
	err := r.client.Get(ctx, req.NamespacedName, cert)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get Certificate: %w", err)
	}

	secretName := cert.Spec.SecretName
	if secretName == "" {
		return reconcile.Result{}, nil
	}

	secret := &corev1.Secret{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: secretName}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get Secret: %w", err)
	}

	source := r.certificateSecretToSource(secret)
	if source == nil {
		log.Debug("Secret does not have replication annotations from Certificate")
		return reconcile.Result{}, nil
	}

	if err := r.reflectUseCase.Execute(ctx, source); err != nil {
		log.Error("Failed to replicate resources from Certificate", "error", err)
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to replicate: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *CertificateReconciler) certificateSecretToSource(secret *corev1.Secret) *entity.Source {
	if secret.Annotations == nil {
		return nil
	}

	allowedStr := secret.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(secret.Namespace, secret.Name, entity.ResourceTypeSecret)

	allowedNamespacesStr := secret.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := secret.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	autoNamespacesStr := secret.Annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(true)
	source.SetVersion(secret.ResourceVersion)

	return source
}

type IngressReconciler struct {
	client         client.Client
	scheme         *runtime.Scheme
	recorder       record.EventRecorder
	reflectUseCase *usecase.ReflectResourcesUseCase
	log            logger.Logger
}

func NewIngressReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	reflectUC *usecase.ReflectResourcesUseCase,
	log logger.Logger,
) *IngressReconciler {
	return &IngressReconciler{
		client:         c,
		scheme:         scheme,
		recorder:       recorder,
		reflectUseCase: reflectUC,
		log:            log,
	}
}

func (r *IngressReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("namespace", req.Namespace, "name", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("ingress", getResultLabel(reconcileErr), time.Since(start))
	}()

	ingress := &networkingv1.Ingress{}
	err := r.client.Get(ctx, req.NamespacedName, ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get Ingress: %w", err)
	}

	secretTemplateStr, ok := ingress.Annotations["cert-manager.io/secret-template"]
	if !ok || secretTemplateStr == "" {
		return reconcile.Result{}, nil
	}

	var secretTemplate struct {
		Annotations map[string]string `json:"annotations,omitempty"`
	}
	if err := json.Unmarshal([]byte(secretTemplateStr), &secretTemplate); err != nil {
		log.Error("Failed to parse cert-manager.io/secret-template annotation", "error", err)
		return reconcile.Result{}, nil
	}

	replicationAllowed := secretTemplate.Annotations[dto.AnnotationReplicationAllowed]
	if replicationAllowed != "true" {
		return reconcile.Result{}, nil
	}

	if len(ingress.Spec.TLS) == 0 {
		return reconcile.Result{}, nil
	}

	secretName := ingress.Spec.TLS[0].SecretName
	if secretName == "" {
		return reconcile.Result{}, nil
	}

	secret := &corev1.Secret{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: secretName}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get Secret: %w", err)
	}

	source := r.ingressSecretToSource(secret, secretTemplate.Annotations)
	if source == nil {
		log.Debug("Secret does not have replication annotations from Ingress")
		return reconcile.Result{}, nil
	}

	if err := r.reflectUseCase.Execute(ctx, source); err != nil {
		log.Error("Failed to replicate resources from Ingress", "error", err)
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to replicate: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *IngressReconciler) ingressSecretToSource(secret *corev1.Secret, ingressAnnotations map[string]string) *entity.Source {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	for k, v := range ingressAnnotations {
		if _, exists := secret.Annotations[k]; !exists {
			secret.Annotations[k] = v
		}
	}

	allowedStr := secret.Annotations[dto.AnnotationReplicationAllowed]
	if allowedStr != "true" {
		return nil
	}

	source := entity.NewSource(secret.Namespace, secret.Name, entity.ResourceTypeSecret)

	allowedNamespacesStr := secret.Annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabledStr := secret.Annotations[dto.AnnotationReplicationAutoEnabled]
	source.SetAutoEnabled(autoEnabledStr == "true")

	autoNamespacesStr := secret.Annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(true)
	source.SetVersion(secret.ResourceVersion)

	return source
}
