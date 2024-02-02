package kubestr

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	kanvolume "github.com/kanisterio/kanister/pkg/kube/volume"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// APIVersionKey describes the APIVersion key
	APIVersionKey = "apiVersion"
	// FeatureGateTestPVCName is the name of the pvc created by the feature gate
	// validation test
	FeatureGateTestPVCName = "kubestr-featuregate-test"
	// DefaultNS describes the default namespace
	DefaultNS = "default"
	// PodNamespaceEnvKey describes the pod namespace env variable
	PodNamespaceEnvKey = "POD_NAMESPACE"
)

// Provisioner holds the important information of a provisioner
type Provisioner struct {
	ProvisionerName       string
	CSIDriver             *CSIDriver
	URL                   string
	StorageClasses        []*SCInfo
	VolumeSnapshotClasses []*VSCInfo
	StatusList            []Status
}

type CSIDriver struct {
	NameUrl             string
	DriverName          string
	Versions            string
	Description         string
	Persistence         string
	AccessModes         string
	DynamicProvisioning string
	Features            string
}

func (c *CSIDriver) Provider() string {
	re := regexp.MustCompile(`\[(.*?)\]`)
	match := re.FindStringSubmatch(c.NameUrl) // find the left most match
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func (c *CSIDriver) URL() string {
	re := regexp.MustCompile(`\((.*?)\)`)
	match := re.FindAllStringSubmatch(c.NameUrl, -1)
	if len(match) < 1 {
		return ""
	}
	url := match[len(match)-1] // find the right most match
	if len(url) < 2 {
		return ""
	}
	return url[1]
}

func (c *CSIDriver) Print(prefix string) {
	fmt.Printf(prefix+"  Provider:            %s\n", c.Provider())
	fmt.Printf(prefix+"  Website:             %s\n", c.URL())
	fmt.Printf(prefix+"  Description:         %s\n", c.Description)
	fmt.Printf(prefix+"  Additional Features: %s\n", c.Features)
}

func (c *CSIDriver) SupportsSnapshots() bool {
	return strings.Contains(c.Features, "Snapshot")
}

// SCInfo stores the info of a StorageClass
type SCInfo struct {
	Name       string
	StatusList []Status
	Raw        interface{} `json:",omitempty"`
}

// VSCInfo stores the info of a VolumeSnapshotClass
type VSCInfo struct {
	Name          string
	StatusList    []Status
	HasAnnotation bool
	Raw           interface{} `json:",omitempty"`
}

// Print prints the provionsioner specific details
func (v *Provisioner) Print() {
	printSuccessColor("  " + v.ProvisionerName + ":")
	for _, status := range v.StatusList {
		status.Print("    ")
	}
	switch {
	case v.CSIDriver != nil:
		fmt.Println("    This is a CSI driver!")
		fmt.Println("    (The following info may not be up to date. Please check with the provider for more information.)")
		v.CSIDriver.Print("  ")
	case strings.HasPrefix(v.ProvisionerName, "kubernetes.io"):
		fmt.Println("    This is an in tree provisioner.")
	case strings.Contains(v.ProvisionerName, "csi"):
		fmt.Println("    This might be a CSI Driver. But it is not publicly listed.")
	default:
		fmt.Println("    Unknown driver type.")
	}
	fmt.Println()
	if len(v.StorageClasses) > 0 {
		fmt.Printf("    Storage Classes:\n")
		for _, sc := range v.StorageClasses {
			fmt.Printf("      * %s\n", sc.Name)
			for _, status := range sc.StatusList {
				status.Print("        ")
			}
		}
	}

	if len(v.VolumeSnapshotClasses) > 0 {
		fmt.Printf("    Volume Snapshot Classes:\n")
		for _, vsc := range v.VolumeSnapshotClasses {
			fmt.Printf("      * %s\n", vsc.Name)
			for _, status := range vsc.StatusList {
				status.Print("        ")
			}
		}
	}

	if len(v.StorageClasses) > 0 {
		fmt.Println()
		fmt.Println("    To perform a FIO test, run-")
		fmt.Println("      ./kubestr fio -s <storage class>")
		fmt.Println()
		fmt.Println("    To perform a check for block device support, run-")
		fmt.Println("      ./kubestr blockmount -s <storage class>")
		switch {
		case len(v.VolumeSnapshotClasses) == 0 && v.CSIDriver != nil && v.CSIDriver.SupportsSnapshots():
			fmt.Println()
			fmt.Println("    This provisioner supports snapshots, however no Volume Snaphsot Classes were found.")
		case len(v.VolumeSnapshotClasses) > 0:
			fmt.Println()
			fmt.Println("    To test CSI snapshot/restore functionality, run-")
			fmt.Println("      ./kubestr csicheck -s <storage class> -v <volume snapshot class>")
		}
	}
}

// ValidateProvisioners validates the provisioners in a cluster
func (p *Kubestr) ValidateProvisioners(ctx context.Context) ([]*Provisioner, error) {
	provisionerList, err := p.provisionerList(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error listing provisioners: %s", err.Error())
	}
	var validateProvisionersOutput []*Provisioner
	for _, provisioner := range provisionerList {
		processedProvisioner, err := p.processProvisioner(ctx, provisioner)
		if err != nil {
			return nil, err
		}
		validateProvisionersOutput = append(validateProvisionersOutput, processedProvisioner)
	}
	return validateProvisionersOutput, nil
}

func (p *Kubestr) processProvisioner(ctx context.Context, provisioner string) (*Provisioner, error) {
	retProvisioner := &Provisioner{
		ProvisionerName: provisioner,
	}

	storageClassList, err := p.loadStorageClasses(ctx)
	if err != nil {
		return nil, err
	}
	for _, storageClass := range storageClassList.Items {
		if storageClass.Provisioner == provisioner {
			retProvisioner.StorageClasses = append(retProvisioner.StorageClasses,
				p.validateStorageClass(provisioner, storageClass)) // review this
		}
	}

	for _, csiDriver := range CSIDriverList {
		if strings.Contains(provisioner, csiDriver.DriverName) {
			retProvisioner.CSIDriver = csiDriver
		}
	}

	if retProvisioner.CSIDriver != nil {
		if !p.hasCSIDriverObject(ctx, provisioner) {
			retProvisioner.StatusList = append(retProvisioner.StatusList,
				makeStatus(StatusWarning, "Missing CSIDriver Object. Required by some provisioners.", nil))
		}
		if clusterCsiSnapshotCapable, err := p.isK8sVersionCSISnapshotCapable(ctx); err != nil || !clusterCsiSnapshotCapable {
			retProvisioner.StatusList = append(retProvisioner.StatusList,
				makeStatus(StatusInfo, "Cluster is not CSI snapshot capable. Requires VolumeSnapshotDataSource feature gate.", nil))
			return retProvisioner, errors.Wrap(err, "Failed to validate if Kubernetes version was CSI capable")
		}
		csiSnapshotGroupVersion := p.getCSIGroupVersion()
		if csiSnapshotGroupVersion == nil {
			retProvisioner.StatusList = append(retProvisioner.StatusList,
				makeStatus(StatusInfo, "Can't find the CSI snapshot group api version.", nil))
			return retProvisioner, nil
		}
		// load volumeSnapshotClass
		vscs, err := p.loadVolumeSnapshotClasses(ctx, csiSnapshotGroupVersion.Version)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to load volume snapshot classes")
		}
		for _, vsc := range vscs.Items {
			if p.getDriverNameFromUVSC(vsc, csiSnapshotGroupVersion.GroupVersion) == provisioner {
				retProvisioner.VolumeSnapshotClasses = append(retProvisioner.VolumeSnapshotClasses,
					p.validateVolumeSnapshotClass(vsc, csiSnapshotGroupVersion.GroupVersion))
			}
		}
	}
	return retProvisioner, nil
}

