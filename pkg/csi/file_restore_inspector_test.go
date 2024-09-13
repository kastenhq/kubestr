package csi

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func (s *CSITestSuite) TestRunFileRestoreHelper(c *C) {
	ctx := context.Background()
	type fields struct {
		stepperOps *mocks.MockFileRestoreStepper
	}
	for _, tc := range []struct {
		kubeCli    kubernetes.Interface
		dynCli     dynamic.Interface
		args       *types.FileRestoreArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{
			// success
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(
						&snapv1.VolumeSnapshot{}, &v1.PersistentVolumeClaim{}, &v1.PersistentVolumeClaim{}, &sv1.StorageClass{}, nil,
					),
					f.stepperOps.EXPECT().CreateInspectorApplication(gomock.Any(), gomock.Any(),
						&snapv1.VolumeSnapshot{}, &v1.PersistentVolumeClaim{}, &v1.PersistentVolumeClaim{}, &sv1.StorageClass{},
					).Return(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "ns",
							},
						},
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc1",
								Namespace: "ns",
							},
						},
						"",
						nil,
					),
					f.stepperOps.EXPECT().PortForwardAPod(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "ns",
							},
						}, gomock.Any(),
					).Return(nil),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(),
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc1",
								Namespace: "ns",
							},
						},
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod1",
								Namespace: "ns",
							},
						},
					),
				)
			},
			errChecker: IsNil,
		},
		{
			// portforward failure
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil, nil),
					f.stepperOps.EXPECT().CreateInspectorApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, "", nil),
					f.stepperOps.EXPECT().PortForwardAPod(gomock.Any(), gomock.Any()).Return(fmt.Errorf("portforward error")),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
		{
			// createapp failure
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil, nil),
					f.stepperOps.EXPECT().CreateInspectorApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, "", fmt.Errorf("createapp error")),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
		{
			// fetch snapshot failure
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil, fmt.Errorf("snapshot error")),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
		{
			// validate failure
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil, fmt.Errorf("validate error")),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
		{
			// emptycli failure
			kubeCli: nil,
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
		{
			// emptydyncli failure
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  nil,
			args:    &types.FileRestoreArgs{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()),
				)
			},
			errChecker: NotNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			stepperOps: mocks.NewMockFileRestoreStepper(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		runner := &FileRestoreRunner{
			KubeCli:      tc.kubeCli,
			DynCli:       tc.dynCli,
			restoreSteps: f.stepperOps,
		}
		err := runner.RunFileRestoreHelper(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestFileRestoreRunner(c *C) {
	ctx := context.Background()
	r := &FileRestoreRunner{
		restoreSteps: &fileRestoreSteps{},
	}
	args := types.FileRestoreArgs{}
	err := r.RunFileRestoreHelper(ctx, &args)
	c.Check(err, NotNil)
}
