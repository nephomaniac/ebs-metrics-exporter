package daemonset

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/metrics"
)

const (
	logName           = "daemonset-controller"
	recheckInterval   = 30 * time.Second
)

var log = logf.Log.WithName(logName)

// DaemonSetReconciler reconciles EBS metrics exporter DaemonSet
type DaemonSetReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	MetricsAggregator *metrics.EBSMetricsAggregator
	ClusterId         string
}

// Reconcile handles DaemonSet state changes
func (r *DaemonSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling DaemonSet")
	
	defer func() {
		reqLogger.Info("Reconcile Complete")
	}()

	// Fetch the DaemonSet
	daemonSet := &appsv1.DaemonSet{}
	err := r.Get(ctx, req.NamespacedName, daemonSet)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("DaemonSet not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get DaemonSet")
		return ctrl.Result{}, err
	}

	// Monitor DaemonSet status and expose operational metrics
	// This can include: number of ready pods, desired pods, available pods, etc.
	reqLogger.Info("DaemonSet Status",
		"DesiredNumberScheduled", daemonSet.Status.DesiredNumberScheduled,
		"CurrentNumberScheduled", daemonSet.Status.CurrentNumberScheduled,
		"NumberReady", daemonSet.Status.NumberReady,
		"NumberAvailable", daemonSet.Status.NumberAvailable,
	)

	// Fetch all pods managed by this DaemonSet
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(req.Namespace),
		client.MatchingLabels(daemonSet.Spec.Selector.MatchLabels),
	}
	
	if err := r.List(ctx, podList, listOpts...); err != nil {
		reqLogger.Error(err, "Failed to list pods")
		return ctrl.Result{RequeueAfter: recheckInterval}, err
	}

	reqLogger.Info("Found pods", "count", len(podList.Items))
	
	// Process each pod - in the future, this could scrape metrics from each pod's endpoint
	// For now, we just log the pod status
	for _, pod := range podList.Items {
		reqLogger.Info("Pod status",
			"name", pod.Name,
			"node", pod.Spec.NodeName,
			"phase", pod.Status.Phase,
			"ready", isPodReady(&pod),
		)
	}

	// Requeue to continuously monitor the DaemonSet
	return ctrl.Result{RequeueAfter: recheckInterval}, nil
}

// isPodReady checks if a pod is ready
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager
func (r *DaemonSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.DaemonSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
