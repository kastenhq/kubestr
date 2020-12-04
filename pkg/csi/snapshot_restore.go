package csi

import (
	"context"
	"fmt"
	"time"

	kankube "github.com/kanisterio/kanister/pkg/kube"
	kansnapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	originalPVCGenerateName   = "kubestr-csi-original-pvc"
	originalPodGenerateName   = "kubestr-csi-original-pod"
	clonedPVCGenerateName     = "kubestr-csi-cloned-pvc"
	clonedPodGenerateName     = "kubestr-csi-cloned-pod"
	createdByLabel            = "created-by-kubestr-csi"
	DefaultPodImage           = "ghcr.io/kastenhq/kubestr:latest"
	clonePrefix               = "kubestr-clone-"
)

type SnapshotRestoreRunner struct {
	KubeCli kubernetes.Interface
	DynCli  dynamic.Interface
	srSteps snapshotRestoreStepper
}

func (r *SnapshotRestoreRunner) RunSnapshotRestore(ctx context.Context, args *types.CSISnapshotRestoreArgs) (*types.CSISnapshotRestoreResults, error) {
	// r.srSteps = &snapshotRestoreStepper{
	// 	kubeCli: s.KubeCli,
	// 	dynCli:  s.DynCli,
	// }
	return r.RunSnapshotRestoreHelper(ctx, args)
}

func (r *SnapshotRestoreRunner) RunSnapshotRestoreHelper(ctx context.Context, args *types.CSISnapshotRestoreArgs) (*types.CSISnapshotRestoreResults, error) {
	results := &types.CSISnapshotRestoreResults{}
	var err error
	if r.KubeCli == nil || r.DynCli == nil { // for UT purposes
		return results, fmt.Errorf("cli uninitialized")
	}
	if err := r.srSteps.validateArgs(ctx, args); err != nil {
		return results, errors.Wrap(err, "Failed to validate arguments.")
	}
	data := time.Now().Format("2006-01-02 15:04:05")
	results.OriginalPod, results.OriginalPVC, err = r.srSteps.createApplication(ctx, args, data)

	if err == nil {
		err = r.srSteps.validateData(ctx, args, results.OriginalPod, data)
	}

	if err == nil {
		results.Snapshot, err = r.srSteps.snapshotApplication(ctx, args, results.OriginalPVC, "snapshot")
	}

	if err == nil {
		results.ClonedPod, results.ClonedPVC, err = r.srSteps.restoreApplication(ctx, args, results.Snapshot)
	}

	r.srSteps.cleanup(ctx, args, results)

	return results, err
}

type snapshotRestoreStepper interface {
	validateArgs(ctx context.Context, args *types.CSISnapshotRestoreArgs) error
	createApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, data string) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	validateData(ctx context.Context, args *types.CSISnapshotRestoreArgs, pod *v1.Pod, data string) error
	snapshotApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim, snapshotName string) (*v1alpha1.VolumeSnapshot, error)
	restoreApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, snapshot *v1alpha1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	cleanup(ctx context.Context, args *types.CSISnapshotRestoreArgs, results *types.CSISnapshotRestoreResults)
}

type snapshotRestoreSteps struct {
	kubeCli           kubernetes.Interface
	dynCli            dynamic.Interface
	validateOps       ArgumentValidator
	versionFetch      ApiVersionFetcher
	createAppOps      ApplicationCreator
	snapshotCreateOps SnapshotCreator
}

func (s *snapshotRestoreSteps) validateArgs(ctx context.Context, args *types.CSISnapshotRestoreArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}
	if err := s.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return err
	}
	sc, err := s.validateOps.ValidateStorageClass(ctx, args.StorageClass)
	if err != nil {
		return err
	}

	groupVersion, err := s.versionFetch.GetCSISnapshotGroupVersion()
	if err != nil {
		return err
	}

	uVSC, err := s.validateOps.ValidateVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, groupVersion)
	if err != nil {
		return err
	}

	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return nil
}

