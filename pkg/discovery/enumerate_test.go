package discovery

import (
	"testing"
)

func TestIsNVMeBlockDevice(t *testing.T) {
	tests := []struct {
		name       string
		devicePath string
		want       bool
	}{
		{
			name:       "nvme0n1 block device",
			devicePath: "/dev/nvme0n1",
			want:       true,
		},
		{
			name:       "nvme1n1 block device",
			devicePath: "/dev/nvme1n1",
			want:       true,
		},
		{
			name:       "nvme10n1 multi-digit controller",
			devicePath: "/dev/nvme10n1",
			want:       true,
		},
		{
			name:       "nvme0n2 different namespace",
			devicePath: "/dev/nvme0n2",
			want:       true,
		},
		{
			name:       "nvme0n1p1 partition - should reject",
			devicePath: "/dev/nvme0n1p1",
			want:       false,
		},
		{
			name:       "nvme0n1p2 partition - should reject",
			devicePath: "/dev/nvme0n1p2",
			want:       false,
		},
		{
			name:       "nvme1n1p10 multi-digit partition - should reject",
			devicePath: "/dev/nvme1n1p10",
			want:       false,
		},
		{
			name:       "host prefix path",
			devicePath: "/host/dev/nvme0n1",
			want:       true,
		},
		{
			name:       "host prefix partition - should reject",
			devicePath: "/host/dev/nvme0n1p1",
			want:       false,
		},
		{
			name:       "sda not nvme - should reject",
			devicePath: "/dev/sda",
			want:       false,
		},
		{
			name:       "sda1 partition - should reject",
			devicePath: "/dev/sda1",
			want:       false,
		},
		{
			name:       "nvme with invalid format - should reject",
			devicePath: "/dev/nvme",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNVMeBlockDevice(tt.devicePath)
			if got != tt.want {
				t.Errorf("IsNVMeBlockDevice(%q) = %v, want %v", tt.devicePath, got, tt.want)
			}
		})
	}
}

func TestStripHostPrefix(t *testing.T) {
	tests := []struct {
		name       string
		devicePath string
		want       string
	}{
		{
			name:       "with /host prefix",
			devicePath: "/host/dev/nvme0n1",
			want:       "/dev/nvme0n1",
		},
		{
			name:       "without /host prefix",
			devicePath: "/dev/nvme0n1",
			want:       "/dev/nvme0n1",
		},
		{
			name:       "empty path",
			devicePath: "",
			want:       "",
		},
		{
			name:       "short path",
			devicePath: "/dev",
			want:       "/dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHostPrefix(tt.devicePath)
			if got != tt.want {
				t.Errorf("StripHostPrefix(%q) = %q, want %q", tt.devicePath, got, tt.want)
			}
		})
	}
}

func TestAddHostPrefix(t *testing.T) {
	tests := []struct {
		name       string
		devicePath string
		want       string
	}{
		{
			name:       "without /host prefix",
			devicePath: "/dev/nvme0n1",
			want:       "/host/dev/nvme0n1",
		},
		{
			name:       "with /host prefix already",
			devicePath: "/host/dev/nvme0n1",
			want:       "/host/dev/nvme0n1",
		},
		{
			name:       "empty path",
			devicePath: "",
			want:       "/host",
		},
		{
			name:       "relative path",
			devicePath: "dev/nvme0n1",
			want:       "/hostdev/nvme0n1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddHostPrefix(tt.devicePath)
			if got != tt.want {
				t.Errorf("AddHostPrefix(%q) = %q, want %q", tt.devicePath, got, tt.want)
			}
		})
	}
}