// hasCSIDriverObject sees if a provisioner has a CSIDriver Object
func (p *Kubestr) hasCSIDriverObject(ctx context.Context, provisioner string) bool {
	csiDrivers, err := p.cli.StorageV1beta1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	for _, driver := range csiDrivers.Items {
		if driver.Name == provisioner {
			return true
		}
	}
	return false
}

func (p *Kubestr) isK8sVersionCSISnapshotCapable(ctx context.Context) (bool, error) {
	k8sVersion, err := p.validateK8sVersionHelper()
	if err != nil {
		return false, err
	}
	minorStr := k8sVersion.Minor
	if string(minorStr[len(minorStr)-1]) == "+" {
		minorStr = minorStr[:len(minorStr)-1]
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return false, err
	}
	if minor < 17 && k8sVersion.Major == "1" {
		return p.sdsfgValidator.validate(ctx)
	}
	return true, nil
}

// validateStorageClass validates a storageclass
func (p *Kubestr) validateStorageClass(provisioner string, storageClass sv1.StorageClass) *SCInfo {
	scStatus := &SCInfo{
		Name: storageClass.Name,
		Raw:  storageClass,
	}
	return scStatus
}

// validateVolumeSnapshotClass validates the VolumeSnapshotClass
func (p *Kubestr) validateVolumeSnapshotClass(vsc unstructured.Unstructured, groupVersion string) *VSCInfo {
	retVSC := &VSCInfo{
		Name: vsc.GetName(),
		Raw:  vsc,
	}
	switch groupVersion {
	case common.SnapshotAlphaVersion:
		_, ok := vsc.Object[common.VolSnapClassAlphaDriverKey]
		if !ok {
			retVSC.StatusList = append(retVSC.StatusList,
				makeStatus(StatusError, fmt.Sprintf("VolumeSnapshotClass (%s) missing 'snapshotter' field", vsc.GetName()), nil))
		}
	case common.SnapshotBetaVersion:
		_, ok := vsc.Object[common.VolSnapClassBetaDriverKey]
		if !ok {
			retVSC.StatusList = append(retVSC.StatusList,
				makeStatus(StatusError, fmt.Sprintf("VolumeSnapshotClass (%s) missing 'driver' field", vsc.GetName()), nil))
		}
	case common.SnapshotStableVersion:
		_, ok := vsc.Object[common.VolSnapClassStableDriverKey]
		if !ok {
			retVSC.StatusList = append(retVSC.StatusList,
				makeStatus(StatusError, fmt.Sprintf("VolumeSnapshotClass (%s) missing 'driver' field", vsc.GetName()), nil))
		}
	}
	return retVSC
}

