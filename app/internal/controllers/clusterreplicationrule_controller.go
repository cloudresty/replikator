package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "replikator/apis/replication/v1"
	"replikator/internal/application/dto"
	"replikator/internal/application/usecase"
	"replikator/internal/domain/entity"
	"replikator/internal/domain/valueobject"
	"replikator/internal/infrastructure/adapter/k8s"
	infcache "replikator/internal/infrastructure/cache"
	"replikator/internal/metrics"
	"replikator/pkg/logger"
)

const (
	defaultBatchSize      = 50
	defaultRateLimitQPS   = 20
	defaultRateLimitBurst = 100
	defaultMinDelayMs     = 100
	defaultMaxDelayMs     = 2000
)

type RateLimiter struct {
	mu        sync.Mutex
	tokens    float64
	maxTokens float64
	qps       float64
	lastTime  time.Time
}

func NewRateLimiter(qps float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:    float64(burst),
		maxTokens: float64(burst),
		qps:       qps,
		lastTime:  time.Now(),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.lastTime = now

	r.tokens += elapsed * r.qps
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}

	if r.tokens < 1 {
		return false
	}

	r.tokens--
	return true
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	delay := time.Duration(defaultMinDelayMs+rand.Intn(defaultMaxDelayMs-defaultMinDelayMs)) * time.Millisecond
	if !r.Allow() {
		delay = time.Duration(float64(defaultMaxDelayMs)*1.5) * time.Millisecond
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

type BatchProcessor struct {
	limiter   *RateLimiter
	batchSize int
}

func NewBatchProcessor(qps float64, burst, batchSize int) *BatchProcessor {
	return &BatchProcessor{
		limiter:   NewRateLimiter(qps, burst),
		batchSize: batchSize,
	}
}

func (bp *BatchProcessor) AddAndExecute(ctx context.Context, work func() error) error {
	if err := bp.limiter.Wait(ctx); err != nil {
		return err
	}
	return work()
}

func (bp *BatchProcessor) ProcessBatch(ctx context.Context, items []string, work func(item string) error) error {
	for i, item := range items {
		if i > 0 && i%bp.batchSize == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		if err := bp.AddAndExecute(ctx, func() error { return work(item) }); err != nil {
			return err
		}
	}
	return nil
}

type ClusterReplicationRuleReconciler struct {
	client          client.Client
	scheme          *runtime.Scheme
	recorder        record.EventRecorder
	reflectUseCase  *usecase.ReflectResourcesUseCase
	deleteSourceUC  *usecase.HandleSourceDeletionUseCase
	namespaceLister *k8s.NamespaceListerAdapter
	propertiesCache *infcache.PropertiesCache
	notFoundCache   *infcache.NotFoundCache
	log             logger.Logger
}

func NewClusterReplicationRuleReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
	reflectUC *usecase.ReflectResourcesUseCase,
	deleteUC *usecase.HandleSourceDeletionUseCase,
	namespaceLister *k8s.NamespaceListerAdapter,
	propsCache *infcache.PropertiesCache,
	notFoundCache *infcache.NotFoundCache,
	log logger.Logger,
) *ClusterReplicationRuleReconciler {
	return &ClusterReplicationRuleReconciler{
		client:          c,
		scheme:          scheme,
		recorder:        recorder,
		reflectUseCase:  reflectUC,
		deleteSourceUC:  deleteUC,
		namespaceLister: namespaceLister,
		propertiesCache: propsCache,
		notFoundCache:   notFoundCache,
		log:             log,
	}
}

