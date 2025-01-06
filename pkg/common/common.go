package common

const (
	// VolSnapClassDriverKey describes the driver key in VolumeSnapshotClass resource
	VolSnapClassDriverKey = "driver"
	// DefaultPodImage the default pod image
	DefaultPodImage = "ghcr.io/kastenhq/kubestr:latest"
	// SnapGroupName describes the snapshot group name
	SnapGroupName = "snapshot.storage.k8s.io"
	// VolumeSnapshotClassResourcePlural  describes volume snapshot classses
	VolumeSnapshotClassResourcePlural = "volumesnapshotclasses"
	// VolumeSnapshotResourcePlural is "volumesnapshots"
	VolumeSnapshotResourcePlural = "volumesnapshots"
	// SnapshotVersion is the apiversion of the VolumeSnapshot resource
	SnapshotVersion = "snapshot.storage.k8s.io/v1"
)
