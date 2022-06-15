package csi

import (
	"context"
	"fmt"
	"time"

	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	basicK8sObjectWait = 1 * time.Second // Sleeps to avoid burning resources on kubectl calls and to make testing more reliable

	PVCKind = "PersistentVolumeClaim"
	PodKind ="Pod"
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
			k8sObjectReadyTimeout: args.K8sObjectReadyTimeout,
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
		r.srSteps.Cleanup(results)
	}

	return results, err
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_snapshot_restore_stepper.go -package=mocks . SnapshotRestoreStepper
type SnapshotRestoreStepper interface {
	ValidateArgs(ctx context.Context, args *types.CSISnapshotRestoreArgs) error
	CreateApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, data string) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	ValidateData(ctx context.Context, pod *v1.Pod, data string) error
	SnapshotApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim, snapshotName string) (*snapv1.VolumeSnapshot, error)
	RestoreApplication(ctx context.Context, args *types.CSISnapshotRestoreArgs, snapshot *snapv1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	Cleanup(results *types.CSISnapshotRestoreResults)
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
		RunAsUser:      args.RunAsUser,
		ContainerImage: args.ContainerImage,
		Command:        []string{"/bin/sh"},
		ContainerArgs:  []string{"-c", fmt.Sprintf("echo '%s' >> /data/out.txt; sync; tail -f /dev/null", genString)},
		MountPath:      "/data",
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, errors.Wrap(err, "Failed to create POD")
	}

	if args.K8sObjectReadyTimeout == 0 {
		if err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
			return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
		}
		return pod, pvc, nil
	}

	if err = s.createAppOps.WaitForPVCReadyOrCheckEventIssues(ctx, args.Namespace, pvc.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "PVC failed to become ready")
	}

	if err = s.createAppOps.WaitForPodReadyOrCheckEventIssues(ctx, args.Namespace, pod.Name); err != nil {
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
		RunAsUser:      args.RunAsUser,
		ContainerImage: args.ContainerImage,
		Command:        []string{"/bin/sh"},
		ContainerArgs:  []string{"-c", "tail -f /dev/null"},
		MountPath:      "/data",
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, errors.Wrap(err, "Failed to create restored Pod")
	}

	if args.K8sObjectReadyTimeout == 0 {
		if err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
			return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
		}
		return pod, pvc, nil
	}

	if err = s.createAppOps.WaitForPVCReadyOrCheckEventIssues(ctx, args.Namespace, pvc.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "PVC failed to become ready")
	}

	if err = s.createAppOps.WaitForPodReadyOrCheckEventIssues(ctx, args.Namespace, pod.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, pvc, nil
}

func (s *snapshotRestoreSteps) Cleanup(results *types.CSISnapshotRestoreResults) {
	ctx := context.Background()
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
