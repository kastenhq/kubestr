package csi

import (
	"context"
	"fmt"
)

type CSI interface {
	RunSnapshotRestore(ctx context.Context, args *CSISnapshotRestoreArgs) (*CSISnapshotRestoreResults, error)
}

type CSISnapshotRestoreArgs struct {
	StorageClass        string
	VolumeSnapshotClass string
	Namespace           string
	RunAsUser           int64
	ContainerImage      string
}

func (a *CSISnapshotRestoreArgs) Validate() error {
	if a.StorageClass == "" || a.VolumeSnapshotClass == "" || a.Namespace == "" {
		return fmt.Errorf("Require fields are missing. (StorageClass, VolumeSnapshotClass, Namespace)")
	}
	return nil
}

type CSISnapshotRestoreResults struct{}
