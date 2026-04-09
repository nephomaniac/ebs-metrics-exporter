package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// VolumeMetadata contains metadata about an EBS volume
type VolumeMetadata struct {
	VolumeID     string
	VolumeType   string // "root" or "pvc"
	PVCNamespace string // empty for root volumes
	PVCName      string // empty for root volumes
}

// PVCMapper handles mapping EBS volume IDs to PVC information
type PVCMapper struct {
	client *kubernetes.Clientset
}

// NewPVCMapper creates a new PVC mapper using in-cluster config
func NewPVCMapper() (*PVCMapper, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &PVCMapper{
		client: clientset,
	}, nil
}

// getKubernetesConfig tries in-cluster config first, then falls back to kubeconfig
func getKubernetesConfig() (*rest.Config, error) {
	// Try in-cluster config first (for running in pod)
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig (for local development)
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfig = kubeconfigEnv
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// GetVolumeMetadata queries the Kubernetes API to get metadata for a volume ID
func (m *PVCMapper) GetVolumeMetadata(volumeID string) (*VolumeMetadata, error) {
	ctx := context.Background()

	// List all PersistentVolumes
	pvList, err := m.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVs: %w", err)
	}

	// Search for matching volume ID
	for _, pv := range pvList.Items {
		if matchesVolumeID(&pv, volumeID) {
			return buildVolumeMetadata(&pv, volumeID), nil
		}
	}

	// No PVC found - this is a root volume
	return &VolumeMetadata{
		VolumeID:   volumeID,
		VolumeType: "root",
	}, nil
}

// GetAllVolumeMetadata returns metadata for all PVC-backed volumes
func (m *PVCMapper) GetAllVolumeMetadata() (map[string]*VolumeMetadata, error) {
	ctx := context.Background()

	pvList, err := m.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVs: %w", err)
	}

	result := make(map[string]*VolumeMetadata)

	for _, pv := range pvList.Items {
		volumeID := extractVolumeID(&pv)
		if volumeID != "" {
			result[volumeID] = buildVolumeMetadata(&pv, volumeID)
		}
	}

	return result, nil
}

// matchesVolumeID checks if a PV matches the given volume ID
func matchesVolumeID(pv *corev1.PersistentVolume, volumeID string) bool {
	return extractVolumeID(pv) == volumeID
}

// extractVolumeID extracts the EBS volume ID from a PV
func extractVolumeID(pv *corev1.PersistentVolume) string {
	// CSI volumes (modern)
	if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == "ebs.csi.aws.com" {
		return pv.Spec.CSI.VolumeHandle
	}

	// AWS EBS volumes (legacy)
	if pv.Spec.AWSElasticBlockStore != nil {
		// VolumeID format: "aws://us-west-2a/vol-xxxxx"
		// Extract just "vol-xxxxx"
		volumeID := pv.Spec.AWSElasticBlockStore.VolumeID
		if len(volumeID) > 0 {
			// Find the last '/' and take everything after it
			for i := len(volumeID) - 1; i >= 0; i-- {
				if volumeID[i] == '/' {
					return volumeID[i+1:]
				}
			}
			return volumeID
		}
	}

	return ""
}

// buildVolumeMetadata constructs VolumeMetadata from a PV
func buildVolumeMetadata(pv *corev1.PersistentVolume, volumeID string) *VolumeMetadata {
	metadata := &VolumeMetadata{
		VolumeID:   volumeID,
		VolumeType: "pvc",
	}

	// Extract PVC claim reference
	if pv.Spec.ClaimRef != nil {
		metadata.PVCNamespace = pv.Spec.ClaimRef.Namespace
		metadata.PVCName = pv.Spec.ClaimRef.Name
	}

	return metadata
}