func (p *Kubestr) provisionerList(ctx context.Context) ([]string, error) {
	storageClassList, err := p.loadStorageClasses(ctx)
	if err != nil {
		return nil, err
	}
	provisionerSet := make(map[string]struct{})
	for _, storageClass := range storageClassList.Items {
		provisionerSet[storageClass.Provisioner] = struct{}{}
	}
	return convertSetToSlice(provisionerSet), nil
}

func (p *Kubestr) loadStorageClasses(ctx context.Context) (*sv1.StorageClassList, error) {
	if p.storageClassList == nil {
		sc, err := p.cli.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		p.storageClassList = sc
	}
	return p.storageClassList, nil
}

func (p *Kubestr) loadVolumeSnapshotClasses(ctx context.Context, version string) (*unstructured.UnstructuredList, error) {
	if p.volumeSnapshotClassList == nil {
		VolSnapClassGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: version, Resource: common.VolumeSnapshotClassResourcePlural}
		us, err := p.dynCli.Resource(VolSnapClassGVR).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		p.volumeSnapshotClassList = us
	}
	return p.volumeSnapshotClassList, nil
}

// getDriverNameFromUVSC get the driver name from an unstructured VSC
func (p *Kubestr) getDriverNameFromUVSC(vsc unstructured.Unstructured, version string) string {
	var driverName interface{}
	var ok bool
	switch version {
	case common.SnapshotAlphaVersion:
		driverName, ok = vsc.Object[common.VolSnapClassAlphaDriverKey]
		if !ok {
			return ""
		}
	case common.SnapshotBetaVersion:
		driverName, ok = vsc.Object[common.VolSnapClassBetaDriverKey]
		if !ok {
			return ""
		}
	case common.SnapshotStableVersion:
		driverName, ok = vsc.Object[common.VolSnapClassStableDriverKey]
		if !ok {
			return ""
		}
	}
	driver, ok := driverName.(string)
	if !ok {
		return ""
	}
	return driver
}

// getCSIGroupVersion fetches the CSI Group Version
func (p *Kubestr) getCSIGroupVersion() *metav1.GroupVersionForDiscovery {
	groups, _, err := p.cli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil
	}
	for _, group := range groups {
		if group.Name == common.SnapGroupName {
			return &group.PreferredVersion
		}
	}
	return nil
}

type snapshotDataSourceFG interface {
	validate(ctx context.Context) (bool, error)
}

type snapshotDataSourceFGValidator struct {
	cli    kubernetes.Interface
	dynCli dynamic.Interface
}

func (s *snapshotDataSourceFGValidator) validate(ctx context.Context) (bool, error) {
	ns := getPodNamespace()

	// deletes if exists. If it doesn't exist, this is a noop
	err := kanvolume.DeletePVC(s.cli, ns, FeatureGateTestPVCName)
	if err != nil {
		return false, errors.Wrap(err, "Error deleting VolumeSnapshotDataSource feature-gate validation pvc")
	}
	// defer delete
	defer func() {
		_ = kanvolume.DeletePVC(s.cli, ns, FeatureGateTestPVCName)
	}()

	// create PVC
	snapshotKind := "VolumeSnapshot"
	snapshotAPIGroup := "snapshot.storage.k8s.io"
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: FeatureGateTestPVCName,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			DataSource: &v1.TypedLocalObjectReference{
				APIGroup: &snapshotAPIGroup,
				Kind:     snapshotKind,
				Name:     "fakeSnap",
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	pvcRes, err := s.cli.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return false, errors.Wrap(err, "Error creating VolumeSnapshotDataSource feature-gate validation pvc")
	}
	if pvcRes.Spec.DataSource == nil {
		return false, nil
	}
	return true, nil
}
