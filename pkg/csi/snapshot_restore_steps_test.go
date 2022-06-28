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

func (s *CSITestSuite) TestValidateArgs(c *C) {
	ctx := context.Background()
	type fields struct {
		validateOps *mocks.MockArgumentValidator
		versionOps  *mocks.MockApiVersionFetcher
	}
	for _, tc := range []struct {
		args       *types.CSISnapshotRestoreArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{ // valid args
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
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
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
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
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
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
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: common.SnapshotAlphaVersion,
					}).Return(nil, fmt.Errorf("vsc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // groupversion error
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), "sc").Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						nil, fmt.Errorf("groupversion error")),
				)
			},
			errChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), "sc").Return(
						nil, fmt.Errorf("sc error")),
				)
			},
			errChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(fmt.Errorf("ns error")),
				)
			},
			errChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		}, {
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		}, {
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "",
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
		stepper := &snapshotRestoreSteps{
			validateOps:     f.validateOps,
			versionFetchOps: f.versionOps,
		}
		err := stepper.ValidateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestCreateApplication(c *C) {
	ctx := context.Background()
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args       *types.CSISnapshotRestoreArgs
		genString  string
		prepare    func(f *fields)
		errChecker Checker
		podChecker Checker
		pvcChecker Checker
	}{
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &types.CreatePVCArgs{
						GenerateName: originalPVCGenerateName,
						StorageClass: "sc",
						Namespace:    "ns",
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   originalPodGenerateName,
						PVCName:        "pvc1",
						Namespace:      "ns",
						Command:        []string{"/bin/sh"},
						ContainerArgs:  []string{"-c", "echo 'some string' >> /data/out.txt; sync; tail -f /dev/null"},
						RunAsUser:      100,
						ContainerImage: "image",
						MountPath:      "/data",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPVCReady(gomock.Any(), "ns", "pvc1").Return(nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(nil),
				)
			},
			errChecker: IsNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &types.CreatePVCArgs{
						GenerateName: originalPVCGenerateName,
						StorageClass: "sc",
						Namespace:    "ns",
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   originalPodGenerateName,
						PVCName:        "pvc1",
						Namespace:      "ns",
						Command:        []string{"/bin/sh"},
						ContainerArgs:  []string{"-c", "echo 'some string' >> /data/out.txt; sync; tail -f /dev/null"},
						RunAsUser:      100,
						ContainerImage: "image",
						MountPath:      "/data",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPVCReady(gomock.Any(), "ns", "pvc1").Return(nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(fmt.Errorf("pod ready error")),
				)
			},
			errChecker: NotNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("create pod error")),
				)
			},
			errChecker: NotNil,
			podChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("create pvc error")),
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
		stepper := &snapshotRestoreSteps{
			createAppOps: f.createAppOps,
		}
		pod, pvc, err := stepper.CreateApplication(ctx, tc.args, tc.genString)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}

func (s *CSITestSuite) TestSnapshotApplication(c *C) {
	ctx := context.Background()
	snapshotter := &fakeSnapshotter{name: "snapshotter"}
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		snapshotOps *mocks.MockSnapshotCreator
	}
	for _, tc := range []struct {
		args         *types.CSISnapshotRestoreArgs
		pvc          *v1.PersistentVolumeClaim
		snapshotName string
		prepare      func(f *fields)
		errChecker   Checker
		snapChecker  Checker
	}{
		{
			args: &types.CSISnapshotRestoreArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc1",
				},
			},
			snapshotName: "snap1",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().CreateSnapshot(gomock.Any(), snapshotter, &types.CreateSnapshotArgs{
						Namespace:           "ns",
						PVCName:             "pvc1",
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "snap1",
					}).Return(&snapv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name: "createdName",
						},
					}, nil),
					f.snapshotOps.EXPECT().CreateFromSourceCheck(gomock.Any(), snapshotter, &types.CreateFromSourceCheckArgs{
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "createdName",
						Namespace:           "ns",
					}, groupversion).Return(nil),
				)
			},
			errChecker:  IsNil,
			snapChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
				SkipCFSCheck:        true,
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc1",
				},
			},
			snapshotName: "snap1",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().CreateSnapshot(gomock.Any(), snapshotter, &types.CreateSnapshotArgs{
						Namespace:           "ns",
						PVCName:             "pvc1",
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "snap1",
					}).Return(&snapv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name: "createdName",
						},
					}, nil),
				)
			},
			errChecker:  IsNil,
			snapChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc1",
				},
			},
			snapshotName: "snap1",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().CreateSnapshot(gomock.Any(), snapshotter, &types.CreateSnapshotArgs{
						Namespace:           "ns",
						PVCName:             "pvc1",
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "snap1",
					}).Return(&snapv1.VolumeSnapshot{
						ObjectMeta: metav1.ObjectMeta{
							Name: "createdName",
						},
					}, nil),
					f.snapshotOps.EXPECT().CreateFromSourceCheck(gomock.Any(), snapshotter, &types.CreateFromSourceCheckArgs{
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "createdName",
						Namespace:           "ns",
					}, groupversion).Return(fmt.Errorf("cfs error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc1",
				},
			},
			snapshotName: "snap1",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(snapshotter, nil),
					f.snapshotOps.EXPECT().CreateSnapshot(gomock.Any(), snapshotter, &types.CreateSnapshotArgs{
						Namespace:           "ns",
						PVCName:             "pvc1",
						VolumeSnapshotClass: "vsc",
						SnapshotName:        "snap1",
					}).Return(nil, fmt.Errorf("create snapshot error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: IsNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
			},
			pvc: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc1",
				},
			},
			snapshotName: "snap1",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.snapshotOps.EXPECT().NewSnapshotter().Return(nil, fmt.Errorf("snapshotter error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: IsNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			snapshotOps: mocks.NewMockSnapshotCreator(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotRestoreSteps{
			snapshotCreateOps:    f.snapshotOps,
			SnapshotGroupVersion: groupversion,
		}
		snapshot, err := stepper.SnapshotApplication(ctx, tc.args, tc.pvc, tc.snapshotName)
		c.Check(err, tc.errChecker)
		c.Check(snapshot, tc.snapChecker)
	}
}

