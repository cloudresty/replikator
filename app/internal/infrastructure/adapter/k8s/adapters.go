package k8s

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"replikator/internal/application/dto"
	"replikator/internal/application/port"
	"replikator/internal/domain/entity"
	"replikator/internal/domain/valueobject"
)

const (
	ReflectorFinalizer = "reflector.k8s.emberstack.com/finalizer"
)

type KubernetesAdapter struct {
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

func NewKubernetesAdapter(
	c client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme,
) *KubernetesAdapter {
	return &KubernetesAdapter{
		client:   c,
		recorder: recorder,
		scheme:   scheme,
	}
}

func (a *KubernetesAdapter) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (a *KubernetesAdapter) GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func (a *KubernetesAdapter) ListSecrets(ctx context.Context, namespace string) ([]*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	err := a.client.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	result := make([]*corev1.Secret, len(secretList.Items))
	for i := range secretList.Items {
		result[i] = &secretList.Items[i]
	}
	return result, nil
}

func (a *KubernetesAdapter) ListConfigMaps(ctx context.Context, namespace string) ([]*corev1.ConfigMap, error) {
	cmList := &corev1.ConfigMapList{}
	err := a.client.List(ctx, cmList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	result := make([]*corev1.ConfigMap, len(cmList.Items))
	for i := range cmList.Items {
		result[i] = &cmList.Items[i]
	}
	return result, nil
}

func (a *KubernetesAdapter) CreateSecret(ctx context.Context, secret *corev1.Secret) error {
	return a.client.Create(ctx, secret)
}

func (a *KubernetesAdapter) UpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	return a.client.Update(ctx, secret)
}

func (a *KubernetesAdapter) DeleteSecret(ctx context.Context, namespace, name string) error {
	secret := &corev1.Secret{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)
	if err != nil {
		return err
	}
	return a.client.Delete(ctx, secret)
}

func (a *KubernetesAdapter) CreateConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	return a.client.Create(ctx, cm)
}

func (a *KubernetesAdapter) UpdateConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	return a.client.Update(ctx, cm)
}

func (a *KubernetesAdapter) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	cm := &corev1.ConfigMap{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err != nil {
		return err
	}
	return a.client.Delete(ctx, cm)
}

func (a *KubernetesAdapter) EnsureFinalizer(ctx context.Context, obj metav1.Object, finalizer string) error {
	if hasFinalizer(obj, finalizer) {
		return nil
	}

	obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))

	switch v := obj.(type) {
	case *corev1.Secret:
		return a.client.Update(ctx, v)
	case *corev1.ConfigMap:
		return a.client.Update(ctx, v)
	}

	return nil
}

func (a *KubernetesAdapter) RemoveFinalizer(ctx context.Context, obj metav1.Object, finalizer string) error {
	finalizers := obj.GetFinalizers()
	newFinalizers := make([]string, 0, len(finalizers))
	for _, f := range finalizers {
		if f != finalizer {
			newFinalizers = append(newFinalizers, f)
		}
	}

	if len(newFinalizers) == len(finalizers) {
		return nil
	}

	obj.SetFinalizers(newFinalizers)

	switch v := obj.(type) {
	case *corev1.Secret:
		return a.client.Update(ctx, v)
	case *corev1.ConfigMap:
		return a.client.Update(ctx, v)
	}

	return nil
}

func hasFinalizer(obj metav1.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

type MirrorAdapter struct {
	client   client.Client
	recorder record.EventRecorder
}

func NewMirrorAdapter(c client.Client, recorder record.EventRecorder) *MirrorAdapter {
	return &MirrorAdapter{
		client:   c,
		recorder: recorder,
	}
}

func (a *MirrorAdapter) Get(ctx context.Context, namespace, name string) (*entity.Mirror, error) {
	obj := &corev1.Secret{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			cm := &corev1.ConfigMap{}
			err = a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
			if err != nil {
				return nil, err
			}
			return secretToMirror(cm), nil
		}
		return nil, err
	}
	return secretToMirror(obj), nil
}

