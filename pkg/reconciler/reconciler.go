package reconciler

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// ConfigMap containing exporter configuration
	ConfigMapName = "ebs-metrics-exporter-config"
	// DaemonSet name
	DaemonSetName = "ebs-metrics-exporter"
	// ConfigMap containing desired state
	DesiredStateConfigMapName = "ebs-metrics-exporter-desired-state"
)

// Reconciler watches exporter resources and prevents drift
type Reconciler struct {
	client    *kubernetes.Clientset
	namespace string
}

// New creates a new Reconciler
func New(namespace string) (*Reconciler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Reconciler{
		client:    client,
		namespace: namespace,
	}, nil
}

// Reconcile checks resources and reverts drift
func (r *Reconciler) Reconcile(ctx context.Context) error {
	fmt.Println("Running reconciliation...")

	// Check if desired state ConfigMap exists
	desiredCM, err := r.client.CoreV1().ConfigMaps(r.namespace).Get(ctx, DesiredStateConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("Desired state ConfigMap not found - skipping reconciliation\n")
			return nil
		}
		return fmt.Errorf("failed to get desired state ConfigMap: %w", err)
	}

	// Reconcile exporter ConfigMap
	if err := r.reconcileConfigMap(ctx, desiredCM); err != nil {
		return fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	// Reconcile DaemonSet
	if err := r.reconcileDaemonSet(ctx, desiredCM); err != nil {
		return fmt.Errorf("failed to reconcile DaemonSet: %w", err)
	}

	return nil
}

// reconcileConfigMap ensures ConfigMap matches desired state
func (r *Reconciler) reconcileConfigMap(ctx context.Context, desired *corev1.ConfigMap) error {
	actual, err := r.client.CoreV1().ConfigMaps(r.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("ConfigMap not found - PKO will restore via ownerReferences\n")
			return nil
		}
		return err
	}

	// Compare data fields only
	if !reflect.DeepEqual(actual.Data, desired.Data) {
		fmt.Printf("DRIFT DETECTED: ConfigMap data modified\n")
		fmt.Printf("  Actual keys: %v\n", getKeys(actual.Data))
		fmt.Printf("  Desired keys: %v\n", getKeys(desired.Data))

		// Revert to desired state
		actual.Data = desired.Data
		_, err := r.client.CoreV1().ConfigMaps(r.namespace).Update(ctx, actual, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}

		// Create event
		r.recordEvent(ctx, "ConfigMap", ConfigMapName, "DriftReverted", "ConfigMap data reverted to package version")
		fmt.Printf("  ✅ ConfigMap reconciled\n")
	}

	return nil
}

// reconcileDaemonSet ensures DaemonSet critical fields match desired state
func (r *Reconciler) reconcileDaemonSet(ctx context.Context, desiredCM *corev1.ConfigMap) error {
	actual, err := r.client.AppsV1().DaemonSets(r.namespace).Get(ctx, DaemonSetName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("DaemonSet not found - PKO will restore via ownerReferences\n")
			return nil
		}
		return err
	}

	driftDetected := false

	// Check if image was changed
	if desiredImage, ok := desiredCM.Data["image"]; ok && len(actual.Spec.Template.Spec.Containers) > 0 {
		actualImage := actual.Spec.Template.Spec.Containers[0].Image

		if actualImage != desiredImage {
			fmt.Printf("DRIFT DETECTED: DaemonSet image modified\n")
			fmt.Printf("  Actual: %s\n", actualImage)
			fmt.Printf("  Desired: %s\n", desiredImage)

			actual.Spec.Template.Spec.Containers[0].Image = desiredImage
			driftDetected = true
		}
	}

	if driftDetected {
		_, err := r.client.AppsV1().DaemonSets(r.namespace).Update(ctx, actual, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update DaemonSet: %w", err)
		}

		r.recordEvent(ctx, "DaemonSet", DaemonSetName, "DriftReverted", "DaemonSet reverted to package version")
		fmt.Printf("  ✅ DaemonSet reconciled\n")
	}

	return nil
}

// recordEvent creates a Kubernetes event
func (r *Reconciler) recordEvent(ctx context.Context, kind, name, reason, message string) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-drift-%d", name, metav1.Now().Unix()),
			Namespace: r.namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      kind,
			Name:      name,
			Namespace: r.namespace,
		},
		Reason:  reason,
		Message: message,
		Type:    corev1.EventTypeWarning,
		EventTime: metav1.NowMicro(),
	}

	_, err := r.client.CoreV1().Events(r.namespace).Create(ctx, event, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("WARNING: Failed to create event: %v\n", err)
	}
}

// getKeys returns sorted keys from a map
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
