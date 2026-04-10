package reconciler

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatormetrics "github.com/nephomaniac/ebs-metrics-exporter/pkg/metrics"
)

const (
	// ConfigMap containing exporter configuration
	ConfigMapName = "ebs-metrics-exporter-config"
	// DaemonSet name
	DaemonSetName = "ebs-metrics-exporter"
	// Annotation to track last config version applied
	ConfigVersionAnnotation = "ebs-metrics-exporter.openshift.io/config-version"
)

// Reconciler validates configuration and coordinates DaemonSet health
// It follows standard operator patterns: validate, coordinate, monitor
type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string
}

// Reconcile implements the reconciliation loop
// It is triggered by watch events on ConfigMap and DaemonSet resources
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// Determine which resource triggered this reconciliation
	switch req.Name {
	case ConfigMapName:
		log.Info("Reconciling ConfigMap", "name", req.Name)
		if err := r.reconcileConfigMap(ctx); err != nil {
			log.Error(err, "Failed to reconcile ConfigMap")
			operatormetrics.ReconciliationErrors.WithLabelValues("configmap").Inc()
			return reconcile.Result{}, err
		}
		operatormetrics.LastSuccessfulReconciliation.WithLabelValues("configmap").SetToCurrentTime()

	case DaemonSetName:
		log.Info("Reconciling DaemonSet", "name", req.Name)
		if err := r.reconcileDaemonSet(ctx); err != nil {
			log.Error(err, "Failed to reconcile DaemonSet")
			operatormetrics.ReconciliationErrors.WithLabelValues("daemonset").Inc()
			return reconcile.Result{}, err
		}
		operatormetrics.LastSuccessfulReconciliation.WithLabelValues("daemonset").SetToCurrentTime()

	default:
		log.Info("Ignoring event for unknown resource", "name", req.Name)
	}

	return reconcile.Result{}, nil
}

// reconcileConfigMap validates configuration and triggers pod restarts if needed
func (r *Reconciler) reconcileConfigMap(ctx context.Context) error {
	log := log.FromContext(ctx)

	actual := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      ConfigMapName,
		Namespace: r.Namespace,
	}, actual); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ConfigMap not found - waiting for PKO to create it")
			return nil
		}
		return err
	}

	// Increment change counter
	operatormetrics.ConfigMapChanges.Inc()

	// Extract and validate config.yaml
	configYAML, ok := actual.Data["config.yaml"]
	if !ok {
		log.Error(nil, "ConfigMap missing config.yaml field")
		operatormetrics.ConfigValidationStatus.Set(0)
		r.recordEvent(ctx, "ConfigMap", ConfigMapName, "ValidationFailed", "config.yaml field is missing")
		return fmt.Errorf("ConfigMap missing config.yaml field")
	}

	// Validate configuration
	if err := ValidateConfigYAML(configYAML); err != nil {
		log.Error(err, "Configuration validation failed")
		operatormetrics.ConfigValidationStatus.Set(0)
		r.recordEvent(ctx, "ConfigMap", ConfigMapName, "ValidationFailed", err.Error())
		// Don't return error - we want to keep reconciling even with invalid config
		// Pods will use default config if config file is invalid
		return nil
	}

	log.Info("Configuration validated successfully")
	operatormetrics.ConfigValidationStatus.Set(1)

	// Check if config changed since last DaemonSet restart
	if r.shouldRestartPods(ctx, actual) {
		log.Info("Configuration changed - triggering pod restart")
		if err := r.triggerPodRestart(ctx, actual); err != nil {
			return fmt.Errorf("failed to trigger pod restart: %w", err)
		}
		operatormetrics.PodRestartTriggers.Inc()
		r.recordEvent(ctx, "DaemonSet", DaemonSetName, "ConfigUpdated", "Restarting pods to apply new configuration")
	}

	return nil
}

