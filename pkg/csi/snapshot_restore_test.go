package csi

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	"github.com/kastenhq/kubestr/pkg/csi/mocks"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func Test(t *testing.T) { TestingT(t) }

type CSITestSuite struct{}

var _ = Suite(&CSITestSuite{})

func (s *CSITestSuite) TestGetDriverNameFromUVSC(c *C) {

	for _, tc := range []struct {
		vsc     unstructured.Unstructured
		version string
		expOut  string
	}{
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					VolSnapClassAlphaDriverKey: "p2",
				},
			},
			version: alphaVersion,
			expOut:  "p2",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			version: alphaVersion,
			expOut:  "",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					VolSnapClassBetaDriverKey: "p2",
				},
			},
			version: betaVersion,
			expOut:  "p2",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			version: betaVersion,
			expOut:  "",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					VolSnapClassBetaDriverKey: map[string]string{},
				},
			},
			version: betaVersion,
			expOut:  "",
		},
	} {
		driverName := getDriverNameFromUVSC(tc.vsc, tc.version)
		c.Assert(driverName, Equals, tc.expOut)
	}

}

func (s *CSITestSuite) TestGetCSISnapshotGroupVersion(c *C) {
	for _, tc := range []struct {
		resources  []*metav1.APIResourceList
		errChecker Checker
		gvChecker  Checker
	}{
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "/////",
				},
			},
			errChecker: NotNil,
			gvChecker:  IsNil,
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "snapshot.storage.k8s.io/v1alpha1",
				},
			},
			errChecker: IsNil,
			gvChecker:  NotNil,
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "notrbac.authorization.k8s.io/v1",
				},
			},
			errChecker: NotNil,
			gvChecker:  IsNil,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).Resources = tc.resources
		p := &apiVersionFetch{cli: cli}
		gv, err := p.getCSISnapshotGroupVersion()
		c.Check(err, tc.errChecker)
		c.Check(gv, tc.gvChecker)
	}
}

func (s *CSITestSuite) TestValidateNamespace(c *C) {
	ctx := context.Background()
	ops := &validateOperations{
		kubeCli: fake.NewSimpleClientset(),
	}
	err := ops.validateNamespace(ctx, "ns")
	c.Check(err, NotNil)

	ops = &validateOperations{
		kubeCli: fake.NewSimpleClientset(&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		}),
	}
	err = ops.validateNamespace(ctx, "ns")
	c.Check(err, IsNil)
}

func (s *CSITestSuite) TestValidateStorageClass(c *C) {
	ctx := context.Background()
	ops := &validateOperations{
		kubeCli: fake.NewSimpleClientset(),
	}
	sc, err := ops.validateStorageClass(ctx, "sc")
	c.Check(err, NotNil)
	c.Check(sc, IsNil)

	ops = &validateOperations{
		kubeCli: fake.NewSimpleClientset(&sv1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sc",
			},
		}),
	}
	sc, err = ops.validateStorageClass(ctx, "sc")
	c.Check(err, IsNil)
	c.Check(sc, NotNil)
}

func (s *CSITestSuite) TestValidateVolumeSnapshotClass(c *C) {
	ctx := context.Background()
	ops := &validateOperations{
		dynCli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	}
	uVSC, err := ops.validateVolumeSnapshotClass(ctx, "vsc", &metav1.GroupVersionForDiscovery{GroupVersion: alphaVersion})
	c.Check(err, NotNil)
	c.Check(uVSC, IsNil)

	ops = &validateOperations{
		dynCli: fakedynamic.NewSimpleDynamicClient(
			runtime.NewScheme(),
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": fmt.Sprintf("%s/%s", v1alpha1.GroupName, v1alpha1.Version),
					"kind":       "VolumeSnapshotClass",
					"metadata": map[string]interface{}{
						"name": "vsc",
					},
					"snapshotter":    "somesnapshotter",
					"deletionPolicy": "Delete",
				},
			},
		),
	}
	uVSC, err = ops.validateVolumeSnapshotClass(ctx, "vsc", &metav1.GroupVersionForDiscovery{Version: v1alpha1.Version})
	c.Check(err, IsNil)
	c.Check(uVSC, NotNil)
}

