package types

import (
	"fmt"

	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	Snapshot    *v1alpha1.VolumeSnapshot
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
	Cmd            string
	RunAsUser      int64
	ContainerImage string
}

func (c *CreatePodArgs) Validate() error {
	if c.GenerateName == "" || c.PVCName == "" || c.Namespace == "" || c.Cmd == "" {
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
