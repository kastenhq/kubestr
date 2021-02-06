package csi

import (
	"context"

	"github.com/kastenhq/kubestr/pkg/csi/types"
)

type CSI interface {
	RunSnapshotRestore(ctx context.Context, args *types.CSISnapshotRestoreArgs) (*types.CSISnapshotRestoreResults, error)
}
