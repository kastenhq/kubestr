package csi

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type FileRestoreRunner struct {
	KubeCli      kubernetes.Interface
	DynCli       dynamic.Interface
	restoreSteps FileRestoreStepper
	restorePVC   *v1.PersistentVolumeClaim
	pod          *v1.Pod
	snapshot     *snapv1.VolumeSnapshot
}

func (f *FileRestoreRunner) RunFileRestore(ctx context.Context, args *types.FileRestoreArgs) error {
	f.restoreSteps = &fileRestoreSteps{
		validateOps: &validateOperations{
			kubeCli: f.KubeCli,
			dynCli:  f.DynCli,
		},
		versionFetchOps: &apiVersionFetch{
			kubeCli: f.KubeCli,
		},
		createAppOps: &applicationCreate{
			kubeCli: f.KubeCli,
		},
		portForwardOps: &portforward{},
		kubeExecutor: &kubeExec{
			kubeCli: f.KubeCli,
		},
		cleanerOps: &cleanse{
			kubeCli: f.KubeCli,
			dynCli:  f.DynCli,
		},
	}
	return f.RunFileRestoreHelper(ctx, args)
}

func (f *FileRestoreRunner) RunFileRestoreHelper(ctx context.Context, args *types.FileRestoreArgs) error {
	defer func() {
		f.restoreSteps.Cleanup(ctx, args, f.restorePVC, f.pod)
	}()

	if f.KubeCli == nil || f.DynCli == nil {
		return fmt.Errorf("cli uninitialized")
	}

	fmt.Println("Fetching the snapshot or PVC.")
	vs, restorePVC, sourcePVC, sc, err := f.restoreSteps.ValidateArgs(ctx, args)
	if err != nil {
		return errors.Wrap(err, "Failed to validate arguments.")
	}
	f.snapshot = vs

	fmt.Println("Creating the browser pod & mounting the PVCs.")
	var restoreMountPath string
	f.pod, f.restorePVC, restoreMountPath, err = f.restoreSteps.CreateInspectorApplication(ctx, args, f.snapshot, restorePVC, sourcePVC, sc)
	if err != nil {
		return errors.Wrap(err, "Failed to create inspector application.")
	}

	if args.Path != "" {
		fmt.Printf("Restoring the file %s\n", args.Path)
		_, err := f.restoreSteps.ExecuteCopyCommand(ctx, args, f.pod, restoreMountPath)
		if err != nil {
			return errors.Wrap(err, "Failed to execute cp command in pod.")
		}
		if args.FromSnapshotName != "" {
			fmt.Printf("File restored from VolumeSnapshot %s to Source PVC %s.\n", f.snapshot.Name, sourcePVC.Name)
		} else {
			fmt.Printf("File restored from PVC %s to Source PVC %s.\n", f.restorePVC.Name, sourcePVC.Name)
		}
		return nil
	}

	fmt.Println("Forwarding the port.")
	err = f.restoreSteps.PortForwardAPod(f.pod, args.LocalPort)
	if err != nil {
		return errors.Wrap(err, "Failed to port forward Pod.")
	}

	return nil
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_file_restore_stepper.go -package=mocks . FileRestoreStepper
type FileRestoreStepper interface {
	ValidateArgs(ctx context.Context, args *types.FileRestoreArgs) (*snapv1.VolumeSnapshot, *v1.PersistentVolumeClaim, *v1.PersistentVolumeClaim, *sv1.StorageClass, error)
	CreateInspectorApplication(ctx context.Context, args *types.FileRestoreArgs, snapshot *snapv1.VolumeSnapshot, restorePVC *v1.PersistentVolumeClaim, sourcePVC *v1.PersistentVolumeClaim, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, string, error)
	ExecuteCopyCommand(ctx context.Context, args *types.FileRestoreArgs, pod *v1.Pod, restoreMountPath string) (string, error)
	PortForwardAPod(pod *v1.Pod, localPort int) error
	Cleanup(ctx context.Context, args *types.FileRestoreArgs, restorePVC *v1.PersistentVolumeClaim, pod *v1.Pod)
}

type fileRestoreSteps struct {
	validateOps          ArgumentValidator
	versionFetchOps      ApiVersionFetcher
	createAppOps         ApplicationCreator
	portForwardOps       PortForwarder
	cleanerOps           Cleaner
	kubeExecutor         KubeExecutor
	SnapshotGroupVersion *metav1.GroupVersionForDiscovery
}

func (f *fileRestoreSteps) ValidateArgs(ctx context.Context, args *types.FileRestoreArgs) (*snapv1.VolumeSnapshot, *v1.PersistentVolumeClaim, *v1.PersistentVolumeClaim, *sv1.StorageClass, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate input arguments")
	}
	if err := f.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate Namespace")
	}
	groupVersion, err := f.versionFetchOps.GetCSISnapshotGroupVersion()
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "Failed to fetch groupVersion")
	}
	f.SnapshotGroupVersion = groupVersion
	var snapshot *snapv1.VolumeSnapshot
	var restorePVC, sourcePVC *v1.PersistentVolumeClaim
	var sc *sv1.StorageClass
	if args.FromSnapshotName != "" {
		fmt.Println("Fetching the snapshot.")
		snapshot, err := f.validateOps.ValidateVolumeSnapshot(ctx, args.FromSnapshotName, args.Namespace, groupVersion)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshot")
		}
		if args.ToPVCName == "" {
			fmt.Println("Fetching the source PVC from snapshot.")
			if *snapshot.Spec.Source.PersistentVolumeClaimName == "" {
				return nil, nil, nil, nil, errors.Wrap(err, "Failed to fetch source PVC. VolumeSnapshot does not have a PVC as it's source")
			}
			sourcePVC, err = f.validateOps.ValidatePVC(ctx, *snapshot.Spec.Source.PersistentVolumeClaimName, args.Namespace)
			if err != nil {
				return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate source PVC")
			}
		} else {
			fmt.Println("Fetching the source PVC.")
			sourcePVC, err = f.validateOps.ValidatePVC(ctx, args.ToPVCName, args.Namespace)
			if err != nil {
				return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate source PVC")
			}
		}
		sc, err = f.validateOps.ValidateStorageClass(ctx, *sourcePVC.Spec.StorageClassName)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate StorageClass for source PVC")
		}
		uVSC, err := f.validateOps.ValidateVolumeSnapshotClass(ctx, *snapshot.Spec.VolumeSnapshotClassName, groupVersion)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshotClass")
		}
		vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
		if sc.Provisioner != vscDriver {
			return nil, nil, nil, nil, fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
		}
	} else {
		fmt.Println("Fetching the restore PVC.")
		restorePVC, err = f.validateOps.ValidatePVC(ctx, args.FromPVCName, args.Namespace)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate restore PVC")
		}
		fmt.Println("Fetching the source PVC.")
		sourcePVC, err = f.validateOps.ValidatePVC(ctx, args.ToPVCName, args.Namespace)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate source PVC")
		}
		_, err = f.validateOps.ValidateStorageClass(ctx, *restorePVC.Spec.StorageClassName)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate StorageClass for restore PVC")
		}
		sc, err = f.validateOps.ValidateStorageClass(ctx, *sourcePVC.Spec.StorageClassName)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "Failed to validate StorageClass for source PVC")
		}
	}
	for _, sourceAccessMode := range sourcePVC.Spec.AccessModes {
		if sourceAccessMode == v1.ReadWriteOncePod {
			return nil, nil, nil, nil, fmt.Errorf("Unsupported %s AccessMode found in source PVC. Supported AccessModes are ReadOnlyMany & ReadWriteMany", sourceAccessMode)
		}
	}

	return snapshot, restorePVC, sourcePVC, sc, nil
}