func secretToMirror(obj any) *entity.Mirror {
	var annotations map[string]string
	var name, namespace string
	var resourceType entity.ResourceType

	switch v := obj.(type) {
	case *corev1.Secret:
		annotations = v.Annotations
		name = v.Name
		namespace = v.Namespace
		resourceType = entity.ResourceTypeSecret
	case *corev1.ConfigMap:
		annotations = v.Annotations
		name = v.Name
		namespace = v.Namespace
		resourceType = entity.ResourceTypeConfigMap
	}

	if annotations == nil {
		return nil
	}

	reflects, ok := annotations[dto.AnnotationReflects]
	if !ok || reflects == "" {
		return nil
	}

	parts := splitReflects(reflects)
	if len(parts) != 2 {
		return nil
	}

	sourceID := valueobject.NewSourceID(parts[0], parts[1])
	mirrorID := valueobject.NewMirrorID(namespace, name)

	version := ""
	if v, ok := annotations[dto.AnnotationReflectedVersion]; ok {
		version = v
	}

	mirror := entity.NewMirror(mirrorID, sourceID, namespace, name, resourceType)
	mirror.SetVersion(version)
	return mirror
}

func splitReflects(value string) []string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return []string{"", value}
}

type SourceAdapter struct {
	client   client.Client
	recorder record.EventRecorder
}

func NewSourceAdapter(c client.Client, recorder record.EventRecorder) *SourceAdapter {
	return &SourceAdapter{
		client:   c,
		recorder: recorder,
	}
}

func (a *SourceAdapter) Get(ctx context.Context, namespace, name string) (*entity.Source, error) {
	obj := &corev1.Secret{}
	err := a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			cm := &corev1.ConfigMap{}
			err = a.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
			if err != nil {
				return nil, err
			}
			return configMapToSource(cm), nil
		}
		return nil, err
	}
	return secretToSource(obj), nil
}

func secretToSource(obj *corev1.Secret) *entity.Source {
	annotations := obj.Annotations
	if annotations == nil {
		return nil
	}

	allowed := annotations[dto.AnnotationReplicationAllowed] == "true"
	if !allowed {
		return nil
	}

	source := entity.NewSource(obj.Namespace, obj.Name, entity.ResourceTypeSecret)

	allowedNamespacesStr := annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabled := annotations[dto.AnnotationReplicationAutoEnabled] == "true"
	source.SetAutoEnabled(autoEnabled)

	autoNamespacesStr := annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(allowed)
	source.SetVersion(obj.ResourceVersion)

	return source
}

func configMapToSource(obj *corev1.ConfigMap) *entity.Source {
	annotations := obj.Annotations
	if annotations == nil {
		return nil
	}

	allowed := annotations[dto.AnnotationReplicationAllowed] == "true"
	if !allowed {
		return nil
	}

	source := entity.NewSource(obj.Namespace, obj.Name, entity.ResourceTypeConfigMap)

	allowedNamespacesStr := annotations[dto.AnnotationReplicationAllowedNamespaces]
	if allowedNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(allowedNamespacesStr)
		allowedNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAllowedNamespaces(allowedNS)
	}

	autoEnabled := annotations[dto.AnnotationReplicationAutoEnabled] == "true"
	source.SetAutoEnabled(autoEnabled)

	autoNamespacesStr := annotations[dto.AnnotationReplicationAutoNamespaces]
	if autoNamespacesStr != "" {
		patterns, _ := valueobject.ParseAllowedNamespacesAnnotation(autoNamespacesStr)
		autoNS, _ := valueobject.NewAllowedNamespaces(patterns)
		source.SetAutoNamespaces(autoNS)
	}

	source.SetAllowed(allowed)
	source.SetVersion(obj.ResourceVersion)

	return source
}

type ReflectorAdapter struct {
	adapter     *KubernetesAdapter
	mirrorStore *InMemoryMirrorStore
}

func NewReflectorAdapter(adapter *KubernetesAdapter) *ReflectorAdapter {
	return &ReflectorAdapter{
		adapter:     adapter,
		mirrorStore: NewInMemoryMirrorStore(),
	}
}

func (a *ReflectorAdapter) Get(ctx context.Context, namespace, name string) (*entity.Source, error) {
	return nil, nil
}

func (a *ReflectorAdapter) List(ctx context.Context) ([]*entity.Source, error) {
	return nil, nil
}

func (a *ReflectorAdapter) Create(ctx context.Context, source *entity.Source) error {
	return nil
}

