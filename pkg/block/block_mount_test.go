package block

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang/mock/gomock"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBlockMountCheckerNew(t *testing.T) {
	kubeCli := fake.NewSimpleClientset()
	dynCli := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())

	invalidArgs := []struct {
		name string
		args BlockMountCheckerArgs
	}{
		{"args:empty", BlockMountCheckerArgs{}},
		{"args:KubeCli", BlockMountCheckerArgs{
			KubeCli: kubeCli,
		}},
		{"args:KubeCli-DynCli", BlockMountCheckerArgs{
			KubeCli: kubeCli,
			DynCli:  dynCli,
		}},
		{"args:KubeCli-DynCli-StorageClass", BlockMountCheckerArgs{
			KubeCli:      kubeCli,
			DynCli:       dynCli,
			StorageClass: "sc",
		}},
	}
	for _, tc := range invalidArgs {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			bmt, err := NewBlockMountChecker(tc.args)
			c.Assert(err, qt.IsNotNil)
			c.Assert(bmt, qt.IsNil)
		})
	}

	t.Run("success", func(t *testing.T) {
		c := qt.New(t)
		args := BlockMountCheckerArgs{
			KubeCli:      kubeCli,
			DynCli:       dynCli,
			StorageClass: "sc",
			Namespace:    "namespace",
		}
		bmt, err := NewBlockMountChecker(args)
		c.Assert(err, qt.IsNil)
		c.Assert(bmt, qt.IsNotNil)

		b, ok := bmt.(*blockMountChecker)
		c.Assert(ok, qt.IsTrue)

		c.Assert(b.args, qt.Equals, args)
		c.Assert(b.validator, qt.IsNotNil)
		c.Assert(b.appCreator, qt.IsNotNil)
		c.Assert(b.cleaner, qt.IsNotNil)
		c.Assert(b.podName, qt.Equals, fmt.Sprintf(blockMountCheckerPodNameFmt, args.StorageClass))
		c.Assert(b.pvcName, qt.Equals, fmt.Sprintf(blockMountCheckerPVCNameFmt, args.StorageClass))
		c.Assert(b.podCleanupTimeout, qt.Equals, blockModeCheckerPodCleanupTimeout)
		c.Assert(b.pvcCleanupTimeout, qt.Equals, blockModeCheckerPVCCleanupTimeout)
	})
}

