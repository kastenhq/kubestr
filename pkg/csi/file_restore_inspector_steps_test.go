package csi

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/golang/mock/gomock"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (s *CSITestSuite) TestFileRestoreValidateArgs(c *C) {
	ctx := context.Background()
	scName := "sc"
	vscName := "vsc"
	pvcName := "pvc"
	type fields struct {
		validateOps *mocks.MockArgumentValidator
		versionOps  *mocks.MockApiVersionFetcher
	}
	for _, tc := range []struct {
		args       *types.FileRestoreArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{ // valid args
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
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
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), scName).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
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
		{ // valid args
			args: &types.FileRestoreArgs{
				FromPVCName: "restorePVC",
				ToPVCName:   "sourcePVC",
				Namespace:   "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(
						&metav1.GroupVersionForDiscovery{
							GroupVersion: common.SnapshotAlphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), "restorePVC", "ns").Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "restorePVC",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().ValidatePVC(gomock.Any(), "sourcePVC", "ns").Return(
						&v1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "sourcePVC",
								Namespace: "ns",
							},
							Spec: v1.PersistentVolumeClaimSpec{
								VolumeName:       "vol",
								StorageClassName: &scName,
							},
						}, nil,
					),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), scName).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), scName).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
						}, nil),
				)
			},
			errChecker: IsNil,
		},
		{ // driver mismatch
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
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
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(
						&sv1.StorageClass{
							Provisioner: "p1",
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
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
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
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("vsc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // get driver versionn error
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, fmt.Errorf("driver version error")),
				)
			},
			errChecker: NotNil,
		},
		{ // sc error
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
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
					f.validateOps.EXPECT().ValidateStorageClass(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("sc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate vs error
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
				Namespace:        "ns",
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.validateOps.EXPECT().ValidateNamespace(gomock.Any(), "ns").Return(nil),
					f.versionOps.EXPECT().GetCSISnapshotGroupVersion().Return(nil, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshot(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("validate vs error")),
				)
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.FileRestoreArgs{
				FromSnapshotName: "vs",
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
			args: &types.FileRestoreArgs{
				FromSnapshotName: "",
				Namespace:        "ns",
			},
			errChecker: NotNil,
		},
		{ // validate ns error
			args: &types.FileRestoreArgs{
				FromSnapshotName: "dfd",
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
		stepper := &fileRestoreSteps{
			validateOps:     f.validateOps,
			versionFetchOps: f.versionOps,
		}
		_, _, _, _, err := stepper.ValidateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestCreateInspectorApplicationForFileRestore(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args         *types.FileRestoreArgs
		fromSnapshot *snapv1.VolumeSnapshot
		fromPVC      *v1.PersistentVolumeClaim
		sc           *sv1.StorageClass
		prepare      func(f *fields)
		errChecker   Checker
		podChecker   Checker
		pvcChecker   Checker
	}{
		{
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			fromSnapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			fromPVC: &v1.PersistentVolumeClaim{},
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
							Name: "restorePVC",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth"},
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
						PVCMap: map[string]types.VolumePath{
							"restorePVC": {
								MountPath: "/srv/snapshot-data",
							},
							"sourcePVC": {
								MountPath: "/srv/source-data",
							},
						},
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
			args: &types.FileRestoreArgs{
				Namespace:   "ns",
				RunAsUser:   100,
				FromPVCName: "restorePVC",
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			fromSnapshot: &snapv1.VolumeSnapshot{},
			fromPVC: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "restorePVC",
				},
			},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth"},
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
						PVCMap: map[string]types.VolumePath{
							"restorePVC": {
								MountPath: "/srv/restore-pvc-data",
							},
							"sourcePVC": {
								MountPath: "/srv/source-data",
							},
						},
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
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			fromSnapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			fromPVC: &v1.PersistentVolumeClaim{},
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
							Name: "restorePVC",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &types.CreatePodArgs{
						GenerateName:   clonedPodGenerateName,
						Namespace:      "ns",
						ContainerArgs:  []string{"--noauth"},
						RunAsUser:      100,
						ContainerImage: "filebrowser/filebrowser:v2",
						PVCMap: map[string]types.VolumePath{
							"restorePVC": {
								MountPath: "/srv/snapshot-data",
							},
							"sourcePVC": {
								MountPath: "/srv/source-data",
							},
						},
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
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			fromSnapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			fromPVC: &v1.PersistentVolumeClaim{},
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), gomock.Any()).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "restorePVC",
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
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			sc: &sv1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "sc",
				},
			},
			fromSnapshot: &snapv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "vs",
				},
				Status: &snapv1.VolumeSnapshotStatus{
					RestoreSize: &resourceQuantity,
				},
			},
			fromPVC: &v1.PersistentVolumeClaim{},
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
		stepper := &fileRestoreSteps{
			createAppOps: f.createAppOps,
		}
		sourcePVC := v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sourcePVC",
				Namespace: tc.args.Namespace,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteOnce,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		pod, pvc, _, err := stepper.CreateInspectorApplication(ctx, tc.args, tc.fromSnapshot, tc.fromPVC, &sourcePVC, tc.sc)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}

func (s *CSITestSuite) TestFileRestoreCleanup(c *C) {
	ctx := context.Background()
	groupversion := &metav1.GroupVersionForDiscovery{
		GroupVersion: "gv",
		Version:      "v",
	}
	type fields struct {
		cleanerOps *mocks.MockCleaner
	}
	for _, tc := range []struct {
		args       *types.FileRestoreArgs
		restorePVC *v1.PersistentVolumeClaim
		pod        *v1.Pod
		prepare    func(f *fields)
	}{
		{
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			restorePVC: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "restorePVC",
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
					f.cleanerOps.EXPECT().DeletePVC(ctx, "restorePVC", "ns").Return(nil),
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(nil),
				)
			},
		},
		{
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "",
				FromPVCName:      "restorePVC",
				ToPVCName:        "sourcePVC",
			},
			restorePVC: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "restorePVC",
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
					f.cleanerOps.EXPECT().DeletePod(ctx, "pod", "ns").Return(nil),
				)
			},
		},
		{
			args: &types.FileRestoreArgs{
				Namespace:        "ns",
				RunAsUser:        100,
				FromSnapshotName: "vs",
			},
			restorePVC: &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "restorePVC",
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
					f.cleanerOps.EXPECT().DeletePVC(ctx, "restorePVC", "ns").Return(fmt.Errorf("err")),
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
		stepper := &fileRestoreSteps{
			cleanerOps:           f.cleanerOps,
			SnapshotGroupVersion: groupversion,
		}
		stepper.Cleanup(ctx, tc.args, tc.restorePVC, tc.pod)
	}
}