func (f *fileRestoreSteps) CreateInspectorApplication(ctx context.Context, args *types.FileRestoreArgs, snapshot *snapv1.VolumeSnapshot, restorePVC *v1.PersistentVolumeClaim, sourcePVC *v1.PersistentVolumeClaim, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, string, error) {
	restoreMountPath := "/restore-pvc-data"
	if args.FromSnapshotName != "" {
		snapshotAPIGroup := "snapshot.storage.k8s.io"
		snapshotKind := "VolumeSnapshot"
		dataSource := &v1.TypedLocalObjectReference{
			APIGroup: &snapshotAPIGroup,
			Kind:     snapshotKind,
			Name:     snapshot.Name,
		}
		pvcArgs := &types.CreatePVCArgs{
			GenerateName: clonedPVCGenerateName,
			StorageClass: storageClass.Name,
			Namespace:    args.Namespace,
			DataSource:   dataSource,
			RestoreSize:  snapshot.Status.RestoreSize,
		}
		var err error
		restorePVC, err = f.createAppOps.CreatePVC(ctx, pvcArgs)
		if err != nil {
			return nil, nil, "", errors.Wrap(err, "Failed to restore PVC")
		}
		restoreMountPath = "/snapshot-data"
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   clonedPodGenerateName,
		Namespace:      args.Namespace,
		RunAsUser:      args.RunAsUser,
		ContainerImage: "filebrowser/filebrowser:v2",
		ContainerArgs:  []string{"--noauth"},
		PVCMap: map[string]types.VolumePath{
			restorePVC.Name: {
				MountPath: fmt.Sprintf("/srv%s", restoreMountPath),
			},
			sourcePVC.Name: {
				MountPath: "/srv/source-data",
			},
		},
	}
	if args.Path != "" {
		podArgs = &types.CreatePodArgs{
			GenerateName:   clonedPodGenerateName,
			Namespace:      args.Namespace,
			RunAsUser:      args.RunAsUser,
			ContainerImage: "alpine:3.19",
			Command:        []string{"/bin/sh"},
			ContainerArgs:  []string{"-c", "while true; do sleep 3600; done"},
			PVCMap: map[string]types.VolumePath{
				restorePVC.Name: {
					MountPath: restoreMountPath,
				},
				sourcePVC.Name: {
					MountPath: "/source-data",
				},
			},
		}
	}
	pod, err := f.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, restorePVC, "", errors.Wrap(err, "Failed to create browse Pod")
	}
	if err = f.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, restorePVC, "", errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, restorePVC, restoreMountPath, nil
}

