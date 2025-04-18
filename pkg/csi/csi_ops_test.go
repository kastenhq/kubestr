package csi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	kansnapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	pkgerrors "github.com/pkg/errors"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func (s *CSITestSuite) TestGetDriverNameFromUVSC(c *C) {

	for _, tc := range []struct {
		vsc     unstructured.Unstructured
		version string
		expOut  string
	}{
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					common.VolSnapClassDriverKey: "p2",
				},
			},
			version: common.SnapshotVersion,
			expOut:  "p2",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			version: common.SnapshotVersion,
			expOut:  "",
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					common.VolSnapClassDriverKey: map[string]string{},
				},
			},
			version: common.SnapshotVersion,
			expOut:  "",
		},
	} {
		driverName := getDriverNameFromUVSC(tc.vsc, tc.version)
		c.Assert(driverName, Equals, tc.expOut)
	}

}

func (s *CSITestSuite) TestGetCSISnapshotGroupVersion(c *C) {
	for _, tc := range []struct {
		cli        kubernetes.Interface
		resources  []*metav1.APIResourceList
		errChecker Checker
		gvChecker  Checker
	}{
		{
			cli: fake.NewSimpleClientset(),
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "/////",
				},
			},
			errChecker: NotNil,
			gvChecker:  IsNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "snapshot.storage.k8s.io/v1",
				},
			},
			errChecker: IsNil,
			gvChecker:  NotNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "notrbac.authorization.k8s.io/v1",
				},
			},
			errChecker: NotNil,
			gvChecker:  IsNil,
		},
		{
			cli:        nil,
			resources:  nil,
			errChecker: NotNil,
			gvChecker:  IsNil,
		},
	} {
		cli := tc.cli
		if cli != nil {
			cli.Discovery().(*discoveryfake.FakeDiscovery).Resources = tc.resources
		}
		p := &apiVersionFetch{kubeCli: cli}
		gv, err := p.GetCSISnapshotGroupVersion()
		c.Check(err, tc.errChecker)
		c.Check(gv, tc.gvChecker)
	}
}

func (s *CSITestSuite) TestValidatePVC(c *C) {
	ctx := context.Background()
	ops := NewArgumentValidator(fake.NewSimpleClientset(), nil)
	pvc, err := ops.ValidatePVC(ctx, "pvc", "ns")
	c.Check(err, NotNil)
	c.Check(pvc, IsNil)

	ops = NewArgumentValidator(fake.NewSimpleClientset(&v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns",
			Name:      "pvc",
		},
	}), nil)
	pvc, err = ops.ValidatePVC(ctx, "pvc", "ns")
	c.Check(err, IsNil)
	c.Check(pvc, NotNil)

	ops = NewArgumentValidator(nil, nil)
	pvc, err = ops.ValidatePVC(ctx, "pvc", "ns")
	c.Check(err, NotNil)
	c.Check(pvc, IsNil)
}

func (s *CSITestSuite) TestFetchPV(c *C) {
	ctx := context.Background()
	ops := NewArgumentValidator(fake.NewSimpleClientset(), nil)
	pv, err := ops.FetchPV(ctx, "pv")
	c.Check(err, NotNil)
	c.Check(pv, IsNil)

	ops = NewArgumentValidator(fake.NewSimpleClientset(&v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv",
		},
	}), nil)
	pv, err = ops.FetchPV(ctx, "pv")
	c.Check(err, IsNil)
	c.Check(pv, NotNil)

	ops = NewArgumentValidator(nil, nil)
	pv, err = ops.FetchPV(ctx, "pv")
	c.Check(err, NotNil)
	c.Check(pv, IsNil)
}

