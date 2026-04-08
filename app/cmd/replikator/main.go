package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8scontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"replikator/apis/replication/v1"
	"replikator/internal/application/usecase"
	"replikator/internal/controllers"
	infrastructureevent "replikator/internal/domain/event"
	"replikator/internal/domain/service"
	"replikator/internal/infrastructure/adapter/k8s"
	infcache "replikator/internal/infrastructure/cache"
	"replikator/internal/infrastructure/config"
	"replikator/internal/watcher"
	"replikator/internal/webhooks"
	"replikator/pkg/logger"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = certmanagerv1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var configFile string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&configFile, "config", "", "The path to the configuration file.")
	flag.Parse()

	cfg := config.Load()

	zapLogger := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(zapLogger)
	log := logger.NewSlogLogger()

	mainCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	restConfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		setupLog.Error(err, "Failed to build rest config")
		os.Exit(1)
	}

	if cfg.Kubernetes.SkipTlsVerify {
		restConfig.Insecure = true
		restConfig.CAData = nil
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Namespace{}:          {},
				&corev1.Secret{}:             {},
				&corev1.ConfigMap{}:          {},
				&certmanagerv1.Certificate{}: {},
				&networkingv1.Ingress{}:      {},
				&v1.ClusterReplicationRule{}: {},
			},
		},
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "replikator-lock",
	})
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	nonDeletingClient := &NonDeletingClient{
		Client: mgr.GetClient(),
	}

	k8sAdapter := k8s.NewKubernetesAdapter(nonDeletingClient, mgr.GetEventRecorderFor("replikator"), mgr.GetScheme())
	namespaceLister := k8s.NewNamespaceListerAdapter(nonDeletingClient)
	mirrorStore := k8s.NewInMemoryMirrorStore()

	eventDispatcher := infrastructureevent.NewInMemoryDispatcher()
	reflectionSvc := service.NewReflectionService(eventDispatcher)

	notFoundCache := infcache.NewNotFoundCache(cfg.WatcherTimeout())
	propertiesCache := infcache.NewPropertiesCache(cfg.WatcherTimeout())
	mirrorCache := infcache.NewMirrorCache()

	sessionManager := watcher.NewSessionManager(cfg.WatcherTimeout(), log)

	reflectUseCase := usecase.NewReflectResourcesUseCase(
		reflectionSvc,
		k8sAdapter,
		mirrorStore,
		namespaceLister,
		log,
	)

	deleteSourceUC := usecase.NewHandleSourceDeletionUseCase(
		mirrorStore,
		k8sAdapter,
		cfg.Reflection.DeleteOrphanedMirrors,
	)

	createMirrorUC := usecase.NewCreateMirrorUseCase(
		mirrorStore,
		reflectionSvc,
	)

	deleteMirrorUC := usecase.NewDeleteMirrorUseCase(
		mirrorStore,
		k8sAdapter,
		cfg.Reflection.DeleteOrphanedMirrors,
	)

	watcherConfig := config.NewWatcherConfig(cfg)

	sourceReconciler := controllers.NewSourceReconciler(
		nonDeletingClient,
		mgr.GetEventRecorderFor("replikator"),
		reflectUseCase,
		deleteSourceUC,
		reflectionSvc,
		namespaceLister,
		propertiesCache,
		notFoundCache,
		log,
		*watcherConfig,
	)

	mirrorReconciler := controllers.NewMirrorReconciler(
		nonDeletingClient,
		mgr.GetEventRecorderFor("replikator"),
		createMirrorUC,
		deleteMirrorUC,
		reflectUseCase,
		reflectionSvc,
		namespaceLister,
		mirrorStore,
		mirrorCache,
		log,
	)

	namespaceReconciler := controllers.NewNamespaceReconciler(
		nonDeletingClient,
		mgr.GetEventRecorderFor("replikator"),
		reflectUseCase,
		namespaceLister,
		mirrorCache,
		mirrorStore,
		log,
		sessionManager,
	)

	certReconciler := controllers.NewCertificateReconciler(
		nonDeletingClient,
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("replikator"),
		reflectUseCase,
		log,
	)

	ingressReconciler := controllers.NewIngressReconciler(
		nonDeletingClient,
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("replikator"),
		reflectUseCase,
		log,
	)

	clusterRuleReconciler := controllers.NewClusterReplicationRuleReconciler(
		nonDeletingClient,
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("replikator"),
		reflectUseCase,
		deleteSourceUC,
		namespaceLister,
		propertiesCache,
		notFoundCache,
		log,
	)

	sessionManager.OnSessionEnd(func() {
		log.Info("Watch session ended, clearing caches")
		notFoundCache.Clear()
		propertiesCache.Clear()
		mirrorCache.Clear()
	})

	if err := setupReconcilers(mgr, sourceReconciler, mirrorReconciler, namespaceReconciler, certReconciler, ingressReconciler, clusterRuleReconciler, notFoundCache); err != nil {
		setupLog.Error(err, "Unable to set up reconcilers")
		os.Exit(1)
	}

	clusterRuleValidator := webhooks.NewClusterReplicationRuleValidator()
	mgr.GetWebhookServer().Register("/validate-replication-cloudresty-io-v1-clusterreplicationrule", &admission.Webhook{Handler: clusterRuleValidator})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	sessionManager.StartSession()

	go func() {
		klog.Info("Starting manager")
		if err := mgr.Start(mainCtx); err != nil {
			setupLog.Error(err, "Problem running manager")
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	setupLog.Info("Shutting down...")
	sessionManager.EndSession()
	cancel()
}

func setupReconcilers(
	mgr ctrl.Manager,
	sourceReconciler *controllers.SourceReconciler,
	mirrorReconciler *controllers.MirrorReconciler,
	namespaceReconciler *controllers.NamespaceReconciler,
	certReconciler *controllers.CertificateReconciler,
	ingressReconciler *controllers.IngressReconciler,
	clusterRuleReconciler *controllers.ClusterReplicationRuleReconciler,
	notFoundCache *infcache.NotFoundCache,
) error {
	sourcePred := controllers.NewSourcePredicate(notFoundCache)
	mirrorPred := controllers.NewMirrorPredicate()

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Named("source-secret").
		WithEventFilter(sourcePred).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 10}).
		Complete(sourceReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Named("source-configmap").
		WithEventFilter(sourcePred).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 10}).
		Complete(sourceReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Named("mirror-secret").
		WithEventFilter(mirrorPred).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 10}).
		Complete(mirrorReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Named("mirror-configmap").
		WithEventFilter(mirrorPred).
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 10}).
		Complete(mirrorReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Named("namespace-watcher").
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 1}).
		Complete(namespaceReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&certmanagerv1.Certificate{}).
		Named("certificate-watcher").
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 5}).
		Complete(certReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress-watcher").
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 5}).
		Complete(ingressReconciler); err != nil {
		return err
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1.ClusterReplicationRule{}).
		Named("cluster-replication-rule").
		WithOptions(k8scontroller.Options{MaxConcurrentReconciles: 5}).
		Complete(clusterRuleReconciler); err != nil {
		return err
	}

	return nil
}

type NonDeletingClient struct {
	client.Client
}

func (c *NonDeletingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (c *NonDeletingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

var _ reconcile.Reconciler = &controllers.SourceReconciler{}
var _ reconcile.Reconciler = &controllers.MirrorReconciler{}
var _ reconcile.Reconciler = &controllers.NamespaceReconciler{}
var _ reconcile.Reconciler = &controllers.CertificateReconciler{}
var _ reconcile.Reconciler = &controllers.IngressReconciler{}
var _ reconcile.Reconciler = &controllers.ClusterReplicationRuleReconciler{}