func (r *ClusterReplicationRuleReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.With("name", req.Name)
	start := time.Now()
	var reconcileErr error

	defer func() {
		metrics.RecordReconciliationDuration("clusterrule", getResultLabel(reconcileErr), time.Since(start))
	}()

	rule := &v1.ClusterReplicationRule{}
	err := r.client.Get(ctx, req.NamespacedName, rule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to get ClusterReplicationRule: %w", err)
	}

	if rule.Spec.SourceNamespace == "" {
		log.Warn("ClusterReplicationRule has no source namespace, skipping")
		return reconcile.Result{}, nil
	}

	selector, err := r.compileSelector(rule.Spec.SourceSelector)
	if err != nil {
		log.Error("Invalid source selector", "error", err)
		return reconcile.Result{}, nil
	}

	targetNamespaces, err := r.determineTargetNamespaces(ctx, &rule.Spec.TargetNamespaces)
	if err != nil {
		log.Error("Failed to determine target namespaces", "error", err)
		reconcileErr = err
		return reconcile.Result{}, fmt.Errorf("failed to determine target namespaces: %w", err)
	}

	if err := r.processRule(ctx, rule, selector, targetNamespaces); err != nil {
		log.Error("Failed to process rule", "error", err)
		reconcileErr = err
	}

	r.updateRuleStatus(ctx, rule, targetNamespaces)

	log.Debug("ClusterReplicationRule reconciled successfully",
		"source_namespace", rule.Spec.SourceNamespace,
		"target_namespaces", len(targetNamespaces),
		"synced_resources", len(rule.Status.SyncedResources))
	return reconcile.Result{}, nil
}

func (r *ClusterReplicationRuleReconciler) compileSelector(selector string) (*regexp.Regexp, error) {
	if selector == "" {
		return nil, fmt.Errorf("selector cannot be empty")
	}

	regexStr := selector
	if !strings.HasPrefix(selector, "^") && !strings.HasPrefix(selector, ".*") {
		regexStr = "^" + regexStr
	}
	if !strings.HasSuffix(selector, "$") && !strings.HasSuffix(selector, ".*") {
		regexStr = regexStr + "$"
	}

	return regexp.Compile(regexStr)
}

func (r *ClusterReplicationRuleReconciler) determineTargetNamespaces(ctx context.Context, target *v1.TargetNamespaces) ([]string, error) {
	allNamespaces, err := r.namespaceLister.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	result := make([]string, 0)

	if len(target.Includes) > 0 {
		for _, ns := range allNamespaces {
			for _, include := range target.Includes {
				if ns == include {
					result = append(result, ns)
					break
				}
			}
		}
	} else {
		result = allNamespaces
	}

	if target.Selector != "" {
		selectorRe, err := r.compileSelector(target.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid namespace selector: %w", err)
		}
		filtered := make([]string, 0)
		for _, ns := range result {
			if selectorRe.MatchString(ns) {
				filtered = append(filtered, ns)
			}
		}
		result = filtered
	}

	excludes := make(map[string]bool)
	for _, ex := range target.Excludes {
		excludes[ex] = true
	}

	final := make([]string, 0)
	for _, ns := range result {
		if !excludes[ns] {
			final = append(final, ns)
		}
	}

	return final, nil
}

func (r *ClusterReplicationRuleReconciler) processRule(ctx context.Context, rule *v1.ClusterReplicationRule, selector *regexp.Regexp, targetNamespaces []string) error {
	resourceTypes := rule.Spec.ResourceTypes
	if len(resourceTypes) == 0 {
		resourceTypes = []string{"Secret", "ConfigMap"}
	}

	processedResources := make([]v1.SyncedResource, 0)
	totalMirrors := 0
	now := time.Now()

	for _, rt := range resourceTypes {
		synced, mirrors, err := r.processResourceTypeWithStats(ctx, rule, rt, selector, targetNamespaces)
		if err != nil {
			r.log.Error("Failed to process resource type", "type", rt, "error", err)
			continue
		}
		processedResources = append(processedResources, synced...)
		totalMirrors += mirrors
	}

	rule.Status.SyncedResources = processedResources
	rule.Status.MirroredCount = int32(totalMirrors)
	rule.Status.LastSyncTime = &metav1.Time{Time: now}

	if len(processedResources) > 0 {
		rule.Status.Condition = &v1.ReplicationRuleCondition{
			Type:               v1.ReplicationRuleActive,
			Status:             metav1.ConditionStatus("True"),
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             "ResourcesSynced",
			Message:            fmt.Sprintf("Synced %d resources to %d mirrors", len(processedResources), totalMirrors),
		}
	}

	return nil
}

