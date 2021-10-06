package types

import (
	"bytes"
	"fmt"

	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/rest"
)

type CSISnapshotRestoreArgs struct {
	StorageClass        string
	VolumeSnapshotClass string
	Namespace           string
	RunAsUser           int64
	ContainerImage      string
	Cleanup             bool
	SkipCFSCheck        bool
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
	GenerateName string
	StorageClass string
	Namespace    string
	DataSource   *v1.TypedLocalObjectReference
	RestoreSize  *resource.Quantity
}

func (c *CreatePVCArgs) Validate() error {
	if c.GenerateName == "" || c.StorageClass == "" || c.Namespace == "" {
		return fmt.Errorf("Invalid CreatePVCArgs (%v)", c)
	}
	return nil
}

type CreatePodArgs struct {
	GenerateName   string
	PVCName        string
	Namespace      string
	RunAsUser      int64
	ContainerImage string
	Command        []string
	ContainerArgs  []string
	MountPath      string
}

func (c *CreatePodArgs) Validate() error {
	if c.GenerateName == "" || c.PVCName == "" || c.Namespace == "" {
		return fmt.Errorf("Invalid CreatePodArgs (%v)", c)
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
}

func (p *PVCBrowseArgs) Validate() error {
	if p.PVCName == "" || p.Namespace == "" || p.VolumeSnapshotClass == "" {
		return fmt.Errorf("Invalid PVCInspectorArgs (%v)", p)
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
	// Steams configures where to write or read input from
	OutStream    bytes.Buffer
	ErrOutStream bytes.Buffer
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}
