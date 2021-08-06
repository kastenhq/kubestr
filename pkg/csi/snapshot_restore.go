package csi

import (
	"context"
	"fmt"
	"time"

	kankube "github.com/kanisterio/kanister/pkg/kube"
	kansnapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
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
	originalPVCGenerateName = "kubestr-csi-original-pvc"
	originalPodGenerateName = "kubestr-csi-original-pod"
	clonedPVCGenerateName   = "kubestr-csi-cloned-pvc"
	clonedPodGenerateName   = "kubestr-csi-cloned-pod"
	createdByLabel          = "created-by-kubestr-csi"
	clonePrefix             = "kubestr-clone-"
	snapshotPrefix          = "kubestr-snapshot-"
)

type SnapshotRestoreRunner struct {
	KubeCli kubernetes.Interface
	DynCli  dynamic.Interface
	srSteps SnapshotRestoreStepper
}

func (r *SnapshotRestoreRunner) RunSnapshotRestore(ctx context.Context, args *types.CSISnapshotRestoreArgs) (*types.CSISnapshotRestoreResults, error) {
	r.srSteps = &snapshotRestoreSteps{
		validateOps: &validateOperations{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
		versionFetchOps: &apiVersionFetch{
			kubeCli: r.KubeCli,
		},
		createAppOps: &applicationCreate{
			kubeCli: r.KubeCli,
		},
		dataValidatorOps: &validateData{
			kubeCli: r.KubeCli,
		},
		snapshotCreateOps: &snapshotCreate{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
		cleanerOps: &cleanse{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
	}
	return r.RunSnapshotRestoreHelper(ctx, args)
}

func (r *SnapshotRestoreRunner) RunSnapshotRestoreHelper(ctx context.Context, args *types.CSISnapshotRestoreArgs) (*types.CSISnapshotRestoreResults, error) {
	results := &types.CSISnapshotRestoreResults{}
	var err error
	if r.KubeCli == nil || r.DynCli == nil {
		return results, fmt.Errorf("cli uninitialized")
	}
	if err := r.srSteps.ValidateArgs(ctx, args); err != nil {
		return results, errors.Wrap(err, "Failed to validate arguments.")
	}
	data := time.Now().Format("20060102150405")

	fmt.Println("Creating application")
	results.OriginalPod, results.OriginalPVC, err = r.srSteps.CreateApplication(ctx, args, data)

	if err == nil {
		if results.OriginalPod != nil && results.OriginalPVC != nil {
			fmt.Printf("  -> Created pod (%s) and pvc (%s)\n", results.OriginalPod.Name, results.OriginalPVC.Name)
		}
		err = r.srSteps.ValidateData(ctx, results.OriginalPod, data)
	}

	snapName := snapshotPrefix + data
	if err == nil {
		fmt.Println("Taking a snapshot")
		results.Snapshot, err = r.srSteps.SnapshotApplication(ctx, args, results.OriginalPVC, snapName)
	}

	if err == nil {
		if results.Snapshot != nil {
			fmt.Printf("  -> Created snapshot (%s)\n", results.Snapshot.Name)
		}
		fmt.Println("Restoring application")
		results.ClonedPod, results.ClonedPVC, err = r.srSteps.RestoreApplication(ctx, args, results.Snapshot)
	}

	if err == nil {
		if results.ClonedPod != nil && results.ClonedPVC != nil {
			fmt.Printf("  -> Restored pod (%s) and pvc (%s)\n", results.ClonedPod.Name, results.ClonedPVC.Name)
		}
		err = r.srSteps.ValidateData(ctx, results.ClonedPod, data)
	}

	if args.Cleanup {
		fmt.Println("Cleaning up resources")
		r.srSteps.Cleanup(ctx, results)
	}

	return results, err
}

//go:generate mockgen -destination=mocks/mock_snapshot_restore_stepper.go -package=mocks . SnapshotRestoreStepper
type SnapshotRestoreStepper interface {
	ValidateArgs(ctx context.Context, args *types.CSISnapshotRestoreArgs) error
	CreateApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, data string) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	ValidateData(ctx context.Context, pod *v1.Pod, data string) error
	SnapshotApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim, snapshotName string) (*snapv1.VolumeSnapshot, error)
	RestoreApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, snapshot *snapv1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	Cleanup(ctx context.Context, results *types.CSISnapshotRestoreResults)
}

type snapshotRestoreSteps struct {
	validateOps          ArgumentValidator
	versionFetchOps      ApiVersionFetcher
	createAppOps         ApplicationCreator
	dataValidatorOps     DataValidator
	snapshotCreateOps    SnapshotCreator
	cleanerOps           Cleaner
	SnapshotGroupVersion *metav1.GroupVersionForDiscovery
}

func (s *snapshotRestoreSteps) ValidateArgs(ctx context.Context, args *types.CSISnapshotRestoreArgs) error {
	if err := args.Validate(); err != nil {
		return errors.Wrap(err, "Failed to validate input arguments")
	}
	if err := s.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return errors.Wrap(err, "Failed to validate Namespace")
	}
	sc, err := s.validateOps.ValidateStorageClass(ctx, args.StorageClass)
	if err != nil {
		return errors.Wrap(err, "Failed to validate Storageclass")
	}

	groupVersion, err := s.versionFetchOps.GetCSISnapshotGroupVersion()
	if err != nil {
		return errors.Wrap(err, "Failed to fetch groupVersion")
	}
	s.SnapshotGroupVersion = groupVersion

	uVSC, err := s.validateOps.ValidateVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, groupVersion)
	if err != nil {
		return errors.Wrap(err, "Failed to validate VolumeSnapshotClass")
	}

	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return nil
}

