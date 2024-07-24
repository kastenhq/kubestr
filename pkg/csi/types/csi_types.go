package types

import (
	"bytes"
	"fmt"
	"time"

	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/rest"
)

type CSISnapshotRestoreArgs struct {
	StorageClass          string
	VolumeSnapshotClass   string
	Namespace             string
	RunAsUser             int64
	ContainerImage        string
	Cleanup               bool
	SkipCFSCheck          bool
	K8sObjectReadyTimeout time.Duration
}

func (a *CSISnapshotRestoreArgs) Validate() error {
	if a.StorageClass == "" || a.VolumeSnapshotClass == "" || a.Namespace == "" {
		return fmt.Errorf("Require fields are missing. (StorageClass, VolumeSnapshotClass, Namespace)")
	}
	return nil
}

type CSISnapshotRestoreResults struct {
	OriginalPVC *v1.PersistentVolumeClaim
	OriginalPod *v1.Pod
	Snapshot    *snapv1.VolumeSnapshot
	ClonedPVC   *v1.PersistentVolumeClaim
	ClonedPod   *v1.Pod
}

type CreatePVCArgs struct {
	Name         string // Only one of Name or
	GenerateName string // GenerateName should be specified.
	StorageClass string
	Namespace    string
	DataSource   *v1.TypedLocalObjectReference
	RestoreSize  *resource.Quantity
	VolumeMode   *v1.PersistentVolumeMode // missing implies v1.PersistentVolumeFilesystem
}

func (c *CreatePVCArgs) Validate() error {
	if (c.GenerateName == "" && c.Name == "") ||
		(c.GenerateName != "" && c.Name != "") ||
		c.StorageClass == "" || c.Namespace == "" {
		return fmt.Errorf("Invalid CreatePVCArgs (%#v)", c)
	}
	return nil
}

type CreatePodArgs struct {
	Name           string // Only one of Name or
	GenerateName   string // GenerateName should be specified.
	PVCName        string
	Namespace      string
	RunAsUser      int64
	ContainerImage string
	Command        []string
	ContainerArgs  []string
	MountPath      string // Only one of MountPath or
	DevicePath     string // DevicePath should be specified.
}

func (c *CreatePodArgs) Validate() error {
	if (c.GenerateName == "" && c.Name == "") ||
		(c.GenerateName != "" && c.Name != "") ||
		(c.MountPath == "" && c.DevicePath == "") ||
		(c.MountPath != "" && c.DevicePath != "") ||
		c.PVCName == "" || c.Namespace == "" {
		return fmt.Errorf("Invalid CreatePodArgs (%#v)", c)
	}
	return nil
}

type CreateSnapshotArgs struct {
	Namespace           string
	PVCName             string
	VolumeSnapshotClass string
	SnapshotName        string
}

func (c *CreateSnapshotArgs) Validate() error {
	if c.Namespace == "" || c.PVCName == "" || c.VolumeSnapshotClass == "" || c.SnapshotName == "" {
		return fmt.Errorf("Invalid CreateSnapshotArgs (%v)", c)
	}
	return nil
}

type FetchSnapshotArgs struct {
	Namespace    string
	SnapshotName string
}

func (c *FetchSnapshotArgs) Validate() error {
	if c.Namespace == "" || c.SnapshotName == "" {
		return fmt.Errorf("Invalid FetchSnapshotArgs (%v)", c)
	}
	return nil
}

type CreateFromSourceCheckArgs struct {
	VolumeSnapshotClass string
	SnapshotName        string
	Namespace           string
}

func (c *CreateFromSourceCheckArgs) Validate() error {
	if c.VolumeSnapshotClass == "" || c.SnapshotName == "" || c.Namespace == "" {
		return fmt.Errorf("Invalid CreateFromSourceCheckArgs (%v)", c)
	}
	return nil
}

type PVCBrowseArgs struct {
	PVCName             string
	Namespace           string
	VolumeSnapshotClass string
	RunAsUser           int64
	LocalPort           int
	ShowTree            bool
}

func (p *PVCBrowseArgs) Validate() error {
	if p.PVCName == "" || p.Namespace == "" || p.VolumeSnapshotClass == "" {
		return fmt.Errorf("Invalid PVCBrowseArgs (%v)", p)
	}
	return nil
}

type SnapshotBrowseArgs struct {
	SnapshotName     string
	Namespace        string
	StorageClassName string
	RunAsUser        int64
	LocalPort        int
	ShowTree         bool
}

func (p *SnapshotBrowseArgs) Validate() error {
	if p.SnapshotName == "" || p.Namespace == "" || p.StorageClassName == "" {
		return fmt.Errorf("Invalid SnapshotBrowseArgs (%v)", p)
	}
	return nil
}

type PortForwardAPodRequest struct {
	// RestConfig is the kubernetes config
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod *v1.Pod
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// PodPort is the target port for the pod
	PodPort int
	// Streams configures where to write or read input from
	OutStream    bytes.Buffer
	ErrOutStream bytes.Buffer
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}