func (f *fileRestoreSteps) ExecuteCopyCommand(ctx context.Context, args *types.FileRestoreArgs, pod *v1.Pod, restoreMountPath string) (string, error) {
	command := []string{"cp", "-rf", fmt.Sprintf("%s%s", restoreMountPath, args.Path), fmt.Sprintf("/source-data%s", args.Path)}
	stdout, err := f.kubeExecutor.Exec(ctx, args.Namespace, pod.Name, pod.Spec.Containers[0].Name, command)
	if err != nil {
		return "", errors.Wrapf(err, "Error running command:(%v)", command)
	}
	return stdout, nil
}

func (f *fileRestoreSteps) PortForwardAPod(pod *v1.Pod, localPort int) error {
	var wg sync.WaitGroup
	wg.Add(1)
	stopChan, readyChan, errChan := make(chan struct{}, 1), make(chan struct{}, 1), make(chan string)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cfg, err := f.portForwardOps.FetchRestConfig()
	if err != nil {
		return errors.New("Failed to fetch rest config")
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nStopping port forward.")
		close(stopChan)
		wg.Done()
	}()

	go func() {
		pfArgs := &types.PortForwardAPodRequest{
			RestConfig:   cfg,
			Pod:          pod,
			LocalPort:    localPort,
			PodPort:      80,
			OutStream:    bytes.Buffer(*out),
			ErrOutStream: bytes.Buffer(*errOut),
			StopCh:       stopChan,
			ReadyCh:      readyChan,
		}
		err = f.portForwardOps.PortForwardAPod(pfArgs)
		if err != nil {
			errChan <- fmt.Sprintf("Failed to port forward (%s)", err.Error())
		}
	}()

	select {
	case <-readyChan:
		url := fmt.Sprintf("http://localhost:%d/", localPort)
		fmt.Printf("Port forwarding is ready to get traffic. visit %s\n", url)
		openbrowser(url)
		wg.Wait()
	case msg := <-errChan:
		return errors.New(msg)
	}

	return nil
}

func (f *fileRestoreSteps) Cleanup(ctx context.Context, args *types.FileRestoreArgs, restorePVC *v1.PersistentVolumeClaim, pod *v1.Pod) {
	if args.FromSnapshotName != "" {
		fmt.Println("Cleaning up restore PVC.")
		if restorePVC != nil {
			err := f.cleanerOps.DeletePVC(ctx, restorePVC.Name, restorePVC.Namespace)
			if err != nil {
				fmt.Println("Failed to delete restore PVC", restorePVC)
			}
		}
	}
	fmt.Println("Cleaning up browser pod.")
	if pod != nil {
		err := f.cleanerOps.DeletePod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			fmt.Println("Failed to delete Pod", pod)
		}
	}
}