func (s *snapshotRestoreSteps) CreateApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, genString string) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
	pvcArgs := &types.CreatePVCArgs{
		GenerateName: originalPVCGenerateName,
		StorageClass: args.StorageClass,
		Namespace:    args.Namespace,
	}
	pvc, err := s.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to create PVC")
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
		return nil, pvc, errors.Wrap(err, "Failed to create POD")
	}
	if err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, pvc, nil
}

func (s *snapshotRestoreSteps) ValidateData(ctx context.Context, pod *v1.Pod, data string) error {
	podData, err := s.dataValidatorOps.FetchPodData(pod.Name, pod.Namespace)
	if err != nil {
		return errors.Wrap(err, "Failed to fetch data from pod. Failure may be due to permissions issues. Try again with runAsUser=1000 option.")
	}
	if podData != data {
		return fmt.Errorf("string didn't match (%s , %s)", podData, data)
	}
	return nil
}

func (s *snapshotRestoreSteps) SnapshotApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim, snapshotName string) (*snapv1.VolumeSnapshot, error) {
	snapshotter, err := s.snapshotCreateOps.NewSnapshotter()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load snapshotter")
	}
	createSnapshotArgs := &types.CreateSnapshotArgs{
		Namespace:           args.Namespace,
		PVCName:             pvc.Name,
		VolumeSnapshotClass: args.VolumeSnapshotClass,
		SnapshotName:        snapshotName,
	}
	snapshot, err := s.snapshotCreateOps.CreateSnapshot(ctx, snapshotter, createSnapshotArgs)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Snapshot")
	}
	if !args.SkipCFSCheck {
		cfsArgs := &types.CreateFromSourceCheckArgs{
			VolumeSnapshotClass: args.VolumeSnapshotClass,
			SnapshotName:        snapshot.Name,
			Namespace:           args.Namespace,
		}
		if err = s.snapshotCreateOps.CreateFromSourceCheck(ctx, snapshotter, cfsArgs, s.SnapshotGroupVersion); err != nil {
			return snapshot, errors.Wrap(err, "Failed to create duplicate snapshot from source. To skip check use '--skipcfs=true' option.")
		}
	}
	return snapshot, nil
}

func (s *snapshotRestoreSteps) RestoreApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, snapshot *snapv1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
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
		return nil, nil, errors.Wrap(err, "Failed to restore PVC")
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
		return nil, pvc, errors.Wrap(err, "Failed to create restored Pod")
	}
	if err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, pvc, nil
}

