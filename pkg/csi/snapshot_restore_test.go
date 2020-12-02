package csi

import (
	"context"
	"fmt"
	"testing"

	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
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

func (s *CSITestSuite) TestValidateArgs(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		args        *CSISnapshotRestoreArgs
		validateOps *fakeValidateOps
		versionOps  *fakeApiVersionFetch
		errChecker  Checker
	}{
		{ // valid args
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				sc: &sv1.StorageClass{
					Provisioner: "p1",
				},
				vsc: &unstructured.Unstructured{
					Object: map[string]interface{}{
						VolSnapClassAlphaDriverKey: "p1",
					},
				},
			},
			versionOps: &fakeApiVersionFetch{
				gvr: &metav1.GroupVersionForDiscovery{
					GroupVersion: alphaVersion,
				},
			},
			errChecker: IsNil,
		},
		{ // driver mismatch
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				sc: &sv1.StorageClass{
					Provisioner: "p1",
				},
				vsc: &unstructured.Unstructured{
					Object: map[string]interface{}{
						VolSnapClassAlphaDriverKey: "p2",
					},
				},
			},
			versionOps: &fakeApiVersionFetch{
				gvr: &metav1.GroupVersionForDiscovery{
					GroupVersion: alphaVersion,
				},
			},
			errChecker: NotNil,
		},
		{ // vsc error
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				vscErr: fmt.Errorf("vsc error"),
			},
			versionOps: &fakeApiVersionFetch{
				gvr: &metav1.GroupVersionForDiscovery{
					GroupVersion: alphaVersion,
				},
			},
			errChecker: NotNil,
		},
		{ // groupversion error
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				vscErr: fmt.Errorf("vsc error"),
			},
			versionOps: &fakeApiVersionFetch{
				err: fmt.Errorf("groupversion error"),
			},
			errChecker: NotNil,
		},
		{
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				scErr: fmt.Errorf("sc error"),
			},
			errChecker: NotNil,
		},
		{
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{
				vnErr: fmt.Errorf("ns error"),
			},
			errChecker: NotNil,
		},
		{
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "",
				VolumeSnapshotClass: "vsc",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{},
			errChecker:  NotNil,
		}, {
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "",
				Namespace:           "ns",
			},
			validateOps: &fakeValidateOps{},
			errChecker:  NotNil,
		}, {
			args: &CSISnapshotRestoreArgs{
				StorageClass:        "sc",
				VolumeSnapshotClass: "vsc",
				Namespace:           "",
			},
			validateOps: &fakeValidateOps{},
			errChecker:  NotNil,
		},
	} {
		stepper := &snapshotRestoreStepper{
			validateOps:  tc.validateOps,
			versionFetch: tc.versionOps,
		}
		err := stepper.validateArgs(ctx, tc.args)
		c.Check(err, tc.errChecker)
	}
}

type fakeValidateOps struct {
	vnErr error

	sc    *sv1.StorageClass
	scErr error

	vsc    *unstructured.Unstructured
	vscErr error
}

func (f *fakeValidateOps) validateNamespace(ctx context.Context, namespace string) error {
	return f.vnErr
}
func (f *fakeValidateOps) validateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	return f.sc, f.scErr
}
func (f *fakeValidateOps) validateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error) {
	return f.vsc, f.vscErr
}

type fakeApiVersionFetch struct {
	gvr *metav1.GroupVersionForDiscovery
	err error
}

func (f *fakeApiVersionFetch) getCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error) {
	return f.gvr, f.err
}
