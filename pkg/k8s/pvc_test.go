package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestExtractVolumeID_CSI(t *testing.T) {
	pv := &corev1.PersistentVolume{
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "ebs.csi.aws.com",
					VolumeHandle: "vol-abc123",
				},
			},
		},
	}

	volumeID := extractVolumeID(pv)
	if volumeID != "vol-abc123" {
		t.Errorf("extractVolumeID() = %q, want %q", volumeID, "vol-abc123")
	}
}

func TestExtractVolumeID_Legacy(t *testing.T) {
	tests := []struct {
		name     string
		volumeID string
		want     string
	}{
		{
			name:     "full_path",
			volumeID: "aws://us-west-2a/vol-def456",
			want:     "vol-def456",
		},
		{
			name:     "short_path",
			volumeID: "vol-ghi789",
			want:     "vol-ghi789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						AWSElasticBlockStore: &corev1.AWSElasticBlockStoreVolumeSource{
							VolumeID: tt.volumeID,
						},
					},
				},
			}

			volumeID := extractVolumeID(pv)
			if volumeID != tt.want {
				t.Errorf("extractVolumeID() = %q, want %q", volumeID, tt.want)
			}
		})
	}
}

func TestExtractVolumeID_NoEBS(t *testing.T) {
	pv := &corev1.PersistentVolume{
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/mnt/data",
				},
			},
		},
	}

	volumeID := extractVolumeID(pv)
	if volumeID != "" {
		t.Errorf("extractVolumeID() = %q, want empty string for non-EBS volume", volumeID)
	}
}

func TestMatchesVolumeID(t *testing.T) {
	pv := &corev1.PersistentVolume{
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "ebs.csi.aws.com",
					VolumeHandle: "vol-test123",
				},
			},
		},
	}

	tests := []struct {
		name     string
		volumeID string
		want     bool
	}{
		{"matches", "vol-test123", true},
		{"no_match", "vol-other456", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesVolumeID(pv, tt.volumeID)
			if result != tt.want {
				t.Errorf("matchesVolumeID() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestBuildVolumeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		pv       *corev1.PersistentVolume
		volumeID string
		want     *VolumeMetadata
	}{
		{
			name: "pvc_with_claim",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef: &corev1.ObjectReference{
						Namespace: "default",
						Name:      "my-pvc",
					},
				},
			},
			volumeID: "vol-abc123",
			want: &VolumeMetadata{
				VolumeID:     "vol-abc123",
				VolumeType:   "pvc",
				PVCNamespace: "default",
				PVCName:      "my-pvc",
			},
		},
		{
			name: "pvc_without_claim",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef: nil,
				},
			},
			volumeID: "vol-def456",
			want: &VolumeMetadata{
				VolumeID:     "vol-def456",
				VolumeType:   "pvc",
				PVCNamespace: "",
				PVCName:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildVolumeMetadata(tt.pv, tt.volumeID)
			if result.VolumeID != tt.want.VolumeID {
				t.Errorf("VolumeID = %q, want %q", result.VolumeID, tt.want.VolumeID)
			}
			if result.VolumeType != tt.want.VolumeType {
				t.Errorf("VolumeType = %q, want %q", result.VolumeType, tt.want.VolumeType)
			}
			if result.PVCNamespace != tt.want.PVCNamespace {
				t.Errorf("PVCNamespace = %q, want %q", result.PVCNamespace, tt.want.PVCNamespace)
			}
			if result.PVCName != tt.want.PVCName {
				t.Errorf("PVCName = %q, want %q", result.PVCName, tt.want.PVCName)
			}
		})
	}
}