func (a *ReflectorAdapter) Update(ctx context.Context, source *entity.Source) error {
	return nil
}

func (a *ReflectorAdapter) Delete(ctx context.Context, id valueobject.SourceID) error {
	return nil
}

func (a *ReflectorAdapter) Watch(ctx context.Context, handler port.SourceEventHandler) error {
	return nil
}

type InMemoryMirrorStore struct {
	mu      sync.RWMutex
	mirrors map[valueobject.MirrorID]*entity.Mirror
}

func NewInMemoryMirrorStore() *InMemoryMirrorStore {
	return &InMemoryMirrorStore{
		mirrors: make(map[valueobject.MirrorID]*entity.Mirror),
	}
}

func (s *InMemoryMirrorStore) Get(ctx context.Context, namespace, name string) (*entity.Mirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id := valueobject.NewMirrorID(namespace, name)
	mirror, ok := s.mirrors[id]
	if !ok {
		return nil, apierrors.NewNotFound(corev1.Resource("mirror"), name)
	}
	return mirror, nil
}

func (s *InMemoryMirrorStore) GetBySourceID(ctx context.Context, sourceID valueobject.SourceID) ([]*entity.Mirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*entity.Mirror, 0)
	for _, mirror := range s.mirrors {
		if mirror.SourceID().Equals(sourceID) {
			result = append(result, mirror)
		}
	}
	return result, nil
}

func (s *InMemoryMirrorStore) List(ctx context.Context) ([]*entity.Mirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*entity.Mirror, 0, len(s.mirrors))
	for _, mirror := range s.mirrors {
		result = append(result, mirror)
	}
	return result, nil
}

func (s *InMemoryMirrorStore) ListByNamespace(namespace string) ([]*entity.Mirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*entity.Mirror, 0)
	for _, mirror := range s.mirrors {
		if mirror.Namespace() == namespace {
			result = append(result, mirror)
		}
	}
	return result, nil
}

func (s *InMemoryMirrorStore) Create(ctx context.Context, mirror *entity.Mirror) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mirrors[mirror.ID()] = mirror
	return nil
}

func (s *InMemoryMirrorStore) Update(ctx context.Context, mirror *entity.Mirror) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mirrors[mirror.ID()] = mirror
	return nil
}

func (s *InMemoryMirrorStore) Delete(ctx context.Context, id valueobject.MirrorID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.mirrors, id)
	return nil
}

func (s *InMemoryMirrorStore) Watch(ctx context.Context, handler port.MirrorEventHandler) error {
	return nil
}

func (s *InMemoryMirrorStore) RegisterMirror(mirror *entity.Mirror) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mirrors[mirror.ID()] = mirror
}

func (s *InMemoryMirrorStore) UnregisterMirror(id valueobject.MirrorID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.mirrors, id)
}

type NamespaceListerAdapter struct {
	client client.Client
}

func NewNamespaceListerAdapter(c client.Client) *NamespaceListerAdapter {
	return &NamespaceListerAdapter{
		client: c,
	}
}

func (a *NamespaceListerAdapter) List(ctx context.Context) ([]string, error) {
	nsList := &corev1.NamespaceList{}
	if err := a.client.List(ctx, nsList); err != nil {
		return nil, err
	}

	result := make([]string, len(nsList.Items))
	for i, ns := range nsList.Items {
		result[i] = ns.Name
	}
	return result, nil
}

type OwnerReferenceHelper struct {
	client client.Client
}

func NewOwnerReferenceHelper(c client.Client) *OwnerReferenceHelper {
	return &OwnerReferenceHelper{
		client: c,
	}
}

func (h *OwnerReferenceHelper) CreateOwnerReference(ctx context.Context, obj, owner client.Object) error {
	trueVal := true
	gvk := owner.GetObjectKind().GroupVersionKind()
	ownerRef := metav1.OwnerReference{
		UID:        owner.GetUID(),
		Kind:       gvk.Kind,
		Name:       owner.GetName(),
		APIVersion: gvk.GroupVersion().String(),
		Controller: &trueVal,
	}

	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
	return h.client.Update(ctx, obj)
}

var _ port.SourceStore = &KubernetesAdapter{}
var _ port.MirrorRepository = &InMemoryMirrorStore{}