func (s *snapshotRestoreSteps) Cleanup(ctx context.Context, results *types.CSISnapshotRestoreResults) {
	if results == nil {
		return
	}
	if results.OriginalPVC != nil {
		err := s.cleanerOps.DeletePVC(ctx, results.OriginalPVC.Name, results.OriginalPVC.Namespace)
		if err != nil {
			fmt.Printf("Error deleteing PVC (%s) - (%v)\n", results.OriginalPVC.Name, err)
		}
	}
	if results.OriginalPod != nil {
		err := s.cleanerOps.DeletePod(ctx, results.OriginalPod.Name, results.OriginalPod.Namespace)
		if err != nil {
			fmt.Printf("Error deleteing Pod (%s) - (%v)\n", results.OriginalPod.Name, err)
		}
	}
	if results.ClonedPVC != nil {
		err := s.cleanerOps.DeletePVC(ctx, results.ClonedPVC.Name, results.ClonedPVC.Namespace)
		if err != nil {
			fmt.Printf("Error deleteing PVC (%s) - (%v)\n", results.ClonedPVC.Name, err)
		}
	}
	if results.ClonedPod != nil {
		err := s.cleanerOps.DeletePod(ctx, results.ClonedPod.Name, results.ClonedPod.Namespace)
		if err != nil {
			fmt.Printf("Error deleteing Pod (%s) - (%v)\n", results.ClonedPod.Name, err)
		}
	}
	if results.Snapshot != nil {
		err := s.cleanerOps.DeleteSnapshot(ctx, results.Snapshot.Name, results.Snapshot.Namespace, s.SnapshotGroupVersion)
		if err != nil {
			fmt.Printf("Error deleteing Snapshot (%s) - (%v)\n", results.Snapshot.Name, err)
		}
	}
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

func (o *validateOperations) ValidateNamespace(ctx context.Context, namespace string) error {
	if o.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	_, err := o.kubeCli.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err
}

func (o *validateOperations) ValidateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	if o.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	return o.kubeCli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
}

func (o *validateOperations) ValidateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error) {
	if o.dynCli == nil {
		return nil, fmt.Errorf("dynCli not initialized")
	}
	VolSnapClassGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: groupVersion.Version, Resource: common.VolumeSnapshotClassResourcePlural}
	return o.dynCli.Resource(VolSnapClassGVR).Get(ctx, volumeSnapshotClass, metav1.GetOptions{})
}

//go:generate mockgen -destination=mocks/mock_application_creator.go -package=mocks . ApplicationCreator
type ApplicationCreator interface {
	CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error)
	CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error)
	WaitForPodReady(ctx context.Context, namespace string, podName string) error
}

type applicationCreate struct {
	kubeCli kubernetes.Interface
}

func (c *applicationCreate) CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
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

	pvcRes, err := c.kubeCli.CoreV1().PersistentVolumeClaims(args.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return pvc, err
	}

	return pvcRes, nil
}

func (c *applicationCreate) CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	if args.ContainerImage == "" {
		args.ContainerImage = common.DefaultPodImage
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

	podRes, err := c.kubeCli.CoreV1().Pods(args.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return pod, err
	}
	return podRes, nil
}

func (c *applicationCreate) WaitForPodReady(ctx context.Context, namespace string, podName string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	err := kankube.WaitForPodReady(ctx, c.kubeCli, namespace, podName)
	return err
}

//go:generate mockgen -destination=mocks/mock_snapshot_creator.go -package=mocks . SnapshotCreator
type SnapshotCreator interface {
	NewSnapshotter() (kansnapshot.Snapshotter, error)
	CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*snapv1.VolumeSnapshot, error)
	CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error
}

type snapshotCreate struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (c *snapshotCreate) NewSnapshotter() (kansnapshot.Snapshotter, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	if c.dynCli == nil {
		return nil, fmt.Errorf("dynCli not initialized")
	}
	return kansnapshot.NewSnapshotter(c.kubeCli, c.dynCli)
}

func (c *snapshotCreate) CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*snapv1.VolumeSnapshot, error) {
	if snapshotter == nil || args == nil {
		return nil, fmt.Errorf("snapshotter or args are empty")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	err := snapshotter.Create(ctx, args.SnapshotName, args.Namespace, args.PVCName, &args.VolumeSnapshotClass, true, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "CSI Driver failed to create snapshot for PVC (%s) in Namespace (%s)", args.PVCName, args.Namespace)
	}
	snap, err := snapshotter.Get(ctx, args.SnapshotName, args.Namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get CSI snapshot (%s) in Namespace (%s)", args.SnapshotName, args.Namespace)
	}
	return snap, nil
}