func (r *ClusterReplicationRuleReconciler) processResourceTypeWithStats(ctx context.Context, rule *v1.ClusterReplicationRule, resourceType string, selector *regexp.Regexp, targetNamespaces []string) ([]v1.SyncedResource, int, error) {
	now := time.Now()
	synced := make([]v1.SyncedResource, 0)
	totalMirrors := 0

	batchProcessor := NewBatchProcessor(defaultRateLimitQPS, defaultRateLimitBurst, defaultBatchSize)

	switch resourceType {
	case "Secret":
		secrets := &corev1.SecretList{}
		err := r.client.List(ctx, secrets, client.InNamespace(rule.Spec.SourceNamespace))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range secrets.Items {
			if !selector.MatchString(secret.Name) {
				continue
			}

			if dto.IsHelmSecret(&secret) {
				continue
			}

			source := r.secretToSource(&secret, rule, targetNamespaces)
			if source == nil {
				continue
			}

			if err := batchProcessor.AddAndExecute(ctx, func() error {
				return r.reflectUseCase.Execute(ctx, source)
			}); err != nil {
				r.log.Error("Failed to reflect secret", "source", source.FullName(), "error", err)
				continue
			}

			synced = append(synced, v1.SyncedResource{
				Name:            secret.Name,
				Namespace:       secret.Namespace,
				ResourceType:    "Secret",
				MirroredToCount: len(targetNamespaces),
				LastSyncedTime:  &metav1.Time{Time: now},
			})
			totalMirrors += len(targetNamespaces)
		}
	case "ConfigMap":
		configMaps := &corev1.ConfigMapList{}
		err := r.client.List(ctx, configMaps, client.InNamespace(rule.Spec.SourceNamespace))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list configmaps: %w", err)
		}

		for _, cm := range configMaps.Items {
			if !selector.MatchString(cm.Name) {
				continue
			}

			if dto.IsHelmConfigMap(&cm) {
				continue
			}

			source := r.configMapToSource(&cm, rule, targetNamespaces)
			if source == nil {
				continue
			}

			if err := batchProcessor.AddAndExecute(ctx, func() error {
				return r.reflectUseCase.Execute(ctx, source)
			}); err != nil {
				r.log.Error("Failed to reflect configmap", "source", source.FullName(), "error", err)
				continue
			}

			synced = append(synced, v1.SyncedResource{
				Name:            cm.Name,
				Namespace:       cm.Namespace,
				ResourceType:    "ConfigMap",
				MirroredToCount: len(targetNamespaces),
				LastSyncedTime:  &metav1.Time{Time: now},
			})
			totalMirrors += len(targetNamespaces)
		}
	}

	return synced, totalMirrors, nil
}