func (s *CSITestSuite) TestValidateNamespace(c *C) {
	ctx := context.Background()
	ops := NewArgumentValidator(fake.NewSimpleClientset(), nil)
	err := ops.ValidateNamespace(ctx, "ns")
	c.Check(err, NotNil)

	ops = NewArgumentValidator(fake.NewSimpleClientset(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	}), nil)
	err = ops.ValidateNamespace(ctx, "ns")
	c.Check(err, IsNil)

	ops = NewArgumentValidator(nil, nil)
	err = ops.ValidateNamespace(ctx, "ns")
	c.Check(err, NotNil)
}

func (s *CSITestSuite) TestValidateStorageClass(c *C) {
	ctx := context.Background()
	ops := &validateOperations{
		kubeCli: fake.NewSimpleClientset(),
	}
	sc, err := ops.ValidateStorageClass(ctx, "sc")
	c.Check(err, NotNil)
	c.Check(sc, IsNil)

	ops = &validateOperations{
		kubeCli: fake.NewSimpleClientset(&sv1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sc",
			},
		}),
	}
	sc, err = ops.ValidateStorageClass(ctx, "sc")
	c.Check(err, IsNil)
	c.Check(sc, NotNil)

	ops = &validateOperations{
		kubeCli: nil,
	}
	sc, err = ops.ValidateStorageClass(ctx, "sc")
	c.Check(err, NotNil)
	c.Check(sc, IsNil)
}

func (s *CSITestSuite) TestValidateVolumeSnapshotClass(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		ops          *validateOperations
		groupVersion string
		version      string
		errChecker   Checker
		uVCSChecker  Checker
	}{
		{
			ops: &validateOperations{
				dynCli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			},
			groupVersion: common.SnapshotVersion,
			errChecker:   NotNil,
			uVCSChecker:  IsNil,
		},
		{
			ops: &validateOperations{
				dynCli: fakedynamic.NewSimpleDynamicClient(
					runtime.NewScheme(),
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": fmt.Sprintf("%s/%s", kansnapshot.GroupName, kansnapshot.Version),
							"kind":       "VolumeSnapshotClass",
							"metadata": map[string]interface{}{
								"name": "vsc",
							},
							"driver":         "somesnapshotter",
							"deletionPolicy": "Delete",
						},
					},
				),
			},
			groupVersion: common.SnapshotVersion,
			version:      kansnapshot.Version,
			errChecker:   IsNil,
			uVCSChecker:  NotNil,
		},
	} {
		uVSC, err := tc.ops.ValidateVolumeSnapshotClass(ctx, "vsc", &metav1.GroupVersionForDiscovery{GroupVersion: tc.groupVersion, Version: tc.version})
		c.Check(err, tc.errChecker)
		c.Check(uVSC, tc.uVCSChecker)
	}
}