// shouldRestartPods checks if the DaemonSet needs to be restarted due to config changes
func (r *Reconciler) shouldRestartPods(ctx context.Context, configMap *corev1.ConfigMap) bool {
	log := log.FromContext(ctx)

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      DaemonSetName,
		Namespace: r.Namespace,
	}, ds); err != nil {
		log.Error(err, "Failed to get DaemonSet")
		return false
	}

	// Compare ConfigMap resourceVersion with annotation on DaemonSet
	lastAppliedVersion := ds.Spec.Template.Annotations[ConfigVersionAnnotation]
	currentVersion := configMap.ResourceVersion

	return lastAppliedVersion != currentVersion
}

// triggerPodRestart updates DaemonSet annotation to trigger a rolling restart
func (r *Reconciler) triggerPodRestart(ctx context.Context, configMap *corev1.ConfigMap) error {
	log := log.FromContext(ctx)

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      DaemonSetName,
		Namespace: r.Namespace,
	}, ds); err != nil {
		return err
	}

	// Update annotation to trigger pod restart
	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations[ConfigVersionAnnotation] = configMap.ResourceVersion
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	if err := r.Update(ctx, ds); err != nil {
		return fmt.Errorf("failed to update DaemonSet: %w", err)
	}

	log.Info("DaemonSet updated to trigger pod restart")
	return nil
}

// reconcileDaemonSet monitors DaemonSet health and exports metrics
func (r *Reconciler) reconcileDaemonSet(ctx context.Context) error {
	log := log.FromContext(ctx)

	actual := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      DaemonSetName,
		Namespace: r.Namespace,
	}, actual); err != nil {
		if errors.IsNotFound(err) {
			log.Info("DaemonSet not found - waiting for PKO to create it")
			return nil
		}
		return err
	}

	// Export health metrics
	operatormetrics.DaemonSetPodsDesired.Set(float64(actual.Status.DesiredNumberScheduled))
	operatormetrics.DaemonSetPodsReady.Set(float64(actual.Status.NumberReady))

	// Log health status
	log.Info("DaemonSet health status",
		"desired", actual.Status.DesiredNumberScheduled,
		"ready", actual.Status.NumberReady,
		"current", actual.Status.CurrentNumberScheduled,
		"updated", actual.Status.UpdatedNumberScheduled)

	// Check if DaemonSet is unhealthy
	if actual.Status.NumberReady < actual.Status.DesiredNumberScheduled {
		log.Info("DaemonSet not fully ready",
			"ready", actual.Status.NumberReady,
			"desired", actual.Status.DesiredNumberScheduled)
		r.recordEvent(ctx, "DaemonSet", DaemonSetName, "PodsNotReady",
			fmt.Sprintf("Only %d/%d pods are ready",
				actual.Status.NumberReady,
				actual.Status.DesiredNumberScheduled))
	}

	return nil
}

// recordEvent creates a Kubernetes event
func (r *Reconciler) recordEvent(ctx context.Context, kind, name, reason, message string) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", name, reason, time.Now().Unix()),
			Namespace: r.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      kind,
			Name:      name,
			Namespace: r.Namespace,
		},
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeWarning,
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
	}

	if err := r.Create(ctx, event); err != nil {
		log.FromContext(ctx).Error(err, "Failed to create event")
	}
}

// SetupWithManager configures the controller with watches on resources we monitor
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Handler that maps watched resources to reconciliation requests
	mapToSelf := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			}},
		}
	})

	// Predicate to filter events - only our specific resources in our namespace
	resourcePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return (e.Object.GetName() == ConfigMapName || e.Object.GetName() == DaemonSetName) &&
				e.Object.GetNamespace() == r.Namespace
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (e.ObjectNew.GetName() == ConfigMapName || e.ObjectNew.GetName() == DaemonSetName) &&
				e.ObjectNew.GetNamespace() == r.Namespace
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return (e.Object.GetName() == ConfigMapName || e.Object.GetName() == DaemonSetName) &&
				e.Object.GetNamespace() == r.Namespace
		},
	}

	// Set up the controller with watches
	// We use ConfigMap as the primary resource since we always need it
	return ctrl.NewControllerManagedBy(mgr).
		Named("ebs-exporter-operator").
		For(&corev1.ConfigMap{}).
		Watches(&appsv1.DaemonSet{}, mapToSelf).
		WithEventFilter(resourcePredicate).
		Complete(r)
}