func (s *snapshotRestoreSteps) createApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, genString string) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
	pvcArgs := &types.CreatePVCArgs{
		GenerateName: originalPVCGenerateName,
		StorageClass: args.StorageClass,
		Namespace:    args.Namespace,
	}
	pvc, err := s.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, err
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   originalPodGenerateName,
		PVCName:        pvc.Name,
		Namespace:      args.Namespace,
		Cmd:            fmt.Sprintf("echo '%s' >> /data/out.txt; sync; tail -f /dev/null", genString),
		RunAsUser:      args.RunAsUser,
		ContainerImage: args.ContainerImage,
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, err
	}
	err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name)
	return pod, pvc, err
}

func (s *snapshotRestoreSteps) validateData(ctx context.Context, args, pod *v1.Pod, data string) error {
	stdout, _, err := kankube.Exec(s.kubeCli, args.Namespace, pod.Name, "", []string{"sh", "-c", "cat /data/out.txt"}, nil)
	if err != nil {
		return err
	}
	if stdout != data {
		return fmt.Errorf("string didn't match (%s , %s)", stdout, data)
	}
	return nil
}

func (s *snapshotRestoreSteps) snapshotApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim, snapshotName string) (*v1alpha1.VolumeSnapshot, error) {
	snapshotter, err := s.snapshotCreateOps.NewSnapshotter()
	if err != nil {
		return nil, err
	}
	createSnapshotArgs := &types.CreateSnapshotArgs{
		Namespace:           args.Namespace,
		PVCName:             pvc.Name,
		VolumeSnapshotClass: args.VolumeSnapshotClass,
		SnapshotName:        snapshotName,
	}
	snapshot, err := s.snapshotCreateOps.CreateSnapshot(ctx, snapshotter, createSnapshotArgs)
	if err != nil {
		return nil, err
	}
	cfsArgs := &types.CreateFromSourceCheckArgs{
		VolumeSnapshotClass: args.VolumeSnapshotClass,
		SnapshotName:        snapshot.Name,
		Namespace:           args.Namespace,
	}
	err = s.snapshotCreateOps.CreateFromSourceCheck(ctx, snapshotter, cfsArgs)
	return snapshot, err
}

func (s *snapshotRestoreSteps) restoreApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, snapshot *v1alpha1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	snapshotKind := "VolumeSnapshot"
	dataSource := &v1.TypedLocalObjectReference{
		APIGroup: &snapshotAPIGroup,
		Kind:     snapshotKind,
		Name:     snapshot.Name,
	}
	pvcArgs := &types.CreatePVCArgs{
		GenerateName: clonedPVCGenerateName,
		StorageClass: args.StorageClass,
		Namespace:    args.Namespace,
		DataSource:   dataSource,
		RestoreSize:  snapshot.Status.RestoreSize,
	}
	pvc, err := s.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, err
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   clonedPodGenerateName,
		PVCName:        pvc.Name,
		Namespace:      args.Namespace,
		Cmd:            "tail -f /dev/null",
		RunAsUser:      args.RunAsUser,
		ContainerImage: args.ContainerImage,
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, err
	}
	err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name)
	return pod, pvc, err
}

//go:generate mockgen -destination=mocks/mock_argument_validator.go -package=mocks . ArgumentValidator
type ArgumentValidator interface {
	ValidateNamespace(ctx context.Context, namespace string) error
	ValidateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error)
	ValidateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error)
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

//go:generate mockgen -destination=mocks/mock_application_creator.go -package=mocks . ApplicationCreator
type ApplicationCreator interface {
	CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error)
	CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error)
	WaitForPodReady(ctx context.Context, namespace string, podName string) error
}

type applicationCreate struct {
	cli kubernetes.Interface
}

func (c *applicationCreate) CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.GenerateName,
			Namespace:    args.Namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			StorageClassName: &args.StorageClass,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	if args.DataSource != nil {
		pvc.Spec.DataSource = args.DataSource
	}

	if args.RestoreSize != nil && !args.RestoreSize.IsZero() {
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = *args.RestoreSize
	}

	pvcRes, err := c.cli.CoreV1().PersistentVolumeClaims(args.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return pvc, err
	}

	return pvcRes, nil
}

