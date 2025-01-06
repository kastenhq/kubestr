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

func (s *CSITestSuite) TestPvcBrowseValidateArgs(c *C) {
	ctx := context.Background()
	scName := "sc"
	type fields struct {
		validateOps *mocks.MockArgumentValidator
		versionOps  *mocks.MockApiVersionFetcher
	}
	for _, tc := range []struct {
		args       *types.PVCBrowseArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{ // valid args
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), "pvc", "ns").Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									CSI: &v1.CSIPersistentVolumeSource{},
								},
							},
						},
						nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), "sc").Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						&metav1.GroupVersionForDiscovery{
							GroupVersion: common.SnapshotStableVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: common.SnapshotStableVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							common.VolSnapClassStableDriverKey: "p1",
						},
					}, nil),
				)
			},
			errChecker: IsNil,
		},
		{ // driver mismatch
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), "pvc", "ns").Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									CSI: &v1.CSIPersistentVolumeSource{},
								},
							},
						},
						nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						&metav1.GroupVersionForDiscovery{
							GroupVersion: common.SnapshotStableVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: common.SnapshotStableVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							common.VolSnapClassStableDriverKey: "p2",
						},
					}, nil),
				)
			},
			errChecker: NotNil,
		},
		{ // vsc error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									CSI: &v1.CSIPersistentVolumeSource{},
								},
							},
						},
						nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("vsc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // get driver versionn error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									CSI: &v1.CSIPersistentVolumeSource{},
								},
							},
						},
						nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, fmt.Errorf("driver version error")),
				)
			},
			errChecker: NotNil,
		},
		{ // sc error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									CSI: &v1.CSIPersistentVolumeSource{},
								},
							},
						},
						nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("sc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // non csi error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(
						&v1.PersistentVolume{
							ObjectMeta: metav1.ObjectMeta{
								Name: "vol",
							},
							Spec: v1.PersistentVolumeSpec{
								PersistentVolumeSource: v1.PersistentVolumeSource{
									GCEPersistentDisk: &v1.GCEPersistentDiskVolumeSource{},
								},
							},
						},
						nil,
					),
				)
			},
			errChecker: NotNil,
		},
		{ // fetch pv error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "pvc",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().FetchPV(gomock.Any(), "vol").Return(nil, fmt.Errorf("pv fail")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate pvc error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("validate pvc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.PVCBrowseArgs{
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(fmt.Errorf("validate ns error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate pvc error
			args: &types.PVCBrowseArgs{
				PVCName:             "",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{ // validate vsc error
			args: &types.PVCBrowseArgs{
				PVCName:             "dfd",
				VolumeSnapshotClass: "",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.PVCBrowseArgs{
				PVCName:             "dfd",
				VolumeSnapshotClass: "ddd",
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
		stepper := &pvcBrowserSteps{
			validateOps:     f.validateOps,
			versionFetchOps: f.versionOps,
		}
		_, err := stepper.ValidateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestPvcBrowseSnapshotPVC(c *C) {
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
		args         *types.PVCBrowseArgs
		snapshotName string
		prepare      func(f *fields)
		errChecker   Checker
		snapChecker  Checker
	}{
		{
			args: &types.PVCBrowseArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
				PVCName:             "pvc1",
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
			args: &types.PVCBrowseArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
				PVCName:             "pvc1",
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
					}).Return(nil, fmt.Errorf("error")),
				)
			},
			errChecker:  NotNil,
			snapChecker: IsNil,
		},
		{
			args: &types.PVCBrowseArgs{
				Namespace:           "ns",
				VolumeSnapshotClass: "vsc",
				PVCName:             "pvc1",
			},
			snapshotName: "snap1",
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
			snapshotOps: mocks.NewMockSnapshotCreator(ctrl),
		}
		if tc.prepare != nil {
			tc.prepare(&f)
		}
		stepper := &pvcBrowserSteps{
			snapshotCreateOps:    f.snapshotOps,
			SnapshotGroupVersion: groupversion,
		}
		snapshot, err := stepper.SnapshotPVC(ctx, tc.args, tc.snapshotName)
		c.Check(err, tc.errChecker)
		c.Check(snapshot, tc.snapChecker)
	}
}

func (s *CSITestSuite) TestCreateInspectorApplicationForPVC(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args       *types.PVCBrowseArgs
		snapshot   *snapv1.VolumeSnapshot
		sc         *sv1.StorageClass
		prepare    func(f *fields)
		errChecker Checker
		podChecker Checker
		pvcChecker Checker
	}{
		{
			args: &types.PVCBrowseArgs{
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
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth", "-r", "/pvc-data"},
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
						PVCMap: map[string]types.VolumePath{
							"pvc1": {
								MountPath: "/pvc-data",
							},
						},
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(nil),
				)
			},
			errChecker: IsNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.PVCBrowseArgs{
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
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth", "-r", "/pvc-data"},
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
						PVCMap: map[string]types.VolumePath{
							"pvc1": {
								MountPath: "/pvc-data",
							},
						},
					}).Return(&v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod1",
						},
					}, nil),
					f.createAppOps.EXPECT().WaitForPodReady(gomock.Any(), "ns", "pod1").Return(fmt.Errorf("pod ready error")),
				)
			},
			errChecker: NotNil,
			podChecker: NotNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.PVCBrowseArgs{
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
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("pod  error")),
				)
			},
			errChecker: NotNil,
			podChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &types.PVCBrowseArgs{
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
					Name: "snap1",
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
		stepper := &pvcBrowserSteps{
			createAppOps: f.createAppOps,
		}
		pod, pvc, err := stepper.CreateInspectorApplication(ctx, tc.args, tc.snapshot, tc.sc)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}

func (s *CSITestSuite) TestPVCBrowseCleanup(c *C) {
	ctx := context.Background()
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		cleanerOps *mocks.MockCleaner
	}
	for _, tc := range []struct {
		pvc      *v1.PersistentVolumeClaim
		pod      *v1.Pod
		snapshot *snapv1.VolumeSnapshot
		prepare  func(f *fields)
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
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "snap1",
					Namespace: "ns",
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeleteSnapshot(ctx, "snap1", "ns", groupversion).Return(nil),
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
			snapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "snap1",
					Namespace: "ns",
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.cleanerOps.EXPECT().DeletePVC(ctx, "pvc", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(fmt.Errorf("err")),
					f.cleanerOps.EXPECT().DeleteSnapshot(ctx, "snap1", "ns", groupversion).Return(fmt.Errorf("err")),
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
		stepper := &pvcBrowserSteps{
			cleanerOps:           f.cleanerOps,
			SnapshotGroupVersion: groupversion,
		}
		stepper.Cleanup(ctx, tc.pvc, tc.pod, tc.snapshot)
	}
}
