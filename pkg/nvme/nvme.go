package nvme

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// NVMe IOCTL constants
const (
	NVMEAdminIdentify  = 0x06
	NVMEGetLogPage     = 0x02
	NVMEIoctlAdminCmd  = 0xC0484E41
	AmznNVMEEBSMN      = "Amazon Elastic Block Store"
	AmznNVMEStatsLogID = 0xD0
	AmznNVMEStatsMagic = 0x3C23B510
	AmznNVMEVID        = 0x1D0F
)

// nvmeAdminCommand represents the NVMe admin command structure
type nvmeAdminCommand struct {
	Opcode     uint8
	Flags      uint8
	CID        uint16
	NSID       uint32
	Reserved0  uint64
	MPTR       uint64
	Addr       uint64
	MLen       uint32
	ALen       uint32
	CDW10      uint32
	CDW11      uint32
	CDW12      uint32
	CDW13      uint32
	CDW14      uint32
	CDW15      uint32
	Reserved1  uint64
}

// nvmeIdentifyController represents the NVMe Identify Controller structure
type nvmeIdentifyController struct {
	VID       uint16
	SSVID     uint16
	SN        [20]byte
	MN        [40]byte
	FR        [8]byte
	RAB       uint8
	IEEE      [3]uint8
	MIC       uint8
	MDTS      uint8
	Reserved0 [256 - 78]uint8
	OACS      uint16
	ACL       uint8
	AERL      uint8
	FRMW      uint8
	LPA       uint8
	ELPE      uint8
	NPSS      uint8
	AVSCC     uint8
	Reserved1 [512 - 265]uint8
	SQES      uint8
	CQES      uint8
	Reserved2 uint16
	NN        uint32
	ONCS      uint16
	FUSES     uint16
	FNA       uint8
	VWC       uint8
	AWUN      uint16
	AWUPF     uint16
	NVSCC     uint8
	Reserved3 [704 - 531]uint8
	Reserved4 [2048 - 704]uint8
	PSD       [32 * 32]uint8
	VS        [1024]uint8 // Vendor Specific
}

// nvmeHistogramBin represents a histogram bin
type nvmeHistogramBin struct {
	Lower     uint64
	Upper     uint64
	Count     uint32
	Reserved0 uint32
}

// ebsNVMEHistogram represents the EBS NVMe histogram structure
type ebsNVMEHistogram struct {
	NumBins uint64
	Bins    [64]nvmeHistogramBin
}

// EBSNVMEStats represents the Amazon EBS NVMe statistics
type EBSNVMEStats struct {
	Magic                             uint32
	Reserved0                         [4]byte
	TotalReadOps                      uint64
	TotalWriteOps                     uint64
	TotalReadBytes                    uint64
	TotalWriteBytes                   uint64
	TotalReadTime                     uint64
	TotalWriteTime                    uint64
	EBSVolumePerformanceExceededIOPS  uint64
	EBSVolumePerformanceExceededTP    uint64
	EBSInstancePerformanceExceededIOPS uint64
	EBSInstancePerformanceExceededTP   uint64
	VolumeQueueLength                 uint64
	Reserved1                         [416]byte
	ReadIOLatencyHistogram            ebsNVMEHistogram
	WriteIOLatencyHistogram           ebsNVMEHistogram
	Reserved2                         [496]byte
}

// Device represents an NVMe EBS device
type Device struct {
	Path     string
	VolumeID string
}

// nvmeIOCTL performs an NVMe IOCTL command
func nvmeIOCTL(device *os.File, cmd *nvmeAdminCommand) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		device.Fd(),
		uintptr(NVMEIoctlAdminCmd),
		uintptr(unsafe.Pointer(cmd)),
	)
	if errno != 0 {
		return fmt.Errorf("ioctl failed: %v", errno)
	}
	return nil
}

// OpenDevice opens an NVMe device and retrieves its volume ID
func OpenDevice(devicePath string) (*Device, error) {
	// Open device for reading
	dev, err := os.Open(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open device %s: %w", devicePath, err)
	}
	defer dev.Close()

	// Get volume ID
	volumeID, err := getVolumeID(dev)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume ID: %w", err)
	}

	return &Device{
		Path:     devicePath,
		VolumeID: volumeID,
	}, nil
}

// getVolumeID retrieves the EBS volume ID from the device
func getVolumeID(dev *os.File) (string, error) {
	var idCtrl nvmeIdentifyController
	cmd := nvmeAdminCommand{
		Opcode: NVMEAdminIdentify,
		Addr:   uint64(uintptr(unsafe.Pointer(&idCtrl))),
		ALen:   uint32(unsafe.Sizeof(idCtrl)),
		CDW10:  1,
	}

	if err := nvmeIOCTL(dev, &cmd); err != nil {
		return "", fmt.Errorf("identify controller failed: %w", err)
	}

	// Verify it's an Amazon EBS device
	if idCtrl.VID != AmznNVMEVID {
		return "", fmt.Errorf("not an Amazon NVMe device (VID: 0x%x)", idCtrl.VID)
	}

	mn := strings.TrimSpace(string(bytes.Trim(idCtrl.MN[:], "\x00")))
	if mn != AmznNVMEEBSMN {
		return "", fmt.Errorf("not an EBS device (model: %s)", mn)
	}

	// Extract volume ID from serial number
	sn := strings.TrimSpace(string(bytes.Trim(idCtrl.SN[:], "\x00")))
	vol := sn
	if strings.HasPrefix(vol, "vol") && len(vol) > 3 && vol[3] != '-' {
		vol = "vol-" + vol[3:]
	}

	return vol, nil
}

// QueryStats queries EBS performance statistics from the device
func (d *Device) QueryStats() (*EBSNVMEStats, error) {
	dev, err := os.Open(d.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %w", err)
	}
	defer dev.Close()

	var stats EBSNVMEStats
	cmd := nvmeAdminCommand{
		Opcode: NVMEGetLogPage,
		Addr:   uint64(uintptr(unsafe.Pointer(&stats))),
		ALen:   uint32(unsafe.Sizeof(stats)),
		NSID:   1,
		CDW10:  AmznNVMEStatsLogID | (1024 << 16),
	}

	if err := nvmeIOCTL(dev, &cmd); err != nil {
		return nil, fmt.Errorf("get log page failed: %w", err)
	}

	// Verify magic number
	if stats.Magic != AmznNVMEStatsMagic {
		return nil, fmt.Errorf("invalid stats magic number: 0x%x (expected 0x%x)", stats.Magic, AmznNVMEStatsMagic)
	}

	return &stats, nil
}

// MustOpenDevice opens a device and panics on error (for initialization)
func MustOpenDevice(devicePath string) *Device {
	dev, err := OpenDevice(devicePath)
	if err != nil {
		panic(fmt.Sprintf("failed to open device: %v", err))
	}
	return dev
}
