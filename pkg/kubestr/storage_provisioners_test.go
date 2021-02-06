package kubestr

import (
	"context"
	"fmt"

	"github.com/kanisterio/kanister/pkg/kube/snapshot/apis/v1alpha1"
	. "gopkg.in/check.v1"
	scv1 "k8s.io/api/storage/v1"
	"k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	version "k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type ProvisionerTestSuite struct{}

var _ = Suite(&ProvisionerTestSuite{})

func (s *ProvisionerTestSuite) TestHasCSIDriverObject(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli             kubernetes.Interface
		provisionerName string
		hasDriver       bool
	}{
		{
			cli:             fake.NewSimpleClientset(),
			provisionerName: "provisioner",
			hasDriver:       false,
		},
		{
			cli: fake.NewSimpleClientset(&v1beta1.CSIDriverList{
				Items: []v1beta1.CSIDriver{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "drivername",
						},
					},
				}}),
			provisionerName: "drivername",
			hasDriver:       true,
		},
	} {
		p := &Kubestr{cli: tc.cli}
		hasDriver := p.hasCSIDriverObject(ctx, tc.provisionerName)
		c.Assert(hasDriver, Equals, tc.hasDriver)
	}
}

func (s *ProvisionerTestSuite) TestIsK8sVersionCSISnapshotCapable(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		ver     *version.Info
		checker Checker
		capable bool
		sdsfg   snapshotDataSourceFG
	}{
		{
			ver:     &version.Info{Major: "1", Minor: "", GitVersion: "v1.17"},
			checker: NotNil,
			capable: false,
		},
		{
			ver:     &version.Info{Major: "1", Minor: "15+", GitVersion: "v1.15+"},
			checker: NotNil,
			capable: false,
			sdsfg:   &fakeSDSFGValidator{err: fmt.Errorf("someerror"), cap: false},
		},
		{
			ver:     &version.Info{Major: "1", Minor: "15+", GitVersion: "v1.15+"},
			checker: IsNil,
			capable: true,
			sdsfg:   &fakeSDSFGValidator{err: nil, cap: true},
		},
		{
			ver:     &version.Info{Major: "1", Minor: "17", GitVersion: "v1.17"},
			checker: IsNil,
			capable: true,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = tc.ver
		p := &Kubestr{cli: cli, sdsfgValidator: tc.sdsfg}
		cap, err := p.isK8sVersionCSISnapshotCapable(ctx)
		c.Check(err, tc.checker)
		c.Assert(cap, Equals, tc.capable)
	}
}

type fakeSDSFGValidator struct {
	err error
	cap bool
}

func (f *fakeSDSFGValidator) validate(ctx context.Context) (bool, error) {
	return f.cap, f.err
}

func (s *ProvisionerTestSuite) TestValidateVolumeSnapshotClass(c *C) {
	for _, tc := range []struct {
		vsc          unstructured.Unstructured
		groupVersion string
		out          *VSCInfo
	}{
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "vsc1",
					},
					"snapshotter": "something",
				},
			},
			groupVersion: "snapshot.storage.k8s.io/v1alpha1",
			out: &VSCInfo{
				Name: "vsc1",
			},
		},
		{ // failure
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "vsc1",
					},
					"notsnapshotter": "something",
				},
			},
			groupVersion: "snapshot.storage.k8s.io/v1alpha1",
			out: &VSCInfo{
				Name: "vsc1",
				StatusList: []Status{
					makeStatus(StatusError, fmt.Sprintf("VolumeSnapshotClass (%s) missing 'snapshotter' field", "vsc1"), nil),
				},
			},
		},
		{
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "vsc1",
					},
					"driver": "something",
				},
			},
			groupVersion: "snapshot.storage.k8s.io/v1beta1",
			out: &VSCInfo{
				Name: "vsc1",
			},
		},
		{ // failure
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "vsc1",
					},
					"notdriver": "something",
				},
			},
			groupVersion: "snapshot.storage.k8s.io/v1beta1",
			out: &VSCInfo{
				Name: "vsc1",
				StatusList: []Status{
					makeStatus(StatusError, fmt.Sprintf("VolumeSnapshotClass (%s) missing 'driver' field", "vsc1"), nil),
				},
			},
		},
	} {
		p := &Kubestr{}
		out := p.validateVolumeSnapshotClass(tc.vsc, tc.groupVersion)
		c.Assert(out.Name, Equals, tc.out.Name)
		c.Assert(len(out.StatusList), Equals, len(tc.out.StatusList))
	}
}