func (s *CSITestSuite) TestRestoreApplication(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args       *types.CSISnapshotRestoreArgs
		snapshot   *snapv1.VolumeSnapshot
		prepare    func(f *fields)
		errChecker Checker
		podChecker Checker
		pvcChecker Checker
	}{
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "snap1",
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
							Name:     "snap1",
						},
						RestoreSize: &resourceQuantity,
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						PVCName:        "pvc1",
						Namespace:      "ns",
						Command:        []string{"/bin/sh"},
						ContainerArgs:  []string{"-c", "tail -f /dev/null"},
						MountPath:      "/data",
						RunAsUser:      100,
						ContainerImage: "image",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPVCReady(gomock.Any(), "ns", "pvc1").Return(nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(nil),
				)
			},
			errChecker: IsNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "snap1",
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
							Name:     "snap1",
						},
						RestoreSize: &resourceQuantity,
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						PVCName:        "pvc1",
						Namespace:      "ns",
						Command:        []string{"/bin/sh"},
						ContainerArgs:  []string{"-c", "tail -f /dev/null"},
						MountPath:      "/data",
						RunAsUser:      100,
						ContainerImage: "image",
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPVCReady(gomock.Any(), "ns", "pvc1").Return(nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(fmt.Errorf("pod ready error")),
				)
			},
			errChecker: NotNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "snap1",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("create pod error")),
				)
			},
			errChecker: NotNil,
			podChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "snap1",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("create pvc error")),
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
		stepper := &snapshotRestoreSteps{
			createAppOps: f.createAppOps,
		}
		pod, pvc, err := stepper.RestoreApplication(ctx, tc.args, tc.snapshot)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}

func (s *CSITestSuite) TestCleanup(c *C) {
	ctx := context.Background()
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		cleanerOps *mocks.MockCleaner
	}
	for _, tc := range []struct {
		results *types.CSISnapshotRestoreResults
		prepare func(f *fields)
	}{
		{
			results: nil,
		},
		{
			results: &types.CSISnapshotRestoreResults{
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
						Name:      "snapshot",
						Namespace: "ns",
					},
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc1", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod1", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc2", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod2", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeleteSnapshot(ctx, "snapshot", "ns", groupversion).Return(nil),
				)
			},
		},
		{
			results: &types.CSISnapshotRestoreResults{
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
						Name:      "snapshot",
						Namespace: "ns",
					},
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc1", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod1", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc2", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod2", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeleteSnapshot(ctx, "snapshot", "ns", groupversion).Return(fmt.Errorf("err")),
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
		stepper := &snapshotRestoreSteps{
			cleanerOps:           f.cleanerOps,
			SnapshotGroupVersion: groupversion,
		}
		stepper.Cleanup(ctx, tc.results)
	}
}

func (s *CSITestSuite) TestValidateData(c *C) {
	ctx := context.Background()
	type fields struct {
		validatorOps *mocks.MockDataValidator
	}
	for _, tc := range []struct {
		prepare    func(f *fields)
		pod        *v1.Pod
		data       string
		errChecker Checker
	}{
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			},
			data: "somedata",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validatorOps.EXPECT().FetchPodData("pod", "ns").Return("somedata", nil),
				)
			},
			errChecker: IsNil,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			},
			data: "somedata",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validatorOps.EXPECT().FetchPodData("pod", "ns").Return("someotherdata", nil),
				)
			},
			errChecker: NotNil,
		},
		{
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			},
			data: "somedata",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validatorOps.EXPECT().FetchPodData("pod", "ns").Return("", fmt.Errorf("error")),
				)
			},
			errChecker: NotNil,
		},
	} {
		ctrl := gomock.NewController(c)
		defer ctrl.Finish()
		f := fields{
			validatorOps: mocks.NewMockDataValidator(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &snapshotRestoreSteps{
			dataValidatorOps: f.validatorOps,
		}
		err := stepper.ValidateData(ctx, tc.pod, tc.data)
		c.Check(err, tc.errChecker)
	}
}
