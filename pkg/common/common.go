package common

const (
	// VolSnapClassStableDriverKey describes the stable driver key
	VolSnapClassStableDriverKey = "driver"
	// DefaultPodImage the default pod image
	DefaultPodImage = "ghcr.io/kastenhq/kubestr:latest"
	// SnapGroupName describes the snapshot group name
	SnapGroupName = "snapshot.storage.k8s.io"
	// VolumeSnapshotClassResourcePlural  describes volume snapshot classses
	VolumeSnapshotClassResourcePlural = "volumesnapshotclasses"
	// VolumeSnapshotResourcePlural is "volumesnapshots"
	VolumeSnapshotResourcePlural = "volumesnapshots"
	// SnapshotStableVersion is the apiversion of the stable release
	SnapshotStableVersion = "snapshot.storage.k8s.io/v1"
)
