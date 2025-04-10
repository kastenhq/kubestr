package kubestr

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	version "k8s.io/apimachinery/pkg/version"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type K8sChecksTestSuite struct{}

var _ = Suite(&K8sChecksTestSuite{})

func (s *K8sChecksTestSuite) TestGetK8sVersion(c *C) {
	for _, tc := range []struct {
		ver     *version.Info
		checker Checker
		out     *version.Info
	}{
		{
			ver:     &version.Info{Major: "1", Minor: "17", GitVersion: "v1.17"},
			checker: IsNil,
			out:     &version.Info{Major: "1", Minor: "17", GitVersion: "v1.17"},
		},
		{
			ver:     &version.Info{Major: "1", Minor: "11", GitVersion: "v1.11"},
			checker: NotNil,
			out:     &version.Info{Major: "1", Minor: "11", GitVersion: "v1.11"},
		},
		{
			ver:     &version.Info{Major: "1", Minor: "", GitVersion: "v1."},
			checker: NotNil,
			out:     nil,
		},
		{
			ver:     &version.Info{Major: "", Minor: "11", GitVersion: "v."},
			checker: NotNil,
			out:     nil,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = tc.ver
		p := &Kubestr{cli: cli}
		out, err := p.validateK8sVersionHelper()
		c.Assert(out, DeepEquals, tc.out)
		c.Check(err, tc.checker)
	}
}

func (s *K8sChecksTestSuite) TestValidateRBAC(c *C) {
	for _, tc := range []struct {
		resources []*metav1.APIResourceList
		checker   Checker
		out       *metav1.APIGroup
	}{
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "/////",
				},
			},
			checker: NotNil,
			out:     nil,
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "rbac.authorization.k8s.io/v1",
				},
			},
			checker: IsNil,
			out: &metav1.APIGroup{
				Name: "rbac.authorization.k8s.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "rbac.authorization.k8s.io/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "rbac.authorization.k8s.io/v1", Version: "v1"},
			},
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "notrbac.authorization.k8s.io/v1",
				},
			},
			checker: NotNil,
			out:     nil,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).Resources = tc.resources
		p := &Kubestr{cli: cli}
		out, err := p.validateRBACHelper()
		c.Assert(out, DeepEquals, tc.out)
		c.Check(err, tc.checker)
	}
}

func (s *K8sChecksTestSuite) TestValidateAggregatedLayer(c *C) {
	for _, tc := range []struct {
		resources []*metav1.APIResourceList
		checker   Checker
		out       *metav1.APIResourceList
	}{
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "/////",
				},
			},
			checker: NotNil,
			out:     nil,
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "apiregistration.k8s.io/v1",
				},
			},
			checker: IsNil,
			out: &metav1.APIResourceList{
				GroupVersion: "apiregistration.k8s.io/v1",
			},
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "apiregistration.k8s.io/v1beta1",
				},
			},
			checker: IsNil,
			out: &metav1.APIResourceList{
				GroupVersion: "apiregistration.k8s.io/v1beta1",
			},
		},
		{
			resources: []*metav1.APIResourceList{
				{
					GroupVersion: "notapiregistration.k8s.io/v1",
				},
			},
			checker: NotNil,
			out:     nil,
		},
	} {
		cli := fake.NewSimpleClientset()
		cli.Discovery().(*discoveryfake.FakeDiscovery).Resources = tc.resources
		p := &Kubestr{cli: cli}
		out, err := p.validateAggregatedLayerHelper()
		c.Assert(out, DeepEquals, tc.out)
		c.Check(err, tc.checker)
	}
}
