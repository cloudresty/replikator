package port

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"replikator/internal/domain/entity"
	"replikator/internal/domain/valueobject"
)

type SourceRepository interface {
	Get(ctx context.Context, namespace, name string) (*entity.Source, error)
	List(ctx context.Context) ([]*entity.Source, error)
	Create(ctx context.Context, source *entity.Source) error
	Update(ctx context.Context, source *entity.Source) error
	Delete(ctx context.Context, id valueobject.SourceID) error
	Watch(ctx context.Context, handler SourceEventHandler) error
}

type SourceEventHandler interface {
	OnAdd(ctx context.Context, source *entity.Source) error
	OnUpdate(ctx context.Context, oldSource, newSource *entity.Source) error
	OnDelete(ctx context.Context, source *entity.Source) error
}

type MirrorRepository interface {
	Get(ctx context.Context, namespace, name string) (*entity.Mirror, error)
	GetBySourceID(ctx context.Context, sourceID valueobject.SourceID) ([]*entity.Mirror, error)
	List(ctx context.Context) ([]*entity.Mirror, error)
	Create(ctx context.Context, mirror *entity.Mirror) error
	Update(ctx context.Context, mirror *entity.Mirror) error
	Delete(ctx context.Context, id valueobject.MirrorID) error
	Watch(ctx context.Context, handler MirrorEventHandler) error
}

type MirrorEventHandler interface {
	OnAdd(ctx context.Context, mirror *entity.Mirror) error
	OnUpdate(ctx context.Context, oldMirror, newMirror *entity.Mirror) error
	OnDelete(ctx context.Context, mirror *entity.Mirror) error
}

type ResourceReader interface {
	GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error)
	GetConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error)
	ListSecrets(ctx context.Context, namespace string) ([]*corev1.Secret, error)
	ListConfigMaps(ctx context.Context, namespace string) ([]*corev1.ConfigMap, error)
}

type ResourceWriter interface {
	CreateSecret(ctx context.Context, secret *corev1.Secret) error
	UpdateSecret(ctx context.Context, secret *corev1.Secret) error
	DeleteSecret(ctx context.Context, namespace, name string) error
	CreateConfigMap(ctx context.Context, cm *corev1.ConfigMap) error
	UpdateConfigMap(ctx context.Context, cm *corev1.ConfigMap) error
	DeleteConfigMap(ctx context.Context, namespace, name string) error
}

type SourceStore interface {
	ResourceReader
	ResourceWriter
	EnsureFinalizer(ctx context.Context, obj metav1.Object, finalizer string) error
	RemoveFinalizer(ctx context.Context, obj metav1.Object, finalizer string) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event any) error
}