func (c *snapshotCreate) CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error {
	if c.dynCli == nil {
		return fmt.Errorf("dynCli not initialized")
	}
	if SnapshotGroupVersion == nil || SnapshotGroupVersion.Version == "" {
		return fmt.Errorf("snapshot group version not provided")
	}
	if snapshotter == nil || args == nil {
		return fmt.Errorf("snapshotter or args are nil")
	}
	if err := args.Validate(); err != nil {
		return err
	}
	targetSnapClassName := clonePrefix + args.VolumeSnapshotClass
	err := snapshotter.CloneVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, targetSnapClassName, kansnapshot.DeletionPolicyRetain, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to create a VolumeSnapshotClass to use to restore the snapshot")
	}
	defer func() {
		VolSnapClassGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: SnapshotGroupVersion.Version, Resource: common.VolumeSnapshotClassResourcePlural}
		err := c.dynCli.Resource(VolSnapClassGVR).Delete(ctx, targetSnapClassName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Delete VSC Error (%s) - (%v)\n", targetSnapClassName, err)
		}
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

//go:generate mockgen -destination=mocks/mock_cleaner.go -package=mocks . Cleaner
type Cleaner interface {
	DeletePVC(ctx context.Context, pvcName string, namespace string) error
	DeletePod(ctx context.Context, podName string, namespace string) error
	DeleteSnapshot(ctx context.Context, snapshotName string, namespace string, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error
}

type cleanse struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (c *cleanse) DeletePVC(ctx context.Context, pvcName string, namespace string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	return c.kubeCli.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
}

func (c *cleanse) DeletePod(ctx context.Context, podName string, namespace string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	return c.kubeCli.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (c *cleanse) DeleteSnapshot(ctx context.Context, snapshotName string, namespace string, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error {
	if c.dynCli == nil {
		return fmt.Errorf("dynCli not initialized")
	}
	if SnapshotGroupVersion == nil || SnapshotGroupVersion.Version == "" {
		return fmt.Errorf("snapshot group version not provided")
	}
	VolSnapGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: SnapshotGroupVersion.Version, Resource: common.VolumeSnapshotResourcePlural}
	return c.dynCli.Resource(VolSnapGVR).Namespace(namespace).Delete(ctx, snapshotName, metav1.DeleteOptions{})
}

//go:generate mockgen -destination=mocks/mock_api_version_fetcher.go -package=mocks . ApiVersionFetcher
type ApiVersionFetcher interface {
	GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error)
}

type apiVersionFetch struct {
	kubeCli kubernetes.Interface
}

func (p *apiVersionFetch) GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error) {
	if p.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	groups, _, err := p.kubeCli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Name == common.SnapGroupName {
			return &group.PreferredVersion, nil
		}
	}
	return nil, fmt.Errorf("Snapshot API group not found")
}

//go:generate mockgen -destination=mocks/mock_data_validator.go -package=mocks . DataValidator
type DataValidator interface {
	FetchPodData(podName string, podNamespace string) (string, error)
}

type validateData struct {
	kubeCli kubernetes.Interface
}

func (p *validateData) FetchPodData(podName string, podNamespace string) (string, error) {
	if p.kubeCli == nil {
		return "", fmt.Errorf("kubeCli not initialized")
	}
	stdout, _, err := kankube.Exec(p.kubeCli, podNamespace, podName, "", []string{"sh", "-c", "cat /data/out.txt"}, nil)
	return stdout, err
}

func getDriverNameFromUVSC(vsc unstructured.Unstructured, version string) string {
	var driverName interface{}
	var ok bool
	switch version {
	case common.SnapshotAlphaVersion:
		driverName, ok = vsc.Object[common.VolSnapClassAlphaDriverKey]
		if !ok {
			return ""
		}
	case common.SnapshotBetaVersion:
		driverName, ok = vsc.Object[common.VolSnapClassBetaDriverKey]
		if !ok {
			return ""
		}
	case common.SnapshotStableVersion:
		driverName, ok = vsc.Object[common.VolSnapClassStableDriverKey]
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
