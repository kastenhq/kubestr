package kubestr

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
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
	result = append(result, p.getRBAC())
	result = append(result, p.getAggregatedLayer())
	return result
}

// validateK8sVersion validates the clusters K8s version
func (p *Kubestr) validateK8sVersion() *TestOutput {
	testName := "Kubernetes Version Check"
	version, err := p.getK8sVersion()
	if err != nil {
		return makeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return makeTestOutput(testName, StatusOK, fmt.Sprintf("Valid kubernetes version (%s)", version.String()), version)
}

// getK8sVersion fetches the k8s vesion
func (p *Kubestr) getK8sVersion() (*version.Info, error) {
	version, err := p.cli.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	majorStr := version.Major
	if string(majorStr[len(majorStr)-1]) == "+" {
		majorStr = majorStr[:len(majorStr)-1]
	}
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to derive kubernetes major version")
	}

	minorStr := version.Minor
	if string(minorStr[len(minorStr)-1]) == "+" {
		minorStr = minorStr[:len(minorStr)-1]
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to derive kubernetes minor version")
	}
	if (major < MinK8sMajorVersion) ||
		(major == MinK8sMajorVersion && minor < MinK8sMinorVersion) {
		return version, fmt.Errorf("Current kubernetes version (%s) is not supported. Minimum version is %s", version.String(), MinK8sGitVersion)
	}
	return version, nil
}

// getRBAC runs the Rbac test
func (p *Kubestr) getRBAC() *TestOutput {
	testName := "RBAC Check"
	//fmt.Println("  Checking if Kubernetes RBAC is enabled:")
	serverGroups, err := p.cli.Discovery().ServerGroups()
	if err != nil {
		return makeTestOutput(testName, StatusError, err.Error(), nil)
	}
	for _, group := range serverGroups.Groups {
		if group.Name == RbacGroupName {
			return makeTestOutput(testName, StatusOK, "Kubernetes RBAC is enabled", group)
		}
	}
	return makeTestOutput(testName, StatusError, "Kubernetes RBAC is not enabled", nil)
}

// getAggregatedLayer checks the aggregated API layer
func (p *Kubestr) getAggregatedLayer() *TestOutput {
	testName := "Aggregated Layer Check"
	_, serverResources, err := p.cli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return makeTestOutput(testName, StatusError, err.Error(), nil)
	}
	for _, resourceList := range serverResources {
		if resourceList.GroupVersion == "apiregistration.k8s.io/v1" || resourceList.GroupVersion == "apiregistration.k8s.io/v1beta1" {
			return makeTestOutput(testName, StatusOK, "The Kubernetes Aggregated Layer is enabled", resourceList)
		}
	}
	return makeTestOutput(testName, StatusError, "Can not detect the Aggregated Layer. Is it enabled?", nil)
}