func TestBlockMountCheckerPvcWaitForTermination(t *testing.T) {
	type prepareArgs struct {
		b             *blockMountChecker
		mockValidator *mocks.MockArgumentValidator
	}

	kubeCli := fake.NewSimpleClientset()
	dynCli := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())

	tcs := []struct {
		name       string
		pvcTimeout time.Duration
		prepare    func(*prepareArgs)
		expErr     error
	}{
		{
			name:       "success",
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, ""))
			},
		},
		{
			name:       "timeout",
			pvcTimeout: time.Microsecond, // pvc wait will timeout
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(&v1.PersistentVolumeClaim{}, nil).AnyTimes()
			},
			expErr: context.DeadlineExceeded,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			args := BlockMountCheckerArgs{
				KubeCli:      kubeCli,
				DynCli:       dynCli,
				StorageClass: "sc",
				Namespace:    "namespace",
			}
			bmt, err := NewBlockMountChecker(args)
			c.Assert(err, qt.IsNil)
			c.Assert(bmt, qt.IsNotNil)
			b, ok := bmt.(*blockMountChecker)
			c.Assert(ok, qt.IsTrue)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pa := &prepareArgs{
				b:             b,
				mockValidator: mocks.NewMockArgumentValidator(ctrl),
			}
			tc.prepare(pa)
			b.validator = pa.mockValidator

			err = b.pvcWaitForTermination(tc.pvcTimeout)

			if tc.expErr != nil {
				c.Assert(err, qt.ErrorIs, tc.expErr)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestBlockMountCheckerCleanup(t *testing.T) {
	type prepareArgs struct {
		b             *blockMountChecker
		mockCleaner   *mocks.MockCleaner
		mockValidator *mocks.MockArgumentValidator
	}

	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "")
	someError := errors.New("test error")
	scName := "sc"
	namespace := "namespace"
	runningPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(blockMountCheckerPodNameFmt, scName),
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{Name: "container-0"},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	tcs := []struct {
		name       string
		podTimeout time.Duration
		pvcTimeout time.Duration
		objs       []runtime.Object
		prepare    func(*prepareArgs)
	}{
		{
			name:       "nothing-found",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockCleaner.EXPECT().DeletePod(gomock.Any(), pa.b.podName, pa.b.args.Namespace).Return(errNotFound)
				pa.mockCleaner.EXPECT().DeletePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(errNotFound)
				pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(nil, errNotFound)
			},
		},
		{
			name:       "error-deleting-pod",
			podTimeout: time.Microsecond, // pod wait will timeout
			pvcTimeout: time.Hour,
			objs:       []runtime.Object{runningPod},
			prepare: func(pa *prepareArgs) {
				pa.mockCleaner.EXPECT().DeletePod(gomock.Any(), pa.b.podName, pa.b.args.Namespace).Return(someError)
				pa.mockCleaner.EXPECT().DeletePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(errNotFound)
				pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(nil, errNotFound)
			},
		},
		{
			name:       "error-deleting-pvc",
			podTimeout: time.Hour,
			pvcTimeout: time.Microsecond, // timeout
			prepare: func(pa *prepareArgs) {
				pa.mockCleaner.EXPECT().DeletePod(gomock.Any(), pa.b.podName, pa.b.args.Namespace).Return(errNotFound)
				pa.mockCleaner.EXPECT().DeletePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(someError)
				pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(nil, someError).AnyTimes()
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			kubeCli := fake.NewSimpleClientset(tc.objs...)
			dynCli := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
			args := BlockMountCheckerArgs{
				KubeCli:      kubeCli,
				DynCli:       dynCli,
				StorageClass: scName,
				Namespace:    namespace,
			}
			bmt, err := NewBlockMountChecker(args)
			c.Assert(err, qt.IsNil)
			c.Assert(bmt, qt.IsNotNil)
			b, ok := bmt.(*blockMountChecker)
			c.Assert(ok, qt.IsTrue)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pa := &prepareArgs{
				b:             b,
				mockCleaner:   mocks.NewMockCleaner(ctrl),
				mockValidator: mocks.NewMockArgumentValidator(ctrl),
			}
			tc.prepare(pa)
			b.validator = pa.mockValidator
			b.cleaner = pa.mockCleaner
			b.podCleanupTimeout = tc.podTimeout
			b.pvcCleanupTimeout = tc.pvcTimeout

			b.Cleanup()
		})
	}
}

