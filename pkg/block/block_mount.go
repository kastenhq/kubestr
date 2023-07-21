package block

import (
	"context"
	"fmt"
	"time"

	kankube "github.com/kanisterio/kanister/pkg/kube"
	"github.com/kanisterio/kanister/pkg/poll"
	"github.com/kastenhq/kubestr/pkg/csi"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type BlockMountCheckerArgs struct {
	KubeCli kubernetes.Interface
	DynCli  dynamic.Interface

	StorageClass          string
	Namespace             string
	Cleanup               bool
	RunAsUser             int64
	ContainerImage        string
	K8sObjectReadyTimeout time.Duration
}

func (a *BlockMountCheckerArgs) Validate() error {
	if a.KubeCli == nil || a.DynCli == nil || a.StorageClass == "" || a.Namespace == "" {
		return fmt.Errorf("Require fields are missing. (KubeCli, DynCli, StorageClass, Namespace)")
	}
	return nil

}

// BlockMountChecker tests if a storage class can provision volumes for block mounts.
type BlockMountChecker interface {
	Mount(ctx context.Context) (*BlockMountCheckerResult, error)
	Cleanup()
}

type BlockMountCheckerResult struct {
	StorageClass *sv1.StorageClass
}

const (
	blockMountTesterPVCNameFmt = "kubestr-blockmount-%s-pvc"
	blockMountTesterPodNameFmt = "kubestr-blockmount-%s-pod"

	blockModeTesterPodCleanupTimeout = time.Second * 120
	blockModeTesterPvcCleanupTimeout = time.Second * 120
)

// blockMountTester provides BlockMountTester
type blockMountTester struct {
	args              BlockMountCheckerArgs
	podName           string
	pvcName           string
	validator         csi.ArgumentValidator
	appCreator        csi.ApplicationCreator
	cleaner           csi.Cleaner
	podCleanupTimeout time.Duration
	pvcCleanupTimeout time.Duration
}

func NewBlockMountTester(args BlockMountCheckerArgs) (BlockMountChecker, error) {
	if err := args.Validate(); err != nil {
		return nil, err
	}

	b := &blockMountTester{}
	b.args = args
	b.podName = fmt.Sprintf(blockMountTesterPodNameFmt, b.args.StorageClass)
	b.pvcName = fmt.Sprintf(blockMountTesterPVCNameFmt, b.args.StorageClass)
	b.validator = csi.NewArgumentValidator(b.args.KubeCli, b.args.DynCli)
	b.appCreator = csi.NewApplicationCreator(b.args.KubeCli, args.K8sObjectReadyTimeout)
	b.cleaner = csi.NewCleaner(b.args.KubeCli, b.args.DynCli)
	b.podCleanupTimeout = blockModeTesterPodCleanupTimeout
	b.pvcCleanupTimeout = blockModeTesterPvcCleanupTimeout

	return b, nil
}

func (b *blockMountTester) Mount(ctx context.Context) (*BlockMountCheckerResult, error) {
	fmt.Printf("Fetching StorageClass %s ...\n", b.args.StorageClass)
	sc, err := b.validator.ValidateStorageClass(ctx, b.args.StorageClass)
	if err != nil {
		fmt.Printf(" -> Failed to fetch StorageClass(%s): (%v)\n", b.args.StorageClass, err)
		return nil, err
	}

	fmt.Printf(" -> Provisioner: %s\n", sc.Provisioner)

	if b.args.Cleanup {
		defer b.Cleanup()
	} else {
		defer func() {
			fmt.Println("Resources were not cleaned up")
		}()
	}

	fmt.Println("Provisioning a Volume for block mode access ...")
	blockMode := v1.PersistentVolumeBlock
	tB := time.Now()
	_, err = b.appCreator.CreatePVC(ctx, &types.CreatePVCArgs{
		Name:         b.pvcName,
		Namespace:    b.args.Namespace,
		StorageClass: b.args.StorageClass,
		VolumeMode:   &blockMode,
	})
	if err != nil {
		fmt.Printf(" -> Failed to provision a Volume (%v)\n", err)
		return nil, err
	}
	fmt.Printf(" -> Created PVC %s/%s (%s)\n", b.args.Namespace, b.pvcName, time.Since(tB).Truncate(time.Millisecond).String())

	fmt.Println("Creating a Pod with a volumeDevice ...")
	tB = time.Now()
	_, err = b.appCreator.CreatePod(ctx, &types.CreatePodArgs{
		Name:           b.podName,
		PVCName:        b.pvcName,
		Namespace:      b.args.Namespace,
		RunAsUser:      b.args.RunAsUser,
		ContainerImage: b.args.ContainerImage,
		Command:        []string{"/bin/sh"},
		ContainerArgs:  []string{"-c", "tail -f /dev/null"},
		DevicePath:     "/mnt/block",
	})
	if err != nil {
		fmt.Printf(" -> Failed to create Pod (%v)\n", err)
		return nil, err
	}
	fmt.Printf(" -> Created Pod %s/%s\n", b.args.Namespace, b.podName)

	fmt.Printf(" -> Waiting at most %s for Pod to become ready ...\n", b.args.K8sObjectReadyTimeout.String())
	if err = b.appCreator.WaitForPodReady(ctx, b.args.Namespace, b.podName); err != nil {
		fmt.Printf(" -> Pod timed out (%v)\n", err)
		return nil, err
	}
	fmt.Printf(" -> Pod ready (%s)\n", time.Since(tB).Truncate(time.Millisecond).String())

	return &BlockMountCheckerResult{
		StorageClass: sc,
	}, nil
}

func (b *blockMountTester) Cleanup() {
	var (
		ctx = context.Background()
		err error
	)

	// delete Pod
	fmt.Printf("Deleting Pod %s/%s ...\n", b.args.Namespace, b.podName)
	tB := time.Now()
	err = b.cleaner.DeletePod(ctx, b.podName, b.args.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		fmt.Printf("  Error deleting Pod %s/%s - (%v)\n", b.args.Namespace, b.podName, err)
	}

	// Give it a chance to run ...
	podWaitCtx, podWaitCancelFn := context.WithTimeout(context.Background(), b.podCleanupTimeout)
	defer podWaitCancelFn()
	err = kankube.WaitForPodCompletion(podWaitCtx, b.args.KubeCli, b.args.Namespace, b.podName)
	if err == nil || (err != nil && apierrors.IsNotFound(err)) {
		fmt.Printf(" -> Deleted pod (%s)\n", time.Since(tB).Truncate(time.Millisecond).String())
	} else {
		fmt.Printf(" -> Failed to delete Pod in %s\n", time.Since(tB).Truncate(time.Millisecond).String())
	}

	// delete PVC
	fmt.Printf("Deleting PVC %s/%s ...\n", b.args.Namespace, b.pvcName)
	tB = time.Now()
	err = b.cleaner.DeletePVC(ctx, b.pvcName, b.args.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		fmt.Printf("  Error deleting PVC %s/%s - (%v)\n", b.args.Namespace, b.pvcName, err)
	}

	err = b.pvcWaitForTermination(b.pvcCleanupTimeout)
	if err != nil {
		fmt.Printf(" -> PVC failed to delete in %s\n", time.Since(tB).Truncate(time.Millisecond).String())
	} else {
		fmt.Printf(" -> Deleted PVC (%s)\n", time.Since(tB).Truncate(time.Millisecond).String())
	}
}

func (b *blockMountTester) pvcWaitForTermination(timeout time.Duration) error {
	pvcWaitCtx, pvcWaitCancelFn := context.WithTimeout(context.Background(), timeout)
	defer pvcWaitCancelFn()

	return poll.Wait(pvcWaitCtx, func(ctx context.Context) (bool, error) {
		_, err := b.validator.ValidatePVC(ctx, b.pvcName, b.args.Namespace)
		if err != nil && apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, nil
	})
}