func (c *applicationCreate) CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}
	if args.ContainerImage == "" {
		args.ContainerImage = DefaultPodImage
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.GenerateName,
			Namespace:    args.Namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    args.GenerateName,
				Image:   args.ContainerImage,
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", args.Cmd},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "persistent-storage",
					MountPath: "/data",
				}},
			}},
			Volumes: []v1.Volume{{
				Name: "persistent-storage",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: args.PVCName,
					},
				}},
			},
		},
	}

	if args.RunAsUser > 0 {
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser: &args.RunAsUser,
			FSGroup:   &args.RunAsUser,
		}
	}

	podRes, err := c.cli.CoreV1().Pods(args.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return pod, err
	}
	return podRes, nil
}

func (c *applicationCreate) WaitForPodReady(ctx context.Context, namespace string, podName string) error {
	err := kankube.WaitForPodReady(ctx, c.cli, namespace, podName)
	return err
}

//go:generate mockgen -destination=mocks/mock_snapshot_creator.go -package=mocks . SnapshotCreator
type SnapshotCreator interface {
	NewSnapshotter() (kansnapshot.Snapshotter, error)
	CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*v1alpha1.VolumeSnapshot, error)
	CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs) error
}

type snapshotCreate struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (c *snapshotCreate) NewSnapshotter() (kansnapshot.Snapshotter, error) {
	return kansnapshot.NewSnapshotter(c.kubeCli, c.dynCli)
}

func (c *snapshotCreate) CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*v1alpha1.VolumeSnapshot, error) {
	if snapshotter == nil || args == nil {
		return nil, fmt.Errorf("snapshotter or args are empty")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	err := snapshotter.Create(ctx, args.SnapshotName, args.Namespace, args.PVCName, &args.VolumeSnapshotClass, true)
	if err != nil {
		return nil, errors.Wrapf(err, "CSI Driver failed to create snapshot for PVC (%s) in Namspace (%s)", args.PVCName, args.Namespace)
	}
	snap, err := snapshotter.Get(ctx, args.SnapshotName, args.Namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get CSI snapshot (%s) in Namespace (%s)", args.SnapshotName, args.Namespace)
	}
	return snap, nil
}

func (c *snapshotCreate) CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs) error {
	if snapshotter == nil || args == nil {
		return fmt.Errorf("snapshotter or args are nil")
	}
	if err := args.Validate(); err != nil {
		return err
	}
	targetSnapClassName := clonePrefix + args.VolumeSnapshotClass
	err := snapshotter.CloneVolumeSnapshotClass(args.VolumeSnapshotClass, targetSnapClassName, kansnapshot.DeletionPolicyRetain, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to create a VolumeSnapshotClass to use to restore the snapshot")
	}
	defer func() {
		_ = c.dynCli.Resource(v1alpha1.VolSnapClassGVR).Delete(ctx, targetSnapClassName, metav1.DeleteOptions{})
	}()

	snapSrc, err := snapshotter.GetSource(ctx, args.SnapshotName, args.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source snapshot source (%s)", args.SnapshotName)
	}
	snapshotCFSCloneName := clonePrefix + args.SnapshotName
	// test the CreateFromSource API
	defer func() {
		_, _ = snapshotter.Delete(context.Background(), snapshotCFSCloneName, args.Namespace)
	}()
	src := &kansnapshot.Source{
		Handle:                  snapSrc.Handle,
		Driver:                  snapSrc.Driver,
		VolumeSnapshotClassName: targetSnapClassName,
	}
	err = snapshotter.CreateFromSource(ctx, src, snapshotCFSCloneName, args.Namespace, true)
	if err != nil {
		return errors.Wrapf(err, "Failed to clone snapshot from source (%s)", snapshotCFSCloneName)
	}
	return nil
}

//go:generate mockgen -destination=mocks/mock_api_version_fetcher.go -package=mocks . ApiVersionFetcher
type ApiVersionFetcher interface {
	GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error)
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
