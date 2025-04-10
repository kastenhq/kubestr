package kubestr

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	version "k8s.io/apimachinery/pkg/version"
)

const (
	// MinK8sMajorVersion is the minimum supported Major version
	MinK8sMajorVersion = 1
	// MinK8sMinorVersion is the minimum supported Minor version
	MinK8sMinorVersion = 12
	// MinK8sGitVersion is the minimum supported k8s version
	MinK8sGitVersion = "v1.12.0"
	// RbacGroupName describe hte rbac group name
	RbacGroupName = "rbac.authorization.k8s.io"
)

// KubernetesChecks runs all the baseline checks on the cluster
func (p *Kubestr) KubernetesChecks() []*TestOutput {
	var result []*TestOutput
	result = append(result, p.validateK8sVersion())
	result = append(result, p.validateRBAC())
	result = append(result, p.validateAggregatedLayer())
	return result
}

// validateK8sVersion validates the clusters K8s version
func (p *Kubestr) validateK8sVersion() *TestOutput {
	testName := "Kubernetes Version Check"
	version, err := p.validateK8sVersionHelper()
	if err != nil {
		return MakeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return MakeTestOutput(testName, StatusOK, fmt.Sprintf("Valid kubernetes version (%s)", version.String()), version)
}

// getK8sVersion fetches the k8s vesion
func (p *Kubestr) validateK8sVersionHelper() (*version.Info, error) {
	version, err := p.cli.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	majorStr := version.Major
	if len(majorStr) > 1 && string(majorStr[len(majorStr)-1]) == "+" {
		majorStr = majorStr[:len(majorStr)-1]
	}
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to derive kubernetes major version")
	}

	minorStr := version.Minor
	if len(minorStr) > 1 && string(minorStr[len(minorStr)-1]) == "+" {
		minorStr = minorStr[:len(minorStr)-1]
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return nil, errors.Wrap(err, "unable to derive kubernetes minor version")
	}
	if (major < MinK8sMajorVersion) ||
		(major == MinK8sMajorVersion && minor < MinK8sMinorVersion) {
		return version, fmt.Errorf("current kubernetes version (%s) is not supported, minimum version is %s", version.String(), MinK8sGitVersion)
	}
	return version, nil
}

func (p *Kubestr) validateRBAC() *TestOutput {
	testName := "RBAC Check"
	//fmt.Println("  Checking if Kubernetes RBAC is enabled:")
	group, err := p.validateRBACHelper()
	if err != nil {
		return MakeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return MakeTestOutput(testName, StatusOK, "Kubernetes RBAC is enabled", *group)
}

// getRBAC runs the Rbac test
func (p *Kubestr) validateRBACHelper() (*v1.APIGroup, error) {
	serverGroups, err := p.cli.Discovery().ServerGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range serverGroups.Groups {
		if group.Name == RbacGroupName {
			return &group, nil
		}
	}
	return nil, fmt.Errorf("Kubernetes RBAC is not enabled") //nolint:staticcheck
}

func (p *Kubestr) validateAggregatedLayer() *TestOutput {
	testName := "Aggregated Layer Check"
	resourceList, err := p.validateAggregatedLayerHelper()
	if err != nil {
		MakeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return MakeTestOutput(testName, StatusOK, "The Kubernetes Aggregated Layer is enabled", resourceList)
}

// getAggregatedLayer checks the aggregated API layer
func (p *Kubestr) validateAggregatedLayerHelper() (*v1.APIResourceList, error) {
	_, serverResources, err := p.cli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	for _, resourceList := range serverResources {
		if resourceList.GroupVersion == "apiregistration.k8s.io/v1" || resourceList.GroupVersion == "apiregistration.k8s.io/v1beta1" {
			return resourceList, nil
		}
	}
	return nil, fmt.Errorf("can not detect the Aggregated API Layer, is it enabled?")
}
