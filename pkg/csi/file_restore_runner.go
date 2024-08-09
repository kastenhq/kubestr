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
		fmt.Println("Cleaning up browser pod & restored PVC.")
		f.restoreSteps.Cleanup(ctx, f.restorePVC, f.pod)
	}()

	if f.KubeCli == nil || f.DynCli == nil {
		return fmt.Errorf("cli uninitialized")
	}

	fmt.Println("Fetching the snapshot.")
	vs, sourcePVC, sc, err := f.restoreSteps.ValidateArgs(ctx, args)
	if err != nil {
		return errors.Wrap(err, "Failed to validate arguments.")
	}
	f.snapshot = vs

	fmt.Println("Creating the restored PVC & browser Pod.")
	f.pod, f.restorePVC, err = f.restoreSteps.CreateInspectorApplication(ctx, args, f.snapshot, sourcePVC, sc)
	if err != nil {
		return errors.Wrap(err, "Failed to create inspector application.")
	}

	if args.Path != "" {
		fmt.Printf("Restoring the file %s\n", args.Path)
		_, err := f.restoreSteps.ExecuteCopyCommand(ctx, args, f.pod)
		if err != nil {
			return errors.Wrap(err, "Failed to execute cp command in pod.")
		}
		fmt.Printf("File restored from VolumeSnapshot %s to Source PVC %s.\n", f.snapshot.Name, sourcePVC.Name)
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
	ValidateArgs(ctx context.Context, args *types.FileRestoreArgs) (*snapv1.VolumeSnapshot, *v1.PersistentVolumeClaim, *sv1.StorageClass, error)
	CreateInspectorApplication(ctx context.Context, args *types.FileRestoreArgs, snapshot *snapv1.VolumeSnapshot, sourcePVC *v1.PersistentVolumeClaim, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	ExecuteCopyCommand(ctx context.Context, args *types.FileRestoreArgs, pod *v1.Pod) (string, error)
	PortForwardAPod(pod *v1.Pod, localPort int) error
	Cleanup(ctx context.Context, restorePVC *v1.PersistentVolumeClaim, pod *v1.Pod)
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

func (f *fileRestoreSteps) ValidateArgs(ctx context.Context, args *types.FileRestoreArgs) (*snapv1.VolumeSnapshot, *v1.PersistentVolumeClaim, *sv1.StorageClass, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate input arguments")
	}
	if err := f.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate Namespace")
	}
	groupVersion, err := f.versionFetchOps.GetCSISnapshotGroupVersion()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to fetch groupVersion")
	}
	f.SnapshotGroupVersion = groupVersion
	snapshot, err := f.validateOps.ValidateVolumeSnapshot(ctx, args.SnapshotName, args.Namespace, groupVersion)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshot")
	}
	sourcePVC, err := f.validateOps.ValidatePVC(ctx, *snapshot.Spec.Source.PersistentVolumeClaimName, args.Namespace)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate source PVC")
	}
	// Validate source PVC acceptable access modes
	sc, err := f.validateOps.ValidateStorageClass(ctx, *sourcePVC.Spec.StorageClassName)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate StorageClass")
	}
	uVSC, err := f.validateOps.ValidateVolumeSnapshotClass(ctx, *snapshot.Spec.VolumeSnapshotClassName, groupVersion)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshotClass")
	}
	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return nil, nil, nil, fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return snapshot, sourcePVC, sc, nil
}

func (f *fileRestoreSteps) CreateInspectorApplication(ctx context.Context, args *types.FileRestoreArgs, snapshot *snapv1.VolumeSnapshot, sourcePVC *v1.PersistentVolumeClaim, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
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
	restorePVC, err := f.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to restore PVC")
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   clonedPodGenerateName,
		Namespace:      args.Namespace,
		RunAsUser:      args.RunAsUser,
		ContainerImage: "filebrowser/filebrowser:v2",
		ContainerArgs:  []string{"--noauth"},
		PVCMap: map[string]types.VolumePath{
			restorePVC.Name: {
				MountPath: "/srv/snapshot-data",
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
					MountPath: "/snapshot-data",
				},
				sourcePVC.Name: {
					MountPath: "/source-data",
				},
			},
		}
	}
	pod, err := f.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, restorePVC, errors.Wrap(err, "Failed to create browse Pod")
	}
	if err = f.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, restorePVC, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, restorePVC, nil
}

func (f *fileRestoreSteps) ExecuteCopyCommand(ctx context.Context, args *types.FileRestoreArgs, pod *v1.Pod) (string, error) {
	command := []string{"cp", "-rf", fmt.Sprintf("/snapshot-data%s", args.Path), fmt.Sprintf("/source-data%s", args.Path)}
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

func (f *fileRestoreSteps) Cleanup(ctx context.Context, restorePVC *v1.PersistentVolumeClaim, pod *v1.Pod) {
	if restorePVC != nil {
		err := f.cleanerOps.DeletePVC(ctx, restorePVC.Name, restorePVC.Namespace)
		if err != nil {
			fmt.Println("Failed to delete restore PVC", restorePVC)
		}
	}
	if pod != nil {
		err := f.cleanerOps.DeletePod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			fmt.Println("Failed to delete Pod", pod)
		}
	}
}