func (s *CSITestSuite) TestCreatePVC(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	for _, tc := range []struct {
		args        interface{}
		failCreates bool
		errChecker  Checker
		pvcChecker  Checker
	}{
		{
			args: &CreatePVCArgs{
				genName:      "genName",
				storageClass: "sc",
				namespace:    "ns",
				dataSource: &v1.TypedLocalObjectReference{
					Name: "ds",
				},
				restoreSize: &resourceQuantity,
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &CreatePVCArgs{
				genName:      "genName",
				storageClass: "sc",
				namespace:    "ns",
				dataSource: &v1.TypedLocalObjectReference{
					Name: "ds",
				},
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &CreatePVCArgs{
				genName:      "genName",
				storageClass: "sc",
				namespace:    "ns",
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			args: &CreatePVCArgs{
				genName:      "genName",
				storageClass: "sc",
				namespace:    "ns",
			},
			failCreates: true,
			errChecker:  NotNil,
			pvcChecker:  NotNil,
		},
		{
			args:       "bad input args",
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
	} {
		creator := &applicationCreate{cli: fake.NewSimpleClientset()}
		if tc.failCreates {
			creator.cli.(*fake.Clientset).Fake.PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		pvc, err := creator.CreatePVC(ctx, tc.args)
		c.Check(pvc, tc.pvcChecker)
		c.Check(err, tc.errChecker)
		if pvc != nil && err == nil {
			args, ok := tc.args.(*CreatePVCArgs)
			c.Assert(ok, Equals, true)
			_, ok = pvc.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			c.Assert(pvc.GenerateName, Equals, args.genName)
			c.Assert(pvc.Namespace, Equals, args.namespace)
			c.Assert(pvc.Spec.AccessModes, DeepEquals, []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce})
			c.Assert(*pvc.Spec.StorageClassName, Equals, args.storageClass)
			c.Assert(pvc.Spec.DataSource, DeepEquals, args.dataSource)
			if args.restoreSize != nil {
				c.Assert(pvc.Spec.Resources, DeepEquals, v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: *args.restoreSize,
					},
				})
			} else {
				c.Assert(pvc.Spec.Resources, DeepEquals, v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				})
			}
		}
	}
}

func (s *CSITestSuite) TestCreatePod(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		args        interface{}
		failCreates bool
		errChecker  Checker
		podChecker  Checker
	}{
		{
			args: &CreatePodArgs{
				genName:        "name",
				pvcName:        "pvcname",
				namespace:      "ns",
				cmd:            "somecommand",
				runAsUser:      1000,
				containerImage: "containerimage",
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			args: &CreatePodArgs{
				genName:   "name",
				pvcName:   "pvcname",
				namespace: "ns",
				cmd:       "somecommand",
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			args: &CreatePodArgs{
				genName:   "name",
				pvcName:   "pvcname",
				namespace: "ns",
				cmd:       "somecommand",
			},
			failCreates: true,
			errChecker:  NotNil,
			podChecker:  NotNil,
		},
		{
			args:       "bad input args",
			errChecker: NotNil,
			podChecker: IsNil,
		},
	} {
		creator := &applicationCreate{cli: fake.NewSimpleClientset()}
		if tc.failCreates {
			creator.cli.(*fake.Clientset).Fake.PrependReactor("create", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		pod, err := creator.CreatePod(ctx, tc.args)
		c.Check(pod, tc.podChecker)
		c.Check(err, tc.errChecker)
		if pod != nil && err == nil {
			args, ok := tc.args.(*CreatePodArgs)
			c.Assert(ok, Equals, true)
			_, ok = pod.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			c.Assert(pod.GenerateName, Equals, args.genName)
			c.Assert(pod.Namespace, Equals, args.namespace)
			c.Assert(len(pod.Spec.Containers), Equals, 1)
			c.Assert(pod.Spec.Containers[0].Name, Equals, args.genName)
			c.Assert(pod.Spec.Containers[0].Command, DeepEquals, []string{"/bin/sh"})
			c.Assert(pod.Spec.Containers[0].Args, DeepEquals, []string{"-c", args.cmd})
			c.Assert(pod.Spec.Containers[0].VolumeMounts, DeepEquals, []v1.VolumeMount{{
				Name:      "persistent-storage",
				MountPath: "/data",
			}})
			c.Assert(pod.Spec.Volumes, DeepEquals, []v1.Volume{{
				Name: "persistent-storage",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: args.pvcName,
					},
				}},
			})
			if args.containerImage == "" {
				c.Assert(pod.Spec.Containers[0].Image, Equals, DefaultPodImage)
			} else {
				c.Assert(pod.Spec.Containers[0].Image, Equals, args.containerImage)
			}
			if args.runAsUser > 0 {
				c.Assert(pod.Spec.SecurityContext, DeepEquals, &v1.PodSecurityContext{
					RunAsUser: &args.runAsUser,
					FSGroup:   &args.runAsUser,
				})
			} else {
				c.Check(pod.Spec.SecurityContext, IsNil)
			}

		}
	}
}

func (s *CSITestSuite) TestValidateArgs(c *C) {
	ctx := context.Background()
	type fields struct {
		validateOps *mocks.MockArgumentValidator
		versionOps  *mocks.MockApiVersionFetcher
	}
	for _, tc := range []struct {
		args       *CSISnapshotRestoreArgs
		prepare    func(f *fields)
		errChecker Checker
	}{
		{ // valid args
			args: &CSISnapshotRestoreArgs{
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
							GroupVersion: alphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: alphaVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							VolSnapClassAlphaDriverKey: "p1",
						},
					}, nil),
				)
			},
			errChecker: IsNil,
		},
		{ // driver mismatch
			args: &CSISnapshotRestoreArgs{
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
							GroupVersion: alphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: alphaVersion,
					}).Return(&unstructured.Unstructured{
						Object: map[string]interface{}{
							VolSnapClassAlphaDriverKey: "p2",
						},
					}, nil),
				)
			},
			errChecker: NotNil,
		},
		{ // vsc error
			args: &CSISnapshotRestoreArgs{
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
							GroupVersion: alphaVersion,
						}, nil),
					f.validateOps.EXPECT().ValidateVolumeSnapshotClass(gomock.Any(), "vsc", &metav1.GroupVersionForDiscovery{
						GroupVersion: alphaVersion,
					}).Return(nil, fmt.Errorf("vsc error")),
				)
			},
			errChecker: NotNil,
		},
		{ // groupversion error
			args: &CSISnapshotRestoreArgs{
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
			args: &CSISnapshotRestoreArgs{
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
			args: &CSISnapshotRestoreArgs{
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
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		}, {
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		}, {
			args: &CSISnapshotRestoreArgs{
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
		stepper := &snapshotRestoreStepper{
			validateOps:  f.validateOps,
			versionFetch: f.versionOps,
		}
		err := stepper.validateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestCreateApplication(c *C) {
	ctx := context.Background()
	type fields struct {
		createAppOps *mocks.MockApplicationCreator
	}
	for _, tc := range []struct {
		args       *CSISnapshotRestoreArgs
		genString  string
		prepare    func(f *fields)
		errChecker Checker
		podChecker Checker
		pvcChecker Checker
	}{
		{
			args: &CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &CreatePVCArgs{
						genName:      originalPVCGenerateName,
						storageClass: "sc",
						namespace:    "ns",
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &CreatePodArgs{
						genName:        originalPodGenerateName,
						pvcName:        "pvc1",
						namespace:      "ns",
						cmd:            "echo 'some string' >> /data/out.txt; sync; tail -f /dev/null",
						runAsUser:      100,
						containerImage: "image",
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
			args: &CSISnapshotRestoreArgs{
				StorageClass:   "sc",
				Namespace:      "ns",
				RunAsUser:      100,
				ContainerImage: "image",
			},
			genString: "some string",
			prepare: func(f *fields) {
				gomock.InOrder(
					f.createAppOps.EXPECT().CreatePVC(gomock.Any(), &CreatePVCArgs{
						genName:      originalPVCGenerateName,
						storageClass: "sc",
						namespace:    "ns",
					}).Return(&v1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pvc1",
						},
					}, nil),
					f.createAppOps.EXPECT().CreatePod(gomock.Any(), &CreatePodArgs{
						genName:        originalPodGenerateName,
						pvcName:        "pvc1",
						namespace:      "ns",
						cmd:            "echo 'some string' >> /data/out.txt; sync; tail -f /dev/null",
						runAsUser:      100,
						containerImage: "image",
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
			args: &CSISnapshotRestoreArgs{
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
			args: &CSISnapshotRestoreArgs{
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
		stepper := &snapshotRestoreStepper{
			createAppOps: f.createAppOps,
		}
		pod, pvc, err := stepper.createApplication(ctx, tc.args, tc.genString)
		c.Check(err, tc.errChecker)
		c.Check(pod, tc.podChecker)
		c.Check(pvc, tc.pvcChecker)
	}
}
