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

type SnapshotBrowseRunner struct {
	KubeCli      kubernetes.Interface
	DynCli       dynamic.Interface
	browserSteps SnapshotBrowserStepper
	pvc          *v1.PersistentVolumeClaim
	pod          *v1.Pod
	snapshot     *snapv1.VolumeSnapshot
}

func (r *SnapshotBrowseRunner) RunSnapshotBrowse(ctx context.Context, args *types.SnapshotBrowseArgs) error {
	r.browserSteps = &snapshotBrowserSteps{
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
		snapshotFetchOps: &snapshotFetch{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
		portForwardOps: &portforward{},
		kubeExecutor: &kubeExec{
			kubeCli: r.KubeCli,
		},
		cleanerOps: &cleanse{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
	}
	if args.ShowTree {
		fmt.Println("Show Tree works for VS!")
		return nil
	}
	return r.RunSnapshotBrowseHelper(ctx, args)
}

func (r *SnapshotBrowseRunner) RunSnapshotBrowseHelper(ctx context.Context, args *types.SnapshotBrowseArgs) error {
	defer func() {
		fmt.Println("Cleaning up resources.")
		r.browserSteps.Cleanup(ctx, r.pvc, r.pod)
	}()

	if r.KubeCli == nil || r.DynCli == nil {
		return fmt.Errorf("cli uninitialized")
	}

	fmt.Println("Fetching the snapshot.")
	vs, sc, err := r.browserSteps.ValidateArgs(ctx, args)
	if err != nil {
		return errors.Wrap(err, "Failed to validate arguments.")
	}
	r.snapshot = vs

	fmt.Println("Creating the browser pod.")
	r.pod, r.pvc, err = r.browserSteps.CreateInspectorApplication(ctx, args, r.snapshot, sc)
	if err != nil {
		return errors.Wrap(err, "Failed to create inspector application.")
	}

	if args.ShowTree {
		fmt.Println("Printing the tree structure from root directory.")
		stdout, err := r.browserSteps.ExecuteTreeCommand(ctx, args, r.pod)
		if err != nil {
			return errors.Wrap(err, "Failed to execute tree command in pod.")
		}
		fmt.Printf("\n%s\n\n", stdout)
		return nil
	}

	fmt.Println("Forwarding the port.")
	err = r.browserSteps.PortForwardAPod(ctx, r.pod, args.LocalPort)
	if err != nil {
		return errors.Wrap(err, "Failed to port forward Pod.")
	}

	return nil
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_snapshot_browser_stepper.go -package=mocks . SnapshotBrowserStepper
type SnapshotBrowserStepper interface {
	ValidateArgs(ctx context.Context, args *types.SnapshotBrowseArgs) (*snapv1.VolumeSnapshot, *sv1.StorageClass, error)
	CreateInspectorApplication(ctx context.Context, args *types.SnapshotBrowseArgs, snapshot *snapv1.VolumeSnapshot, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	ExecuteTreeCommand(ctx context.Context, args *types.SnapshotBrowseArgs, pod *v1.Pod) (string, error)
	PortForwardAPod(ctx context.Context, pod *v1.Pod, localPort int) error
	Cleanup(ctx context.Context, pvc *v1.PersistentVolumeClaim, pod *v1.Pod)
}

type snapshotBrowserSteps struct {
	validateOps          ArgumentValidator
	versionFetchOps      ApiVersionFetcher
	snapshotFetchOps     SnapshotFetcher
	createAppOps         ApplicationCreator
	portForwardOps       PortForwarder
	cleanerOps           Cleaner
	kubeExecutor         KubeExecutor
	SnapshotGroupVersion *metav1.GroupVersionForDiscovery
}

func (s *snapshotBrowserSteps) ValidateArgs(ctx context.Context, args *types.SnapshotBrowseArgs) (*snapv1.VolumeSnapshot, *sv1.StorageClass, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate input arguments")
	}
	if err := s.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate Namespace")
	}
	groupVersion, err := s.versionFetchOps.GetCSISnapshotGroupVersion()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to fetch groupVersion")
	}
	s.SnapshotGroupVersion = groupVersion
	snapshot, err := s.validateOps.ValidateVolumeSnapshot(ctx, args.SnapshotName, args.Namespace, groupVersion)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshot")
	}
	pvc, err := s.validateOps.ValidatePVC(ctx, *snapshot.Spec.Source.PersistentVolumeClaimName, args.Namespace)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate source PVC")
	}
	sc, err := s.validateOps.ValidateStorageClass(ctx, *pvc.Spec.StorageClassName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate SC")
	}
	uVSC, err := s.validateOps.ValidateVolumeSnapshotClass(ctx, *snapshot.Spec.VolumeSnapshotClassName, groupVersion)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to validate VolumeSnapshotClass")
	}
	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return nil, nil, fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return snapshot, sc, nil
}

func (s *snapshotBrowserSteps) CreateInspectorApplication(ctx context.Context, args *types.SnapshotBrowseArgs, snapshot *snapv1.VolumeSnapshot, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
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
	pvc, err := s.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to restore PVC")
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   clonedPodGenerateName,
		Namespace:      args.Namespace,
		RunAsUser:      args.RunAsUser,
		ContainerImage: "filebrowser/filebrowser:v2",
		ContainerArgs:  []string{"--noauth", "-r", "/snapshot-data"},
		PVCMap: map[string]types.VolumePath{
			pvc.Name: {
				MountPath: "/snapshot-data",
			},
		},
	}
	if args.ShowTree {
		podArgs = &types.CreatePodArgs{
			GenerateName:   clonedPodGenerateName,
			Namespace:      args.Namespace,
			RunAsUser:      args.RunAsUser,
			ContainerImage: "alpine:3.19",
			Command:        []string{"/bin/sh"},
			ContainerArgs:  []string{"-c", "while true; do sleep 3600; done"},
			PVCMap: map[string]types.VolumePath{
				pvc.Name: {
					MountPath: "/snapshot-data",
				},
			},
		}
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, errors.Wrap(err, "Failed to create browse Pod")
	}
	if err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, pvc, nil
}

func (s *snapshotBrowserSteps) ExecuteTreeCommand(ctx context.Context, args *types.SnapshotBrowseArgs, pod *v1.Pod) (string, error) {
	command := []string{"tree", "/snapshot-data"}
	stdout, err := s.kubeExecutor.Exec(ctx, args.Namespace, pod.Name, pod.Spec.Containers[0].Name, command)
	if err != nil {
		return "", errors.Wrapf(err, "Error running command:(%v)", command)
	}
	return stdout, nil
}

func (s *snapshotBrowserSteps) PortForwardAPod(ctx context.Context, pod *v1.Pod, localPort int) error {
	var wg sync.WaitGroup
	wg.Add(1)
	stopChan, readyChan, errChan := make(chan struct{}, 1), make(chan struct{}, 1), make(chan string)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cfg, err := s.portForwardOps.FetchRestConfig()
	if err != nil {
		return errors.New("Failed to fetch rest config")
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("Stopping port forward")
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
		err = s.portForwardOps.PortForwardAPod(pfArgs)
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

func (s *snapshotBrowserSteps) Cleanup(ctx context.Context, pvc *v1.PersistentVolumeClaim, pod *v1.Pod) {
	if pvc != nil {
		err := s.cleanerOps.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		if err != nil {
			fmt.Println("Failed to delete PVC", pvc)
		}
	}
	if pod != nil {
		err := s.cleanerOps.DeletePod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			fmt.Println("Failed to delete Pod", pod)
		}
	}
}
