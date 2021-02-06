package csi

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test(t *testing.T) { TestingT(t) }

type CSITestSuite struct{}

var _ = Suite(&CSITestSuite{})

func (s *CSITestSuite) TestRunSnapshotRestoreHelper(c *C) {
	ctx := context.Background()
	type fields struct {
		stepperOps *mocks.MockSnapshotRestoreStepper
	}
	for _, tc := range []struct {
		kubeCli    kubernetes.Interface
		dynCli     dynamic.Interface
		args       *types.CSISnapshotRestoreArgs
		prepare    func(f *fields)
		result     *types.CSISnapshotRestoreResults
		errChecker Checker
	}{
		{ // success
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: true,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(
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
						nil,
					),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), &v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod1",
							Namespace: "ns",
						},
					}, gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(),
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc1",
								Namespace: "ns",
							},
						}, gomock.Any(),
					).Return(
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name: "snapshot",
							},
						},
						nil,
					),
					f.stepperOps.EXPECT().RestoreApplication(gomock.Any(), gomock.Any(),
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name: "snapshot",
							},
						},
					).Return(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod2",
								Namespace: "ns",
							},
						},
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc2",
								Namespace: "ns",
							},
						},
						nil,
					),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), &v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod2",
							Namespace: "ns",
						},
					}, gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().Cleanup(gomock.Any(), gomock.Any()).Return(),
				)
			},
			result: &types.CSISnapshotRestoreResults{
				OriginalPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "ns",
					},
				},
				OriginalPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns",
					},
				},
				ClonedPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc2",
						Namespace: "ns",
					},
				},
				ClonedPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "ns",
					},
				},
				Snapshot: &snapv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name: "snapshot",
					},
				},
			},
			errChecker: IsNil,
		},
		{ // no cleanup
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
					f.stepperOps.EXPECT().RestoreApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: IsNil,
		},
		{ // restored data validation fails
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
					f.stepperOps.EXPECT().RestoreApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("validation error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // restore error, objects still returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
					f.stepperOps.EXPECT().RestoreApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pod2",
								Namespace: "ns",
							},
						},
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc2",
								Namespace: "ns",
							},
						},
						fmt.Errorf("restore error"),
					),
				)
			},
			result: &types.CSISnapshotRestoreResults{
				ClonedPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc2",
						Namespace: "ns",
					},
				},
				ClonedPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "ns",
					},
				},
			},
			errChecker: NotNil,
		},
		{ // restore error, no objects returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil),
					f.stepperOps.EXPECT().RestoreApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, fmt.Errorf("restore error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // snapshot error, object still returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name: "snapshot",
							},
						},
						fmt.Errorf("snapshot error"),
					),
				)
			},
			result: &types.CSISnapshotRestoreResults{
				Snapshot: &snapv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name: "snapshot",
					},
				},
			},
			errChecker: NotNil,
		},
		{ // snapshot error, object not returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().SnapshotApplication(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("snapshot error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // created data validation error
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil),
					f.stepperOps.EXPECT().ValidateData(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("validation error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // create error, objects still returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(
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
						fmt.Errorf("create error"),
					),
				)
			},
			result: &types.CSISnapshotRestoreResults{
				OriginalPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pvc1",
						Namespace: "ns",
					},
				},
				OriginalPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns",
					},
				},
			},
			errChecker: NotNil,
		},
		{ // create error, objects not returned
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(nil),
					f.stepperOps.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, fmt.Errorf("create error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // args validate error
			kubeCli: fake.NewSimpleClientset(),
			dynCli:  fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			args: &types.CSISnapshotRestoreArgs{
				Cleanup: false,
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.stepperOps.EXPECT().ValidateArgs(gomock.Any(), gomock.Any()).Return(fmt.Errorf("create error")),
				)
			},
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // empty cli
			kubeCli:    nil,
			dynCli:     fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
		{ // empty dyncli
			kubeCli:    fake.NewSimpleClientset(),
			dynCli:     nil,
			result:     &types.CSISnapshotRestoreResults{},
			errChecker: NotNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			stepperOps: mocks.NewMockSnapshotRestoreStepper(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		runner := &SnapshotRestoreRunner{
			KubeCli: tc.kubeCli,
			DynCli:  tc.dynCli,
			srSteps: f.stepperOps,
		}
		result, err := runner.RunSnapshotRestoreHelper(ctx, tc.args)
		c.Check(err, tc.errChecker)
		c.Assert(result, DeepEquals, tc.result)
	}
}

func (s *CSITestSuite) TestRunSnapshotRestoreRunner(c *C) {
	ctx := context.Background()
	r := &SnapshotRestoreRunner{}
	_, err := r.RunSnapshotRestore(ctx, nil)
	c.Check(err, NotNil)
}
