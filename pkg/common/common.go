package common

const (
	// VolSnapClassAlphaDriverKey describes alpha driver key
	VolSnapClassAlphaDriverKey = "snapshotter"
	// VolSnapClassBetaDriverKey describes beta driver key
	VolSnapClassBetaDriverKey = "driver"
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
	// SnapshotAlphaVersion is the apiversion of the alpha relase
	SnapshotAlphaVersion = "snapshot.storage.k8s.io/v1alpha1"
	// SnapshotBetaVersion is the apiversion of the beta relase
	SnapshotBetaVersion = "snapshot.storage.k8s.io/v1beta1"
	// SnapshotStableVersion is the apiversion of the stable release
	SnapshotStableVersion = "snapshot.storage.k8s.io/v1"
)