func (r *ClusterReplicationRuleReconciler) secretToSource(secret *corev1.Secret, rule *v1.ClusterReplicationRule, targetNamespaces []string) *entity.Source {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	secret.Annotations[dto.AnnotationReplicationAllowed] = "true"

	if len(rule.Spec.AnnotationsToCopy) > 0 {
		for _, key := range rule.Spec.AnnotationsToCopy {
			if val, ok := secret.Annotations[key]; ok {
				secret.Annotations["replikator.cloudresty.io/copied-"+key] = val
			}
		}
	}

	source := entity.NewSource(secret.Namespace, secret.Name, entity.ResourceTypeSecret)

	if rule.Spec.TargetNameTemplate != "" {
		transformedName := r.transformTargetName(secret.Name, rule.Spec.TargetNameTemplate, rule)
		source.SetTargetName(transformedName)
	}

	allowedNS, _ := valueobject.NewAllowedNamespaces([]string{"*"})
	source.SetAllowedNamespaces(allowedNS)
	source.SetAllowed(true)

	// Enable auto-mirroring to the CRR-specified target namespaces so that
	// handleAutoMirrors creates the mirror entries and reflectToMirror
	// writes the actual Kubernetes secret/configmap.
	if len(targetNamespaces) > 0 {
		source.SetAutoEnabled(true)
		autoNS, _ := valueobject.NewAllowedNamespaces(targetNamespaces)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetVersion(secret.ResourceVersion)

	return source
}

func (r *ClusterReplicationRuleReconciler) configMapToSource(cm *corev1.ConfigMap, rule *v1.ClusterReplicationRule, targetNamespaces []string) *entity.Source {
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}

	cm.Annotations[dto.AnnotationReplicationAllowed] = "true"

	if len(rule.Spec.AnnotationsToCopy) > 0 {
		for _, key := range rule.Spec.AnnotationsToCopy {
			if val, ok := cm.Annotations[key]; ok {
				cm.Annotations["replikator.cloudresty.io/copied-"+key] = val
			}
		}
	}

	source := entity.NewSource(cm.Namespace, cm.Name, entity.ResourceTypeConfigMap)

	if rule.Spec.TargetNameTemplate != "" {
		transformedName := r.transformTargetName(cm.Name, rule.Spec.TargetNameTemplate, rule)
		source.SetTargetName(transformedName)
	}

	allowedNS, _ := valueobject.NewAllowedNamespaces([]string{"*"})
	source.SetAllowedNamespaces(allowedNS)
	source.SetAllowed(true)

	// Enable auto-mirroring to the CRR-specified target namespaces so that
	// handleAutoMirrors creates the mirror entries and reflectToMirror
	// writes the actual Kubernetes secret/configmap.
	if len(targetNamespaces) > 0 {
		source.SetAutoEnabled(true)
		autoNS, _ := valueobject.NewAllowedNamespaces(targetNamespaces)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetVersion(cm.ResourceVersion)

	return source
}

func (r *ClusterReplicationRuleReconciler) transformTargetName(sourceName string, template string, rule *v1.ClusterReplicationRule) string {
	re := regexp.MustCompile(`\$\d+|\$\{\d+\}`)
	matches := re.FindAllString(template, -1)

	result := template
	for _, match := range matches {
		groupIdx := match[1:]
		if match[0] == '$' && match[1] == '{' {
			groupIdx = match[2 : len(match)-1]
		}

		var groupNumber int
		if n, err := fmt.Sscanf(groupIdx, "%d", &groupNumber); err != nil || n != 1 {
			continue
		}

		replacement := ""
		regexStr := rule.Spec.SourceSelector
		if !strings.HasPrefix(rule.Spec.SourceSelector, "^") && !strings.HasPrefix(rule.Spec.SourceSelector, ".*") {
			regexStr = "^" + regexStr
		}
		if !strings.HasSuffix(rule.Spec.SourceSelector, "$") && !strings.HasSuffix(rule.Spec.SourceSelector, ".*") {
			regexStr = regexStr + "$"
		}

		compiledRegex, err := regexp.Compile(regexStr)
		if err == nil {
			matches := compiledRegex.FindStringSubmatch(sourceName)
			if len(matches) > groupNumber {
				replacement = matches[groupNumber]
			}
		}

		result = strings.Replace(result, match, replacement, 1)
	}

	return result
}

func (r *ClusterReplicationRuleReconciler) updateRuleStatus(ctx context.Context, rule *v1.ClusterReplicationRule, targetNamespaces []string) {
	rule.Status.ObservedGeneration = rule.GetGeneration()
	if rule.Status.LastSyncTime == nil {
		rule.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
	}
	rule.Status.ActiveMirrors = int32(len(targetNamespaces))

	if err := r.client.Status().Update(ctx, rule); err != nil {
		r.log.Error("Failed to update rule status", "error", err)
	}
}

func (r *ClusterReplicationRuleReconciler) SetupWithManager(mgr any) error {
	return nil
}
