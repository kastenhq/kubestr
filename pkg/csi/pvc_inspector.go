package csi

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type PVCBrowseRunner struct {
	KubeCli      kubernetes.Interface
	DynCli       dynamic.Interface
	browserSteps PVCBrowserStepper
	pvc          *v1.PersistentVolumeClaim
	pod          *v1.Pod
	snapshot     *snapv1.VolumeSnapshot
}

func (r *PVCBrowseRunner) RunPVCBrowse(ctx context.Context, args *types.PVCBrowseArgs) error {
	r.browserSteps = &pvcBrowserSteps{
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
		snapshotCreateOps: &snapshotCreate{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
		portForwardOps: &portforward{},
		cleanerOps: &cleanse{
			kubeCli: r.KubeCli,
			dynCli:  r.DynCli,
		},
	}
	if args.ShowTree {
		fmt.Println("Show Tree works for PVC!")
		return nil
	}
	return r.RunPVCBrowseHelper(ctx, args)
}

func (r *PVCBrowseRunner) RunPVCBrowseHelper(ctx context.Context, args *types.PVCBrowseArgs) error {
	defer func() {
		fmt.Println("Cleaning up resources")
		r.browserSteps.Cleanup(ctx, r.pvc, r.pod, r.snapshot)
	}()
	if r.KubeCli == nil || r.DynCli == nil {
		return fmt.Errorf("cli uninitialized")
	}
	sc, err := r.browserSteps.ValidateArgs(ctx, args)
	if err != nil {
		return errors.Wrap(err, "Failed to validate arguments.")
	}

	fmt.Println("Taking a snapshot")
	snapName := snapshotPrefix + time.Now().Format("20060102150405")
	r.snapshot, err = r.browserSteps.SnapshotPVC(ctx, args, snapName)
	if err != nil {
		return errors.Wrap(err, "Failed to snapshot PVC.")
	}

	fmt.Println("Creating the file browser application.")
	r.pod, r.pvc, err = r.browserSteps.CreateInspectorApplication(ctx, args, r.snapshot, sc)
	if err != nil {
		return errors.Wrap(err, "Failed to create inspector application.")
	}

	fmt.Println("Forwarding the port.")
	err = r.browserSteps.PortForwardAPod(ctx, r.pod, args.LocalPort)
	if err != nil {
		return errors.Wrap(err, "Failed to port forward Pod.")
	}

	return nil
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_pvc_browser_stepper.go -package=mocks . PVCBrowserStepper
type PVCBrowserStepper interface {
	ValidateArgs(ctx context.Context, args *types.PVCBrowseArgs) (*sv1.StorageClass, error)
	SnapshotPVC(ctx context.Context, args *types.PVCBrowseArgs, snapshotName string) (*snapv1.VolumeSnapshot, error)
	CreateInspectorApplication(ctx context.Context, args *types.PVCBrowseArgs, snapshot *snapv1.VolumeSnapshot, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	PortForwardAPod(ctx context.Context, pod *v1.Pod, localPort int) error
	Cleanup(ctx context.Context, pvc *v1.PersistentVolumeClaim, pod *v1.Pod, snapshot *snapv1.VolumeSnapshot)
}

type pvcBrowserSteps struct {
	validateOps          ArgumentValidator
	versionFetchOps      ApiVersionFetcher
	createAppOps         ApplicationCreator
	snapshotCreateOps    SnapshotCreator
	portForwardOps       PortForwarder
	cleanerOps           Cleaner
	SnapshotGroupVersion *metav1.GroupVersionForDiscovery
}

func (p *pvcBrowserSteps) ValidateArgs(ctx context.Context, args *types.PVCBrowseArgs) (*sv1.StorageClass, error) {
	if err := args.Validate(); err != nil {
		return nil, errors.Wrap(err, "Failed to validate input arguments")
	}
	if err := p.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return nil, errors.Wrap(err, "Failed to validate Namespace")
	}
	pvc, err := p.validateOps.ValidatePVC(ctx, args.PVCName, args.Namespace)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate PVC")
	}

	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		return nil, errors.Errorf("PVC (%s) not bound. namespace - (%s)", pvc.Name, pvc.Namespace)
	}
	pv, err := p.validateOps.FetchPV(ctx, pvName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch PV")
	}
	if pv.Spec.CSI == nil {
		return nil, errors.New("PVC is not using a CSI volume")
	}
	sc, err := p.validateOps.ValidateStorageClass(ctx, *pvc.Spec.StorageClassName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate SC")
	}
	groupVersion, err := p.versionFetchOps.GetCSISnapshotGroupVersion()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to fetch groupVersion")
	}
	p.SnapshotGroupVersion = groupVersion
	uVSC, err := p.validateOps.ValidateVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, groupVersion)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate VolumeSnapshotClass")
	}
	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return nil, fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return sc, nil
}

func (p *pvcBrowserSteps) SnapshotPVC(ctx context.Context, args *types.PVCBrowseArgs, snapshotName string) (*snapv1.VolumeSnapshot, error) {
	snapshotter, err := p.snapshotCreateOps.NewSnapshotter()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load snapshotter")
	}
	createSnapshotArgs := &types.CreateSnapshotArgs{
		Namespace:           args.Namespace,
		PVCName:             args.PVCName,
		VolumeSnapshotClass: args.VolumeSnapshotClass,
		SnapshotName:        snapshotName,
	}
	return p.snapshotCreateOps.CreateSnapshot(ctx, snapshotter, createSnapshotArgs)
}

func (p *pvcBrowserSteps) CreateInspectorApplication(ctx context.Context, args *types.PVCBrowseArgs, snapshot *snapv1.VolumeSnapshot, storageClass *sv1.StorageClass) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
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
	pvc, err := p.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to restore PVC")
	}
	podArgs := &types.CreatePodArgs{
		GenerateName:   clonedPodGenerateName,
		PVCName:        pvc.Name,
		Namespace:      args.Namespace,
		RunAsUser:      args.RunAsUser,
		ContainerImage: "filebrowser/filebrowser:v2",
		ContainerArgs:  []string{"--noauth", "-r", "/data"},
		MountPath:      "/data",
	}
	pod, err := p.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, errors.Wrap(err, "Failed to create restored Pod")
	}
	if err = p.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name); err != nil {
		return pod, pvc, errors.Wrap(err, "Pod failed to become ready")
	}
	return pod, pvc, nil
}

func (p *pvcBrowserSteps) PortForwardAPod(ctx context.Context, pod *v1.Pod, localPort int) error {
	var wg sync.WaitGroup
	wg.Add(1)
	stopChan, readyChan, errChan := make(chan struct{}, 1), make(chan struct{}, 1), make(chan string)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cfg, err := p.portForwardOps.FetchRestConfig()
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
		err = p.portForwardOps.PortForwardAPod(pfArgs)
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

func (p *pvcBrowserSteps) Cleanup(ctx context.Context, pvc *v1.PersistentVolumeClaim, pod *v1.Pod, snapshot *snapv1.VolumeSnapshot) {
	if pvc != nil {
		err := p.cleanerOps.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		if err != nil {
			fmt.Println("Failed to delete PVC", pvc)
		}
	}
	if pod != nil {
		err := p.cleanerOps.DeletePod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			fmt.Println("Failed to delete Pod", pod)
		}
	}
	if snapshot != nil {
		err := p.cleanerOps.DeleteSnapshot(ctx, snapshot.Name, snapshot.Namespace, p.SnapshotGroupVersion)
		if err != nil {
			fmt.Println("Failed to delete Snapshot", snapshot)
		}
	}
}

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