func (s *CSITestSuite) TestCreatePVC(c *C) {
	ctx := context.Background()
	resourceQuantity := resource.MustParse("1Gi")
	for _, tc := range []struct {
		cli         kubernetes.Interface
		args        *types.CreatePVCArgs
		failCreates bool
		errChecker  Checker
		pvcChecker  Checker
	}{
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "genName",
				StorageClass: "sc",
				Namespace:    "ns",
				DataSource: &v1.TypedLocalObjectReference{
					Name: "ds",
				},
				RestoreSize: &resourceQuantity,
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "genName",
				StorageClass: "sc",
				Namespace:    "ns",
				DataSource: &v1.TypedLocalObjectReference{
					Name: "ds",
				},
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "genName",
				StorageClass: "sc",
				Namespace:    "ns",
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "genName",
				StorageClass: "sc",
				Namespace:    "ns",
			},
			failCreates: true,
			errChecker:  NotNil,
			pvcChecker:  NotNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "",
				StorageClass: "sc",
				Namespace:    "ns",
			},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "something",
				StorageClass: "",
				Namespace:    "ns",
			},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
		{
			cli: fake.NewSimpleClientset(),
			args: &types.CreatePVCArgs{
				GenerateName: "Something",
				StorageClass: "sc",
				Namespace:    "",
			},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
		{
			cli:        nil,
			args:       &types.CreatePVCArgs{},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
	} {
		appCreator := NewApplicationCreator(tc.cli, 0)
		creator := appCreator.(*applicationCreate)
		if tc.failCreates {
			creator.kubeCli.(*fake.Clientset).PrependReactor("create", "persistentvolumeclaims", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		pvc, err := creator.CreatePVC(ctx, tc.args)
		c.Check(pvc, tc.pvcChecker)
		c.Check(err, tc.errChecker)
		if pvc != nil && err == nil {
			_, ok := pvc.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			c.Assert(pvc.GenerateName, Equals, tc.args.GenerateName)
			c.Assert(pvc.Namespace, Equals, tc.args.Namespace)
			c.Assert(pvc.Spec.AccessModes, DeepEquals, []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce})
			c.Assert(*pvc.Spec.StorageClassName, Equals, tc.args.StorageClass)
			c.Assert(pvc.Spec.DataSource, DeepEquals, tc.args.DataSource)
			if tc.args.RestoreSize != nil {
				c.Assert(pvc.Spec.Resources, DeepEquals, v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: *tc.args.RestoreSize,
					},
				})
			} else {
				c.Assert(pvc.Spec.Resources, DeepEquals, v1.VolumeResourceRequirements{
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
		description string
		cli         kubernetes.Interface
		args        *types.CreatePodArgs
		failCreates bool
		errChecker  Checker
		podChecker  Checker
	}{
		{
			description: "pod with container image and runAsUser 1000 created",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName:   "name",
				Namespace:      "ns",
				Command:        []string{"somecommand"},
				RunAsUser:      1000,
				ContainerImage: "containerimage",
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			description: "Pod creation error on kubeCli",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			failCreates: true,
			errChecker:  NotNil,
			podChecker:  NotNil,
		},
		{
			description: "Neither Name nor GenerateName set",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "Both Name and GenerateName set",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Name:         "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "Neither MountPath nor DevicePath set error",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap:       map[string]types.VolumePath{"pvcname": {}},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "Both MountPath and DevicePath set error",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath:  "/mnt/fs",
						DevicePath: "/mnt/dev",
					},
				},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "PVC name not set error",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap:       map[string]types.VolumePath{"": {MountPath: "/mnt/fs"}},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "default namespace pod is created",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			description: "ns namespace pod is created (GenerateName/MountPath)",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				GenerateName: "name",
				Namespace:    "ns",
				Command:      []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						MountPath: "/mnt/fs",
					},
				},
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			description: "ns namespace pod is created (Name/DevicePath)",
			cli:         fake.NewSimpleClientset(),
			args: &types.CreatePodArgs{
				Name:      "name",
				Namespace: "ns",
				Command:   []string{"somecommand"},
				PVCMap: map[string]types.VolumePath{
					"pvcname": {
						DevicePath: "/mnt/dev",
					},
				},
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			description: "kubeCli not initialized",
			cli:         nil,
			args:        &types.CreatePodArgs{},
			errChecker:  NotNil,
			podChecker:  IsNil,
		},
	} {
		fmt.Println("test:", tc.description)
		creator := &applicationCreate{kubeCli: tc.cli}
		if tc.failCreates {
			creator.kubeCli.(*fake.Clientset).PrependReactor("create", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		pod, err := creator.CreatePod(ctx, tc.args)
		c.Check(pod, tc.podChecker)
		c.Check(err, tc.errChecker)
		if pod != nil && err == nil {
			_, ok := pod.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			if tc.args.GenerateName != "" {
				c.Assert(pod.GenerateName, Equals, tc.args.GenerateName)
				c.Assert(pod.Spec.Containers[0].Name, Equals, tc.args.GenerateName)
			} else {
				c.Assert(pod.Name, Equals, tc.args.Name)
				c.Assert(pod.Spec.Containers[0].Name, Equals, tc.args.Name)
			}
			c.Assert(pod.Namespace, Equals, tc.args.Namespace)
			c.Assert(len(pod.Spec.Containers), Equals, 1)
			c.Assert(pod.Spec.Containers[0].Command, DeepEquals, tc.args.Command)
			c.Assert(pod.Spec.Containers[0].Args, DeepEquals, tc.args.ContainerArgs)
			index := 0
			pvcCount := 1
			for pvcName, path := range tc.args.PVCMap {
				if len(path.MountPath) != 0 {
					c.Assert(pod.Spec.Containers[0].VolumeMounts[index], DeepEquals, v1.VolumeMount{
						Name:      fmt.Sprintf("persistent-storage-%d", pvcCount),
						MountPath: path.MountPath,
					})
					c.Assert(pod.Spec.Containers[0].VolumeDevices, IsNil)
				} else {
					c.Assert(pod.Spec.Containers[0].VolumeDevices[index], DeepEquals, v1.VolumeDevice{
						Name:       fmt.Sprintf("persistent-storage-%d", pvcCount),
						DevicePath: path.DevicePath,
					})
					c.Assert(pod.Spec.Containers[0].VolumeMounts, IsNil)
				}
				c.Assert(pod.Spec.Volumes[index], DeepEquals, v1.Volume{
					Name: fmt.Sprintf("persistent-storage-%d", pvcCount),
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				})
				index++
				pvcCount++
			}
			if tc.args.ContainerImage == "" {
				c.Assert(pod.Spec.Containers[0].Image, Equals, common.DefaultPodImage)
			} else {
				c.Assert(pod.Spec.Containers[0].Image, Equals, tc.args.ContainerImage)
			}
			if tc.args.RunAsUser > 0 {
				c.Assert(pod.Spec.SecurityContext, DeepEquals, &v1.PodSecurityContext{
					RunAsUser: &tc.args.RunAsUser,
					FSGroup:   &tc.args.RunAsUser,
				})
			} else {
				c.Check(pod.Spec.SecurityContext, IsNil)
			}
		}
	}
}

func (s *CSITestSuite) TestCreateSnapshot(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		snapshotter kansnapshot.Snapshotter
		args        *types.CreateSnapshotArgs
		snapChecker Checker
		errChecker  Checker
	}{
		{
			snapshotter: &fakeSnapshotter{
				getSnap: &snapv1.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name: "createdName",
					},
				},
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snap1",
			},
			snapChecker: NotNil,
			errChecker:  IsNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				getErr: fmt.Errorf("get Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snap1",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				createErr: fmt.Errorf("create Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snap1",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				createErr: fmt.Errorf("create Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "",
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snap1",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				createErr: fmt.Errorf("create Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snap1",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				createErr: fmt.Errorf("create Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "pvc",
				VolumeSnapshotClass: "",
				SnapshotName:        "snap1",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				createErr: fmt.Errorf("create Error"),
			},
			args: &types.CreateSnapshotArgs{
				Namespace:           "ns",
				PVCName:             "pvc",
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "",
			},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{},
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
		{
			snapChecker: IsNil,
			errChecker:  NotNil,
		},
	} {
		snapCreator := &snapshotCreate{}
		snapshot, err := snapCreator.CreateSnapshot(ctx, tc.snapshotter, tc.args)
		c.Check(snapshot, tc.snapChecker)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestCreateFromSourceCheck(c *C) {
	ctx := context.Background()
	gv := &metav1.GroupVersionForDiscovery{Version: kansnapshot.Version}
	for _, tc := range []struct {
		dyncli       dynamic.Interface
		snapshotter  kansnapshot.Snapshotter
		args         *types.CreateFromSourceCheckArgs
		groupVersion *metav1.GroupVersionForDiscovery
		errChecker   Checker
	}{
		{
			dyncli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{
				gsSrc: &kansnapshot.Source{
					Handle: "handle",
					Driver: "driver",
				},
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   IsNil,
		},
		{
			dyncli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{
				gsSrc: &kansnapshot.Source{
					Handle: "handle",
					Driver: "driver",
				},
				cfsErr: fmt.Errorf("cfs error"),
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{
				gsErr: fmt.Errorf("gs error"),
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{
				cvsErr: fmt.Errorf("cvs error"),
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:      fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:      fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "",
				Namespace:           "ns",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:      fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "",
			},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:       fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotter:  &fakeSnapshotter{},
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:       fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			groupVersion: gv,
			errChecker:   NotNil,
		},
		{
			dyncli:       fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			groupVersion: nil,
			errChecker:   NotNil,
		},
		{
			dyncli:     nil,
			errChecker: NotNil,
		},
	} {
		snapCreator := &snapshotCreate{
			dynCli: tc.dyncli,
		}
		err := snapCreator.CreateFromSourceCheck(ctx, tc.snapshotter, tc.args, tc.groupVersion)
		c.Check(err, tc.errChecker)
	}
}

type fakeSnapshotter struct {
	name string

	createErr error

	getSnap *snapv1.VolumeSnapshot
	getErr  error

	cvsErr error

	gsSrc *kansnapshot.Source
	gsErr error

	cfsErr error
}

func (f *fakeSnapshotter) GroupVersion(ctx context.Context) schema.GroupVersion {
	return schema.GroupVersion{
		Group:   common.SnapGroupName,
		Version: "v1",
	}
}

func (f *fakeSnapshotter) GetVolumeSnapshotClass(ctx context.Context, annotationKey, annotationValue, storageClassName string) (string, error) {
	return "", nil
}
func (f *fakeSnapshotter) CloneVolumeSnapshotClass(ctx context.Context, sourceClassName, targetClassName, newDeletionPolicy string, excludeAnnotations []string) error {
	return f.cvsErr
}
func (f *fakeSnapshotter) Create(ctx context.Context, pvcName string, snapshotClass *string, waitForReady bool, snapshotMeta kansnapshot.ObjectMeta) error {
	return f.createErr
}
func (f *fakeSnapshotter) Get(ctx context.Context, name, namespace string) (*snapv1.VolumeSnapshot, error) {
	return f.getSnap, f.getErr
}
func (f *fakeSnapshotter) Delete(ctx context.Context, name, namespace string) (*snapv1.VolumeSnapshot, error) {
	return nil, nil
}
func (f *fakeSnapshotter) DeleteContent(ctx context.Context, name string) error { return nil }
func (f *fakeSnapshotter) Clone(ctx context.Context, name, namespace string, waitForReady bool, snapshotMeta, contentMeta kansnapshot.ObjectMeta) error {
	return nil
}
func (f *fakeSnapshotter) GetSource(ctx context.Context, snapshotName, namespace string) (*kansnapshot.Source, error) {
	return f.gsSrc, f.gsErr
}
func (f *fakeSnapshotter) CreateFromSource(ctx context.Context, source *kansnapshot.Source, waitForReady bool, snapshotMeta, contentMeta kansnapshot.ObjectMeta) error {
	return f.cfsErr
}
func (f *fakeSnapshotter) CreateContentFromSource(ctx context.Context, source *kansnapshot.Source, snapshotName, snapshotNs, deletionPolicy string, contentMeta kansnapshot.ObjectMeta) error {
	return nil
}
func (f *fakeSnapshotter) WaitOnReadyToUse(ctx context.Context, snapshotName, namespace string) error {
	return nil
}

func (f *fakeSnapshotter) List(ctx context.Context, namespace string, labels map[string]string) (*snapv1.VolumeSnapshotList, error) {
	return nil, nil
}

func (s *CSITestSuite) TestDeletePVC(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli        kubernetes.Interface
		pvcName    string
		namespace  string
		errChecker Checker
	}{
		{
			cli:        fake.NewSimpleClientset(),
			pvcName:    "pvc",
			namespace:  "ns",
			errChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(&v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc",
					Namespace: "notns",
				},
			}),
			pvcName:    "pvc",
			namespace:  "ns",
			errChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(&v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc",
					Namespace: "ns",
				},
			}),
			pvcName:    "pvc",
			namespace:  "ns",
			errChecker: IsNil,
		},
		{
			cli:        nil,
			pvcName:    "pvc",
			namespace:  "ns",
			errChecker: NotNil,
		},
	} {
		cleaner := NewCleaner(tc.cli, nil)
		err := cleaner.DeletePVC(ctx, tc.pvcName, tc.namespace)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestDeletePod(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli        kubernetes.Interface
		podName    string
		namespace  string
		errChecker Checker
	}{
		{
			cli:        fake.NewSimpleClientset(),
			podName:    "pod",
			namespace:  "ns",
			errChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "notns",
				},
			}),
			podName:    "pod",
			namespace:  "ns",
			errChecker: NotNil,
		},
		{
			cli: fake.NewSimpleClientset(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: "ns",
				},
			}),
			podName:    "pod",
			namespace:  "ns",
			errChecker: IsNil,
		},
		{
			cli:        nil,
			podName:    "pod",
			namespace:  "ns",
			errChecker: NotNil,
		},
	} {
		cleaner := &cleanse{
			kubeCli: tc.cli,
		}
		err := cleaner.DeletePod(ctx, tc.podName, tc.namespace)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestDeleteSnapshot(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli          dynamic.Interface
		snapshotName string
		namespace    string
		groupVersion *metav1.GroupVersionForDiscovery
		errChecker   Checker
	}{
		{
			cli:          fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotName: "snap1",
			namespace:    "ns",
			groupVersion: &metav1.GroupVersionForDiscovery{
				Version: kansnapshot.Version,
			},
			errChecker: NotNil,
		},
		{
			cli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme(),
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": fmt.Sprintf("%s/%s", kansnapshot.GroupName, "v1beta1"),
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "snap1",
							"namespace": "ns",
						},
					},
				}),
			snapshotName: "snap1",
			namespace:    "ns",
			errChecker:   NotNil,
			groupVersion: &metav1.GroupVersionForDiscovery{
				Version: kansnapshot.Version,
			},
		},
		{
			cli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme(),
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": fmt.Sprintf("%s/%s", kansnapshot.GroupName, kansnapshot.Version),
						"kind":       "VolumeSnapshot",
						"metadata": map[string]interface{}{
							"name":      "snap1",
							"namespace": "ns",
						},
					},
				}),
			snapshotName: "snap1",
			namespace:    "ns",
			errChecker:   IsNil,
			groupVersion: &metav1.GroupVersionForDiscovery{
				Version: kansnapshot.Version,
			},
		},
		{
			cli:          fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
			snapshotName: "pod",
			namespace:    "ns",
			errChecker:   NotNil,
		},
		{
			cli:          nil,
			snapshotName: "pod",
			namespace:    "ns",
			errChecker:   NotNil,
		},
	} {
		cleaner := NewCleaner(nil, tc.cli)
		err := cleaner.DeleteSnapshot(ctx, tc.snapshotName, tc.namespace, tc.groupVersion)
		c.Check(err, tc.errChecker)
	}
}