func TestBlockMountCheckerMount(t *testing.T) {
	type prepareArgs struct {
		b              *blockMountChecker
		mockCleaner    *mocks.MockCleaner
		mockValidator  *mocks.MockArgumentValidator
		mockAppCreator *mocks.MockApplicationCreator
	}

	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "")
	someError := errors.New("test error")
	scName := "sc"
	scProvisioner := "provisioenr"
	sc := &sv1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		Provisioner: scProvisioner,
	}
	namespace := "namespace"
	cleanupCalls := func(pa *prepareArgs) {
		pa.mockCleaner.EXPECT().DeletePod(gomock.Any(), pa.b.podName, pa.b.args.Namespace).Return(errNotFound)
		pa.mockCleaner.EXPECT().DeletePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(errNotFound)
		pa.mockValidator.EXPECT().ValidatePVC(gomock.Any(), pa.b.pvcName, pa.b.args.Namespace).Return(nil, errNotFound)
	}
	createPVCArgs := func(b *blockMountChecker) *types.CreatePVCArgs {
		pvcSize := b.args.PVCSize
		if pvcSize == "" {
			pvcSize = blockModeCheckerPVCDefaultSize
		}
		restoreSize := resource.MustParse(pvcSize)
		blockMode := v1.PersistentVolumeBlock
		return &types.CreatePVCArgs{
			Name:         b.pvcName,
			Namespace:    b.args.Namespace,
			StorageClass: b.args.StorageClass,
			VolumeMode:   &blockMode,
			RestoreSize:  &restoreSize,
		}
	}
	createPVC := func(b *blockMountChecker) *v1.PersistentVolumeClaim {
		return &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.args.Namespace,
				Name:      b.pvcName,
			},
		}
	}
	createPodArgs := func(b *blockMountChecker) *types.CreatePodArgs {
		return &types.CreatePodArgs{
			Name:           b.podName,
			PVCName:        []string{b.pvcName},
			Namespace:      b.args.Namespace,
			RunAsUser:      b.args.RunAsUser,
			ContainerImage: b.args.ContainerImage,
			Command:        []string{"/bin/sh"},
			ContainerArgs:  []string{"-c", "tail -f /dev/null"},
			DevicePath:     []string{"/mnt/block"},
		}
	}
	createPod := func(b *blockMountChecker) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.args.Namespace,
				Name:      b.podName,
			},
		}
	}

	tcs := []struct {
		name       string
		podTimeout time.Duration
		pvcTimeout time.Duration
		noCleanup  bool
		objs       []runtime.Object
		prepare    func(*prepareArgs)
		result     *BlockMountCheckerResult
	}{
		{
			name:       "no-storage-class",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, pa.b.args.StorageClass))
			},
		},
		{
			name:       "invalid-pvc-size",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.b.args.PVCSize = "10Q"
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(sc, nil)
			},
		},
		{
			name:       "create-pvc-error",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(sc, nil)
				pa.mockAppCreator.EXPECT().CreatePVC(gomock.Any(), createPVCArgs(pa.b)).Return(nil, someError)
				cleanupCalls(pa)
			},
		},
		{
			name:       "create-pod-error",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(sc, nil)
				pa.mockAppCreator.EXPECT().CreatePVC(gomock.Any(), createPVCArgs(pa.b)).Return(createPVC(pa.b), nil)
				pa.mockAppCreator.EXPECT().CreatePod(gomock.Any(), createPodArgs(pa.b)).Return(nil, someError)
				cleanupCalls(pa)
			},
		},
		{
			name:       "wait-for-pod-error",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(sc, nil)
				pa.mockAppCreator.EXPECT().CreatePVC(gomock.Any(), createPVCArgs(pa.b)).Return(createPVC(pa.b), nil)
				pa.mockAppCreator.EXPECT().CreatePod(gomock.Any(), createPodArgs(pa.b)).Return(createPod(pa.b), nil)
				pa.mockAppCreator.EXPECT().WaitForPodReady(gomock.Any(), pa.b.args.Namespace, pa.b.podName).Return(someError)
				cleanupCalls(pa)
			},
		},
		{
			name:       "success-no-cleanup",
			podTimeout: time.Hour,
			pvcTimeout: time.Hour,
			noCleanup:  true,
			prepare: func(pa *prepareArgs) {
				pa.mockValidator.EXPECT().ValidateStorageClass(gomock.Any(), pa.b.args.StorageClass).Return(sc, nil)
				pa.b.args.PVCSize = blockModeCheckerPVCDefaultSize
				pa.mockAppCreator.EXPECT().CreatePVC(gomock.Any(), createPVCArgs(pa.b)).Return(createPVC(pa.b), nil)
				pa.mockAppCreator.EXPECT().CreatePod(gomock.Any(), createPodArgs(pa.b)).Return(createPod(pa.b), nil)
				pa.mockAppCreator.EXPECT().WaitForPodReady(gomock.Any(), pa.b.args.Namespace, pa.b.podName).Return(nil)
			},
			result: &BlockMountCheckerResult{
				StorageClass: sc,
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			kubeCli := fake.NewSimpleClientset(tc.objs...)
			dynCli := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
			args := BlockMountCheckerArgs{
				KubeCli:      kubeCli,
				DynCli:       dynCli,
				StorageClass: scName,
				Namespace:    namespace,
				Cleanup:      !tc.noCleanup,
			}
			bmt, err := NewBlockMountChecker(args)
			c.Assert(err, qt.IsNil)
			c.Assert(bmt, qt.IsNotNil)
			b, ok := bmt.(*blockMountChecker)
			c.Assert(ok, qt.IsTrue)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pa := &prepareArgs{
				b:              b,
				mockCleaner:    mocks.NewMockCleaner(ctrl),
				mockValidator:  mocks.NewMockArgumentValidator(ctrl),
				mockAppCreator: mocks.NewMockApplicationCreator(ctrl),
			}
			tc.prepare(pa)
			b.validator = pa.mockValidator
			b.cleaner = pa.mockCleaner
			b.appCreator = pa.mockAppCreator
			b.podCleanupTimeout = tc.podTimeout
			b.pvcCleanupTimeout = tc.pvcTimeout

			result, err := b.Mount(ctx)
			if tc.result != nil {
				c.Assert(result, qt.DeepEquals, tc.result)
				c.Assert(err, qt.IsNil)
			} else {
				c.Assert(result, qt.IsNil)
				c.Assert(err, qt.IsNotNil)
			}
		})
	}
}
