package controllers

import (
	"context"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "replikator/apis/replication/v1"
	"replikator/internal/application/dto"
	"replikator/internal/application/usecase"
	"replikator/internal/domain/event"
	"replikator/internal/domain/service"
	"replikator/internal/infrastructure/adapter/k8s"
	"replikator/pkg/logger"
)

type ruleFixture struct {
	r        *ClusterReplicationRuleReconciler
	c        client.Client
	mirrorWr *atomic.Int32 // updates to mirror secrets/configmaps
}

func newRuleFixture(t *testing.T, objs ...client.Object) *ruleFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	updates := &atomic.Int32{}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1.ClusterReplicationRule{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if obj.GetAnnotations()[dto.AnnotationReflects] != "" {
					updates.Add(1)
				}
				return cl.Update(ctx, obj, opts...)
			},
		}).
		Build()

	log := logger.NewSlogLogger()
	adapter := k8s.NewKubernetesAdapter(c, record.NewFakeRecorder(100), scheme)
	lister := k8s.NewNamespaceListerAdapter(c)
	reflectUC := usecase.NewReflectResourcesUseCase(
		service.NewReflectionService(event.NewInMemoryDispatcher()),
		adapter, k8s.NewInMemoryMirrorStore(), lister, log)

	r := NewClusterReplicationRuleReconciler(c, scheme, record.NewFakeRecorder(100),
		reflectUC, nil, lister, nil, nil, log)
	return &ruleFixture{r: r, c: c, mirrorWr: updates}
}

func ns(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func esSecret(pw string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "elasticsearch", Name: "development-es-elastic-user"},
		Data:       map[string][]byte{"elastic": []byte(pw)},
	}
}

func esRule() *v1.ClusterReplicationRule {
	return &v1.ClusterReplicationRule{
		ObjectMeta: metav1.ObjectMeta{Name: "es-user"},
		Spec: v1.ReplicationRuleSpec{
			SourceNamespace:  "elasticsearch",
			SourceSelector:   "development-es-elastic-user",
			ResourceTypes:    []string{"Secret"},
			TargetNamespaces: v1.TargetNamespaces{Includes: []string{"app"}},
		},
	}
}

func baseObjs(pw string) []client.Object {
	return []client.Object{ns("elasticsearch"), ns("app"), ns("other"), esSecret(pw), esRule()}
}

func reconcileRule(t *testing.T, f *ruleFixture) reconcile.Result {
	t.Helper()
	res, err := f.r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKey{Name: "es-user"}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func getMirror(t *testing.T, f *ruleFixture) *corev1.Secret {
	t.Helper()
	m := &corev1.Secret{}
	if err := f.c.Get(context.Background(), client.ObjectKey{Namespace: "app", Name: "development-es-elastic-user"}, m); err != nil {
		t.Fatalf("mirror missing: %v", err)
	}
	return m
}

func TestMapSecretToRules(t *testing.T) {
	f := newRuleFixture(t, baseObjs("old")...)
	ctx := context.Background()

	cases := []struct {
		name string
		obj  *corev1.Secret
		want int
	}{
		{"unannotated source", esSecret("x"), 1},
		{"same name, other namespace", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "development-es-elastic-user"}}, 0},
		{"other name, source namespace", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "elasticsearch", Name: "unrelated"}}, 0},
		{"mirror maps back to the rule", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: "app", Name: "development-es-elastic-user",
			Annotations: map[string]string{dto.AnnotationReflects: "elasticsearch/development-es-elastic-user"}}}, 1},
		{"helm secret ignored", &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "elasticsearch", Name: "development-es-elastic-user"},
			Type:       "helm.sh/release.v1"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(f.r.mapSecretToRules(ctx, tc.obj)); got != tc.want {
				t.Fatalf("got %d requests, want %d", got, tc.want)
			}
		})
	}

	// A Secret-only rule must not be enqueued by ConfigMap events.
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "elasticsearch", Name: "development-es-elastic-user"}}
	if got := len(f.r.mapConfigMapToRules(ctx, cm)); got != 0 {
		t.Fatalf("configmap event enqueued a secret-only rule: %d", got)
	}
}

func TestSourceChangePropagatesToMirror(t *testing.T) {
	f := newRuleFixture(t, baseObjs("old")...)
	ctx := context.Background()

	res := reconcileRule(t, f)
	if res.RequeueAfter != DefaultRuleResyncPeriod {
		t.Fatalf("RequeueAfter = %v, want %v", res.RequeueAfter, DefaultRuleResyncPeriod)
	}
	m := getMirror(t, f)
	if string(m.Data["elastic"]) != "old" {
		t.Fatalf("initial mirror data = %q", m.Data["elastic"])
	}
	oldVersion := m.Annotations[dto.AnnotationReflectedVersion]

	// Rotate the unannotated source; the event must enqueue the rule.
	src := &corev1.Secret{}
	if err := f.c.Get(ctx, client.ObjectKey{Namespace: "elasticsearch", Name: "development-es-elastic-user"}, src); err != nil {
		t.Fatal(err)
	}
	if _, ok := src.Annotations[dto.AnnotationReplicationAllowed]; ok {
		t.Fatal("source must stay unannotated")
	}
	src.Data["elastic"] = []byte("new")
	if err := f.c.Update(ctx, src); err != nil {
		t.Fatal(err)
	}
	reqs := f.r.mapSecretToRules(ctx, src)
	if len(reqs) != 1 || reqs[0].Name != "es-user" {
		t.Fatalf("source update did not enqueue the rule: %v", reqs)
	}

	reconcileRule(t, f)
	m = getMirror(t, f)
	if string(m.Data["elastic"]) != "new" {
		t.Fatalf("mirror data = %q, want new", m.Data["elastic"])
	}
	if v := m.Annotations[dto.AnnotationReflectedVersion]; v == oldVersion || v != src.ResourceVersion {
		t.Fatalf("replicated-version = %q (old %q, source rv %q)", v, oldVersion, src.ResourceVersion)
	}
}

func TestNoChangeReconcilePerformsNoMirrorWrite(t *testing.T) {
	f := newRuleFixture(t, baseObjs("pw")...)
	reconcileRule(t, f)
	before := getMirror(t, f)
	writes := f.mirrorWr.Load()

	reconcileRule(t, f)
	reconcileRule(t, f)

	if got := f.mirrorWr.Load(); got != writes {
		t.Fatalf("no-change reconcile wrote the mirror %d times", got-writes)
	}
	after := getMirror(t, f)
	if after.ResourceVersion != before.ResourceVersion ||
		after.Annotations[dto.AnnotationReflectedAt] != before.Annotations[dto.AnnotationReflectedAt] {
		t.Fatal("mirror resourceVersion/replicated-at churned on a no-change reconcile")
	}
}

func TestManualMirrorEditIsHealed(t *testing.T) {
	f := newRuleFixture(t, baseObjs("pw")...)
	reconcileRule(t, f)

	m := getMirror(t, f)
	m.Data["elastic"] = []byte("tampered")
	if err := f.c.Update(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if got := len(f.r.mapSecretToRules(context.Background(), m)); got != 1 {
		t.Fatalf("mirror edit enqueued %d rules, want 1", got)
	}

	reconcileRule(t, f)
	if got := string(getMirror(t, f).Data["elastic"]); got != "pw" {
		t.Fatalf("mirror not healed, data = %q", got)
	}
}