func (s *CSITestSuite) TestWaitForPVCReady(c *C) {
	ctx := context.Background()
	const ns = "ns"
	const pvc = "pvc"
	boundPVC := s.getPVC(ns, pvc, v1.ClaimBound)
	claimLostPVC := s.getPVC(ns, pvc, v1.ClaimLost)
	stuckPVC := s.getPVC(ns, pvc, "")
	normalGetFunc := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return
	}
	deadlineExceededGetFunc := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, pkgerrors.Wrapf(context.DeadlineExceeded, "some wrapped error")
	}

	warningEvent := v1.Event{
		Type:    v1.EventTypeWarning,
		Message: "waiting for a volume to be created, either by external provisioner \"ceph.com/rbd\" or manually created by system administrator",
	}
	for _, tc := range []struct {
		description string
		cli         kubernetes.Interface
		pvcGetFunc  func(action k8stesting.Action) (handled bool, ret runtime.Object, err error)
		eventsList  []v1.Event
		errChecker  Checker
		errString   string
	}{
		{
			description: "Happy path",
			cli:         fake.NewSimpleClientset(boundPVC),
			pvcGetFunc:  normalGetFunc,
			errChecker:  IsNil,
		},
		{
			description: "Missing PVC",
			cli:         fake.NewSimpleClientset(),
			pvcGetFunc:  normalGetFunc,
			errChecker:  NotNil,
			errString:   "could not find PVC",
		},
		{
			description: "PVC ClaimLost",
			cli:         fake.NewSimpleClientset(claimLostPVC),
			pvcGetFunc:  normalGetFunc,
			errChecker:  NotNil,
			errString:   "ClaimLost",
		},
		{
			description: "context.DeadlineExceeded but no event warnings",
			cli:         fake.NewSimpleClientset(stuckPVC),
			pvcGetFunc:  deadlineExceededGetFunc,
			errChecker:  NotNil,
			errString:   context.DeadlineExceeded.Error(),
		},
		{
			description: "context.DeadlineExceeded, unable to provision PVC",
			cli:         fake.NewSimpleClientset(stuckPVC),
			pvcGetFunc:  deadlineExceededGetFunc,
			eventsList:  []v1.Event{warningEvent},
			errChecker:  NotNil,
			errString:   warningEvent.Message,
		},
	} {
		fmt.Println("test:", tc.description)
		creator := &applicationCreate{kubeCli: tc.cli}
		creator.kubeCli.(*fake.Clientset).PrependReactor("get", "persistentvolumeclaims", tc.pvcGetFunc)
		creator.kubeCli.(*fake.Clientset).PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &v1.EventList{Items: tc.eventsList}, nil
		})
		err := creator.WaitForPVCReady(ctx, ns, pvc)
		c.Check(err, tc.errChecker)
		if err != nil {
			c.Assert(strings.Contains(err.Error(), tc.errString), Equals, true)
		}
	}
}

