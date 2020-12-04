package csi

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kansnapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	"github.com/kastenhq/kubestr/pkg/csi/types"
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
		args        *types.CreatePVCArgs
		failCreates bool
		errChecker  Checker
		pvcChecker  Checker
	}{
		{
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
			args: &types.CreatePVCArgs{
				GenerateName: "genName",
				StorageClass: "sc",
				Namespace:    "ns",
			},
			errChecker: IsNil,
			pvcChecker: NotNil,
		},
		{
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
			args: &types.CreatePVCArgs{
				GenerateName: "",
				StorageClass: "sc",
				Namespace:    "ns",
			},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
		{
			args: &types.CreatePVCArgs{
				GenerateName: "something",
				StorageClass: "",
				Namespace:    "ns",
			},
			errChecker: NotNil,
			pvcChecker: IsNil,
		},
		{
			args: &types.CreatePVCArgs{
				GenerateName: "Something",
				StorageClass: "sc",
				Namespace:    "",
			},
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
			_, ok := pvc.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			c.Assert(pvc.GenerateName, Equals, tc.args.GenerateName)
			c.Assert(pvc.Namespace, Equals, tc.args.Namespace)
			c.Assert(pvc.Spec.AccessModes, DeepEquals, []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce})
			c.Assert(*pvc.Spec.StorageClassName, Equals, tc.args.StorageClass)
			c.Assert(pvc.Spec.DataSource, DeepEquals, tc.args.DataSource)
			if tc.args.RestoreSize != nil {
				c.Assert(pvc.Spec.Resources, DeepEquals, v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: *tc.args.RestoreSize,
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
		args        *types.CreatePodArgs
		failCreates bool
		errChecker  Checker
		podChecker  Checker
	}{
		{
			args: &types.CreatePodArgs{
				GenerateName:   "name",
				PVCName:        "pvcname",
				Namespace:      "ns",
				Cmd:            "somecommand",
				RunAsUser:      1000,
				ContainerImage: "containerimage",
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "name",
				PVCName:      "pvcname",
				Namespace:    "ns",
				Cmd:          "somecommand",
			},
			errChecker: IsNil,
			podChecker: NotNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "name",
				PVCName:      "pvcname",
				Namespace:    "ns",
				Cmd:          "somecommand",
			},
			failCreates: true,
			errChecker:  NotNil,
			podChecker:  NotNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "",
				PVCName:      "pvcname",
				Namespace:    "ns",
				Cmd:          "somecommand",
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "name",
				PVCName:      "",
				Namespace:    "ns",
				Cmd:          "somecommand",
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "name",
				PVCName:      "pvcname",
				Namespace:    "",
				Cmd:          "somecommand",
			},
			errChecker: NotNil,
			podChecker: IsNil,
		},
		{
			args: &types.CreatePodArgs{
				GenerateName: "name",
				PVCName:      "pvcname",
				Namespace:    "ns",
				Cmd:          "",
			},
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
			_, ok := pod.Labels[createdByLabel]
			c.Assert(ok, Equals, true)
			c.Assert(pod.GenerateName, Equals, tc.args.GenerateName)
			c.Assert(pod.Namespace, Equals, tc.args.Namespace)
			c.Assert(len(pod.Spec.Containers), Equals, 1)
			c.Assert(pod.Spec.Containers[0].Name, Equals, tc.args.GenerateName)
			c.Assert(pod.Spec.Containers[0].Command, DeepEquals, []string{"/bin/sh"})
			c.Assert(pod.Spec.Containers[0].Args, DeepEquals, []string{"-c", tc.args.Cmd})
			c.Assert(pod.Spec.Containers[0].VolumeMounts, DeepEquals, []v1.VolumeMount{{
				Name:      "persistent-storage",
				MountPath: "/data",
			}})
			c.Assert(pod.Spec.Volumes, DeepEquals, []v1.Volume{{
				Name: "persistent-storage",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: tc.args.PVCName,
					},
				}},
			})
			if tc.args.ContainerImage == "" {
				c.Assert(pod.Spec.Containers[0].Image, Equals, DefaultPodImage)
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
				getSnap: &v1alpha1.VolumeSnapshot{
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
	for _, tc := range []struct {
		snapshotter kansnapshot.Snapshotter
		args        *types.CreateFromSourceCheckArgs
		errChecker  Checker
	}{
		{
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
			errChecker: IsNil,
		},
		{
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
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				gsErr: fmt.Errorf("gs error"),
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{
				cvsErr: fmt.Errorf("cvs error"),
			},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "",
				SnapshotName:        "snapshot",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "",
				Namespace:           "ns",
			},
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{},
			args: &types.CreateFromSourceCheckArgs{
				VolumeSnapshotClass: "vsc",
				SnapshotName:        "snapshot",
				Namespace:           "",
			},
			errChecker: NotNil,
		},
		{
			snapshotter: &fakeSnapshotter{},
			errChecker:  NotNil,
		},
		{
			errChecker: NotNil,
		},
	} {
		snapCreator := &snapshotCreate{
			dynCli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
		}
		err := snapCreator.CreateFromSourceCheck(ctx, tc.snapshotter, tc.args)
		c.Check(err, tc.errChecker)
	}
}

type fakeSnapshotter struct {
	name string

	createErr error

	getSnap *v1alpha1.VolumeSnapshot
	getErr  error

	cvsErr error

	gsSrc *kansnapshot.Source
	gsErr error

	cfsErr error
}

func (f *fakeSnapshotter) GetVolumeSnapshotClass(annotationKey, annotationValue, storageClassName string) (string, error) {
	return "", nil
}
func (f *fakeSnapshotter) CloneVolumeSnapshotClass(sourceClassName, targetClassName, newDeletionPolicy string, excludeAnnotations []string) error {
	return f.cvsErr
}
func (f *fakeSnapshotter) Create(ctx context.Context, name, namespace, pvcName string, snapshotClass *string, waitForReady bool) error {
	return f.createErr
}
func (f *fakeSnapshotter) Get(ctx context.Context, name, namespace string) (*v1alpha1.VolumeSnapshot, error) {
	return f.getSnap, f.getErr
}
func (f *fakeSnapshotter) Delete(ctx context.Context, name, namespace string) (*v1alpha1.VolumeSnapshot, error) {
	return nil, nil
}
func (f *fakeSnapshotter) DeleteContent(ctx context.Context, name string) error { return nil }
func (f *fakeSnapshotter) Clone(ctx context.Context, name, namespace, cloneName, cloneNamespace string, waitForReady bool) error {
	return nil
}
func (f *fakeSnapshotter) GetSource(ctx context.Context, snapshotName, namespace string) (*kansnapshot.Source, error) {
	return f.gsSrc, f.gsErr
}
func (f *fakeSnapshotter) CreateFromSource(ctx context.Context, source *kansnapshot.Source, snapshotName, namespace string, waitForReady bool) error {
	return f.cfsErr
}
func (f *fakeSnapshotter) CreateContentFromSource(ctx context.Context, source *kansnapshot.Source, contentName, snapshotName, namespace, deletionPolicy string) error {
	return nil
}
func (f *fakeSnapshotter) WaitOnReadyToUse(ctx context.Context, snapshotName, namespace string) error {
	return nil
}