func (s *ProvisionerTestSuite) TestLoadStorageClassesAndProvisioners(c *C) {
	ctx := context.Background()
	p := &Kubestr{cli: fake.NewSimpleClientset(
		&scv1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc1"}, Provisioner: "provisioner1"},
		&scv1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc2"}, Provisioner: "provisioner2"},
	)}
	scs, err := p.loadStorageClasses(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(scs.Items), Equals, 2)
	c.Assert(scs, Equals, p.storageClassList)

	// reload has the same
	p.cli = fake.NewSimpleClientset()
	scs, err = p.loadStorageClasses(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(scs.Items), Equals, 2)
	c.Assert(scs, Equals, p.storageClassList)

	// proviosners uses loaded list
	provisioners, err := p.provisionerList(ctx)
	c.Assert(err, IsNil)
	c.Assert(len(provisioners), Equals, 2)
}

func (s *ProvisionerTestSuite) TestLoadVolumeSnaphsotClasses(c *C) {
	ctx := context.Background()
	p := &Kubestr{dynCli: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", v1alpha1.GroupName, v1alpha1.Version),
			"kind":       "VolumeSnapshotClass",
			"metadata": map[string]interface{}{
				"name": "theVSC",
			},
			"snapshotter":    "somesnapshotter",
			"deletionPolicy": "Delete",
		},
	})}
	vsc, err := p.loadVolumeSnapshotClasses(ctx, v1alpha1.Version)
	c.Assert(err, IsNil)
	c.Assert(len(vsc.Items), Equals, 1)
	c.Assert(vsc, Equals, p.volumeSnapshotClassList)

	// reload has the same
	p.dynCli = fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	vsc, err = p.loadVolumeSnapshotClasses(ctx, v1alpha1.Version)
	c.Assert(err, IsNil)
	c.Assert(len(vsc.Items), Equals, 1)
	c.Assert(vsc, Equals, p.volumeSnapshotClassList)
}

func (s *ProvisionerTestSuite) TestGetCSIGroupVersion(c *C) {
	for _, tc := range []struct {
		resources []*metav1.APIResourceList
		out       *metav1.GroupVersionForDiscovery
	}{
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "/////",
				},
			},
			out: nil,
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "snapshot.storage.k8s.io/v1beta1",
				},
				{
					GroupVersion: "snapshot.storage.k8s.io/v1apha1",
				},
			},
			out: &metav1.GroupVersionForDiscovery{
				GroupVersion: "snapshot.storage.k8s.io/v1beta1",
				Version:      "v1beta1",
			},
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "NOTsnapshot.storage.k8s.io/v1beta1",
				},
			},
			out: nil,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).Resources = tc.resources
		p := &Kubestr{cli: cli}
		out := p.getCSIGroupVersion()
		c.Assert(out, DeepEquals, tc.out)
	}
}

func (s *ProvisionerTestSuite) TestGetDriverNameFromUVSC(c *C) {
	for _, tc := range []struct {
		vsc     unstructured.Unstructured
		version string
		out     string
	}{
		{ // alpha success
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"snapshotter": "drivername",
				},
			},
			version: "snapshot.storage.k8s.io/v1alpha1",
			out:     "drivername",
		},
		{ // key missing
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			version: "snapshot.storage.k8s.io/v1alpha1",
			out:     "",
		},
		{ // beta success
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"driver": "drivername",
				},
			},
			version: "snapshot.storage.k8s.io/v1beta1",
			out:     "drivername",
		},
		{ // key missing
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			version: "snapshot.storage.k8s.io/v1beta1",
			out:     "",
		},
		{ // type conversion
			vsc: unstructured.Unstructured{
				Object: map[string]interface{}{
					"driver": int64(1),
				},
			},
			version: "snapshot.storage.k8s.io/v1beta1",
			out:     "",
		},
	} {
		p := &Kubestr{}
		out := p.getDriverNameFromUVSC(tc.vsc, tc.version)
		c.Assert(out, Equals, tc.out)
	}
}

// func (s *ProvisionerTestSuite) TestGetDriverStats(c *C) {
// 	var snapshotCount int
// 	var expansionCount int
// 	var cloningCount int
// 	featureMap := make(map[string]struct{})
// 	for _, driver := range CSIDriverList {
// 		if strings.Contains("Snapshot", driver.Features) {
// 			snapshotCount++
// 		}
// 		if strings.Contains("Expansion", driver.Features) {
// 			expansionCount++
// 		}
// 		if strings.Contains("Cloning", driver.Features) {
// 			cloningCount++
// 		}
// 		featureMap[driver.Features] = struct{}{}
// 	}
// 	c.Log("totalcsidrivers: ", len(CSIDriverList))
// 	c.Log("snapshotCount: ", snapshotCount)
// 	c.Log("expansionCount: ", expansionCount)
// 	c.Log("cloningCount: ", cloningCount)
// 	c.Log("unique combinations: ", len(featureMap))
// 	c.Assert(true, Equals, false)
// }