func (s *CSITestSuite) getPVC(ns, pvc string, phase v1.PersistentVolumeClaimPhase) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvc,
			Namespace: ns,
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: phase,
		},
	}
}

func (s *CSITestSuite) TestWaitForPodReady(c *C) {
	ctx := context.Background()
	const ns = "ns"
	const podName = "pod"
	readyPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      podName,
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
	warningEvent := v1.Event{
		Type:    v1.EventTypeWarning,
		Message: "warning event",
	}

	for _, tc := range []struct {
		description string
		cli         kubernetes.Interface
		eventsList  []v1.Event
		errChecker  Checker
		errString   string
	}{
		{
			description: "Happy path",
			cli:         fake.NewSimpleClientset(readyPod),
			errChecker:  IsNil,
		},
		{
			description: "Not found",
			cli:         fake.NewSimpleClientset(),
			errChecker:  NotNil,
			errString:   "not found",
		},
		{
			description: "Pod events",
			cli:         fake.NewSimpleClientset(),
			errChecker:  NotNil,
			errString:   "had issues creating Pod",
			eventsList:  []v1.Event{warningEvent},
		},
		{
			description: "No CLI",
			errChecker:  NotNil,
			errString:   "kubeCli not initialized",
		},
	} {
		fmt.Println("TestWaitForPodReady:", tc.description)
		creator := &applicationCreate{kubeCli: tc.cli}
		if len(tc.eventsList) > 0 {
			creator.kubeCli.(*fake.Clientset).PrependReactor("list", "events", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &v1.EventList{Items: tc.eventsList}, nil
			})
		}
		err := creator.WaitForPodReady(ctx, ns, podName)
		c.Check(err, tc.errChecker)
		if err != nil {
			c.Assert(strings.Contains(err.Error(), tc.errString), Equals, true)
		}
	}
}
