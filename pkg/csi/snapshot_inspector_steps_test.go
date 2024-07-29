package csi

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (s *CSITestSuite) TestSnapshotBrowseValidateArgs(c *C) {
	ctx := context.Background()
	vscName := "vsc"
	pvcName := "pvc"
	type fields struct {
		validateOps *mocks.MockArgumentValidator
		versionOps  *mocks.MockApiVersionFetcher
	}
	for _, tc := range []struct {
		args       *types.SnapshotBrowseArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{ // valid args
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), "sc").Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						&metav1.GroupVersionForDiscovery{
							GroupVersion: common.SnapshotAlphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshot(gomock.Any(), "vs", "ns", gomock.Any()).Return(
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "vs",
								Namespace: "ns",
							},
							Spec: snapv1.VolumeSnapshotSpec{
								Source: snapv1.VolumeSnapshotSource{
									PersistentVolumeClaimName: &pvcName,
								},
								VolumeSnapshotClassName: &vscName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: common.SnapshotAlphaVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							common.VolSnapClassAlphaDriverKey: "p1",
						},
					}, nil),
				)
			},
			errChecker: IsNil,
		},
		{ // driver mismatch
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						&metav1.GroupVersionForDiscovery{
							GroupVersion: common.SnapshotAlphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshot(gomock.Any(), "vs", "ns", gomock.Any()).Return(
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "vs",
								Namespace: "ns",
							},
							Spec: snapv1.VolumeSnapshotSpec{
								Source: snapv1.VolumeSnapshotSource{
									PersistentVolumeClaimName: &pvcName,
								},
								VolumeSnapshotClassName: &vscName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: common.SnapshotAlphaVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							common.VolSnapClassAlphaDriverKey: "p2",
						},
					}, nil),
				)
			},
			errChecker: NotNil,
		},
		{ // vsc error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshot(gomock.Any(), "vs", "ns", gomock.Any()).Return(
						&snapv1.VolumeSnapshot{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "vs",
								Namespace: "ns",
							},
							Spec: snapv1.VolumeSnapshotSpec{
								Source: snapv1.VolumeSnapshotSource{
									PersistentVolumeClaimName: &pvcName,
								},
								VolumeSnapshotClassName: &vscName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("vsc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // get driver versionn error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, fmt.Errorf("driver version error")),
				)
			},
			errChecker: NotNil,
		},
		{ // sc error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("sc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate vs error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshot(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("validate vs error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "vs",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(fmt.Errorf("validate ns error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate vs error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "",
				StorageClassName: "sc",
				Namespace:        "ns",
			},
			errChecker: NotNil,
		},
		{ // validate vsc error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "dfd",
				StorageClassName: "",
				Namespace:        "ns",
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.SnapshotBrowseArgs{
				SnapshotName:     "dfd",
				StorageClassName: "ddd",
				Namespace:        "",
			},
			errChecker: NotNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			validateOps: mocks.NewMockArgumentValidator(ctrl),
			versionOps:  mocks.NewMockApiVersionFetcher(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotBrowserSteps{
			validateOps:     f.validateOps,
			versionFetchOps: f.versionOps,
		}
		_, err := stepper.ValidateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestSnapshotBrowseFetchVS(c *C) {
	ctx := context.Background()
	snapshotter := &fakeSnapshotter{name: "snapshotter"}
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		snapshotOps *mocks.MockSnapshotFetcher
	}
	for _, tc := range []struct {
		args        *types.SnapshotBrowseArgs
		prepare     func(f *fields)
		errChecker  Checker
		snapChecker Checker
	}{
		{
			args: &types.SnapshotBrowseArgs{
				Namespace:        "ns",
				StorageClassName: "sc",
				SnapshotName:     "vs",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().GetVolumeSnapshot(gomock.Any(), snapshotter, &types.FetchSnapshotArgs{
						Namespace:    "ns",
						SnapshotName: "vs",
					}).Return(&snapv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vs",
						},
					}, nil),
				)
			},
			errChecker:  IsNil,
			snapChecker: NotNil,
		},
		{
			args: &types.SnapshotBrowseArgs{
				Namespace:        "ns",
				StorageClassName: "sc",
				SnapshotName:     "vs",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().GetVolumeSnapshot(gomock.Any(), snapshotter, &types.FetchSnapshotArgs{
						Namespace:    "ns",
						SnapshotName: "vs",
					}).Return(nil, fmt.Errorf("error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: IsNil,
		},
		{
			args: &types.SnapshotBrowseArgs{
				Namespace:        "ns",
				StorageClassName: "sc",
				SnapshotName:     "vs",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(nil, fmt.Errorf("error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: IsNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			snapshotOps: mocks.NewMockSnapshotFetcher(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotBrowserSteps{
			snapshotFetchOps:     f.snapshotOps,
			SnapshotGroupVersion: groupversion,
		}
		snapshot, err := stepper.FetchVS(ctx, tc.args)
		c.Check(err, tc.errChecker)
		c.Check(snapshot, tc.snapChecker)
	}
}

func (s *CSITestSuite) TestCreateInspectorApplicationForSnapshot(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args       *types.SnapshotBrowseArgs
		snapshot   *snapv1.VolumeSnapshot
		sc         *sv1.StorageClass
		prepare    func(f *fields)
		errChecker Checker
		podChecker Checker
		pvcChecker Checker
	}{
		{
			args: &types.SnapshotBrowseArgs{
				Namespace: "ns",
				RunAsUser: 100,
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &types.CreatePVCArgs{
						GenerateName: clonedPVCGenerateName,
						StorageClass: "sc",
						Namespace:    "ns",
						DataSource: &v1.TypedLocalObjectReference{
							APIGroup: &snapshotAPIGroup,
							Kind:     "VolumeSnapshot",
							Name:     "vs",
						},
						RestoreSize: &resourceQuantity,
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						PVCName:        "pvc",
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth", "-r", "/data"},
						MountPath:      "/data",
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod").Return(nil),
				)
			},
			errChecker: IsNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.SnapshotBrowseArgs{
				Namespace: "ns",
				RunAsUser: 100,
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &types.CreatePVCArgs{
						GenerateName: clonedPVCGenerateName,
						StorageClass: "sc",
						Namespace:    "ns",
						DataSource: &v1.TypedLocalObjectReference{
							APIGroup: &snapshotAPIGroup,
							Kind:     "VolumeSnapshot",
							Name:     "vs",
						},
						RestoreSize: &resourceQuantity,
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						PVCName:        "pvc",
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth", "-r", "/data"},
						MountPath:      "/data",
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod").Return(fmt.Errorf("pod ready error")),
				)
			},
			errChecker: NotNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.SnapshotBrowseArgs{
				Namespace: "ns",
				RunAsUser: 100,
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("pod  error")),
				)
			},
			errChecker: NotNil,
			podChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.SnapshotBrowseArgs{
				Namespace: "ns",
				RunAsUser: 100,
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error")),
				)
			},
			errChecker: NotNil,
			podChecker: IsNil,
			pvcChecker: IsNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			createAppOps: mocks.NewMockApplicationCreator(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotBrowserSteps{
			createAppOps: f.createAppOps,
		}
		pod, pvc, err := stepper.CreateInspectorApplication(ctx, tc.args, tc.snapshot, tc.sc)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}

func (s *CSITestSuite) TestSnapshotBrowseCleanup(c *C) {
	ctx := context.Background()
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		cleanerOps *mocks.MockCleaner
	}
	for _, tc := range []struct {
		pvc     *v1.PersistentVolumeClaim
		pod     *v1.Pod
		prepare func(f *fields)
	}{
		{
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc",
					Namespace: "ns",
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(nil),
				)
			},
		},
		{
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc",
					Namespace: "ns",
				},
			},
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(fmt.Errorf("err")),
				)
			},
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			cleanerOps: mocks.NewMockCleaner(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotBrowserSteps{
			cleanerOps:           f.cleanerOps,
			SnapshotGroupVersion: groupversion,
		}
		stepper.Cleanup(ctx, tc.pvc, tc.pod)
	}
}
