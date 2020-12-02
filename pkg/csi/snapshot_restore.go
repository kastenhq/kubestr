package csi

import (
	"context"
	"fmt"

	"github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// SnapGroupName describes the snapshot group name
	SnapGroupName = "snapshot.storage.k8s.io"
	// VolumeSnapshotClassResourcePlural  describes volume snapshot classses
	VolumeSnapshotClassResourcePlural = "volumesnapshotclasses"
	// VolSnapClassAlphaDriverKey describes alpha driver key
	VolSnapClassAlphaDriverKey = "snapshotter"
	// VolSnapClassBetaDriverKey describes beta driver key
	VolSnapClassBetaDriverKey = "driver"
	alphaVersion              = "snapshot.storage.k8s.io/v1alpha1"
	betaVersion               = "snapshot.storage.k8s.io/v1beta1"
)

type SnapshotRestoreRunner struct {
	KubeCli kubernetes.Interface
	DynCli  dynamic.Interface
	srSteps snapshotRestoreSteps
}

func (r *SnapshotRestoreRunner) RunSnapshotRestore(ctx context.Context, args *CSISnapshotRestoreArgs) (*CSISnapshotRestoreResults, error) {
	// r.srSteps = &snapshotRestoreStepper{
	// 	kubeCli: s.KubeCli,
	// 	dynCli:  s.DynCli,
	// }
	return r.RunSnapshotRestoreHelper(ctx, args)
}

func (r *SnapshotRestoreRunner) RunSnapshotRestoreHelper(ctx context.Context, args *CSISnapshotRestoreArgs) (*CSISnapshotRestoreResults, error) {
	if r.KubeCli == nil || r.DynCli == nil { // for UT purposes
		return nil, fmt.Errorf("cli uninitialized")
	}
	if err := r.srSteps.validateArgs(ctx, args); err != nil {
		return nil, err
	}

	return nil, nil
}

type snapshotRestoreSteps interface {
	validateArgs(ctx context.Context, args *CSISnapshotRestoreArgs) error
	createApplication(ctx context.Context, args *CSISnapshotRestoreArgs) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	snapshotApplication(ctx context.Context, args *CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim) (*v1alpha1.VolumeSnapshot, error)
	restoreApplication(ctx context.Context, args *CSISnapshotRestoreArgs, snapshot *v1alpha1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	cleanup(ctx context.Context)
}

type snapshotRestoreStepper struct {
	kubeCli      kubernetes.Interface
	dynCli       dynamic.Interface
	validateOps  validateOperationsInterface
	versionFetch apiVersionFetchInterface
}

func (s *snapshotRestoreStepper) validateArgs(ctx context.Context, args *CSISnapshotRestoreArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}
	if err := s.validateOps.validateNamespace(ctx, args.Namespace); err != nil {
		return err
	}
	sc, err := s.validateOps.validateStorageClass(ctx, args.StorageClass)
	if err != nil {
		return err
	}

	groupVersion, err := s.versionFetch.getCSISnapshotGroupVersion()
	if err != nil {
		return err
	}

	uVSC, err := s.validateOps.validateVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, groupVersion)
	if err != nil {
		return err
	}

	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return nil
}

// func (s *snapshotRestoreStepper) createApplication(ctx context.Context, args *CSISnapshotRestoreArgs) (*v1.Pod, *v1.PersistentVolumeClaim, error) {

// }

type validateOperationsInterface interface {
	validateNamespace(ctx context.Context, namespace string) error
	validateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error)
	validateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error)
}

type validateOperations struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (o *validateOperations) validateNamespace(ctx context.Context, namespace string) error {
	_, err := o.kubeCli.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err
}

func (o *validateOperations) validateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	return o.kubeCli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
}

func (o *validateOperations) validateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error) {
	VolSnapClassGVR := schema.GroupVersionResource{Group: SnapGroupName, Version: groupVersion.Version, Resource: VolumeSnapshotClassResourcePlural}
	return o.dynCli.Resource(VolSnapClassGVR).Get(ctx, volumeSnapshotClass, metav1.GetOptions{})
}

// type createApplicationInterface interface {
// }

// type createSnapshotInterface interface {
// }

type apiVersionFetchInterface interface {
	getCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error)
}

type apiVersionFetch struct {
	cli kubernetes.Interface
}

func (p *apiVersionFetch) getCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error) {
	groups, _, err := p.cli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Name == SnapGroupName {
			return &group.PreferredVersion, nil
		}
	}
	return nil, fmt.Errorf("Snapshot API group not found")
}

func getDriverNameFromUVSC(vsc unstructured.Unstructured, version string) string {
	var driverName interface{}
	var ok bool
	switch version {
	case alphaVersion:
		driverName, ok = vsc.Object[VolSnapClassAlphaDriverKey]
		if !ok {
			return ""
		}

	case betaVersion:
		driverName, ok = vsc.Object[VolSnapClassBetaDriverKey]
		if !ok {
			return ""
		}
	}
	driver, ok := driverName.(string)
	if !ok {
		return ""
	}
	return driver
}
