package csi

import (
	"context"
	"fmt"

	kankube "github.com/kanisterio/kanister/pkg/kube"
	"github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// SnapGroupName describes the snapshot group name
	SnapGroupName = "snapshot.storage.k8s.io"
	// VolumeSnapshotClassResourcePlural  describes volume snapshot classses
	VolumeSnapshotClassResourcePlural = "volumesnapshotclasses"
	// VolSnapClassAlphaDriverKey describes alpha driver key
	VolSnapClassAlphaDriverKey = "snapshotter"
	// VolSnapClassBetaDriverKey describes beta driver key
	VolSnapClassBetaDriverKey = "driver"
	alphaVersion              = "snapshot.storage.k8s.io/v1alpha1"
	betaVersion               = "snapshot.storage.k8s.io/v1beta1"
	originalPVCGenerateName   = "kubestr-csi-original-pvc"
	originalPodGenerateName   = "kubestr-csi-original-pod"
	createdByLabel            = "created-by-kubestr-csi"
	DefaultPodImage           = "ghcr.io/kastenhq/kubestr:latest"
)

type SnapshotRestoreRunner struct {
	KubeCli kubernetes.Interface
	DynCli  dynamic.Interface
	srSteps snapshotRestoreSteps
}

func (r *SnapshotRestoreRunner) RunSnapshotRestore(ctx context.Context, args *CSISnapshotRestoreArgs) (*CSISnapshotRestoreResults, error) {
	// r.srSteps = &snapshotRestoreStepper{
	// 	kubeCli: s.KubeCli,
	// 	dynCli:  s.DynCli,
	// }
	return r.RunSnapshotRestoreHelper(ctx, args)
}

func (r *SnapshotRestoreRunner) RunSnapshotRestoreHelper(ctx context.Context, args *CSISnapshotRestoreArgs) (*CSISnapshotRestoreResults, error) {
	if r.KubeCli == nil || r.DynCli == nil { // for UT purposes
		return nil, fmt.Errorf("cli uninitialized")
	}
	if err := r.srSteps.validateArgs(ctx, args); err != nil {
		return nil, err
	}

	return nil, nil
}

type snapshotRestoreSteps interface {
	validateArgs(ctx context.Context, args *CSISnapshotRestoreArgs) error
	createApplication(ctx context.Context, args *CSISnapshotRestoreArgs, data string) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	validateData(ctx context.Context, args, pod *v1.Pod, data string) error
	snapshotApplication(ctx context.Context, args *CSISnapshotRestoreArgs, pvc *v1.PersistentVolumeClaim) (*v1alpha1.VolumeSnapshot, error)
	restoreApplication(ctx context.Context, args *CSISnapshotRestoreArgs, snapshot *v1alpha1.VolumeSnapshot) (*v1.Pod, *v1.PersistentVolumeClaim, error)
	cleanup(ctx context.Context)
}

type snapshotRestoreStepper struct {
	kubeCli      kubernetes.Interface
	dynCli       dynamic.Interface
	validateOps  ArgumentValidator
	versionFetch ApiVersionFetcher
	createAppOps ApplicationCreator
}

func (s *snapshotRestoreStepper) validateArgs(ctx context.Context, args *CSISnapshotRestoreArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}
	if err := s.validateOps.ValidateNamespace(ctx, args.Namespace); err != nil {
		return err
	}
	sc, err := s.validateOps.ValidateStorageClass(ctx, args.StorageClass)
	if err != nil {
		return err
	}

	groupVersion, err := s.versionFetch.GetCSISnapshotGroupVersion()
	if err != nil {
		return err
	}

	uVSC, err := s.validateOps.ValidateVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, groupVersion)
	if err != nil {
		return err
	}

	vscDriver := getDriverNameFromUVSC(*uVSC, groupVersion.GroupVersion)
	if sc.Provisioner != vscDriver {
		return fmt.Errorf("StorageClass provisioner (%s) and VolumeSnapshotClass driver (%s) are different.", sc.Provisioner, vscDriver)
	}
	return nil
}

func (s *snapshotRestoreStepper) createApplication(ctx context.Context, args *CSISnapshotRestoreArgs, genString string) (*v1.Pod, *v1.PersistentVolumeClaim, error) {
	pvcArgs := &CreatePVCArgs{
		genName:      originalPVCGenerateName,
		storageClass: args.StorageClass,
		namespace:    args.Namespace,
	}
	pvc, err := s.createAppOps.CreatePVC(ctx, pvcArgs)
	if err != nil {
		return nil, nil, err
	}
	podArgs := &CreatePodArgs{
		genName:        originalPodGenerateName,
		pvcName:        pvc.Name,
		namespace:      args.Namespace,
		cmd:            fmt.Sprintf("echo '%s' >> /data/out.txt; sync; tail -f /dev/null", genString),
		runAsUser:      args.RunAsUser,
		containerImage: args.ContainerImage,
	}
	pod, err := s.createAppOps.CreatePod(ctx, podArgs)
	if err != nil {
		return nil, pvc, err
	}
	err = s.createAppOps.WaitForPodReady(ctx, args.Namespace, pod.Name)
	return pod, pvc, err
}

//go:generate mockgen -destination=mocks/mock_argument_validator.go -package=mocks . ArgumentValidator
type ArgumentValidator interface {
	ValidateNamespace(ctx context.Context, namespace string) error
	ValidateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error)
	ValidateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error)
}

type validateOperations struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (o *validateOperations) validateNamespace(ctx context.Context, namespace string) error {
	_, err := o.kubeCli.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err
}

func (o *validateOperations) validateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	return o.kubeCli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
}

func (o *validateOperations) validateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error) {
	VolSnapClassGVR := schema.GroupVersionResource{Group: SnapGroupName, Version: groupVersion.Version, Resource: VolumeSnapshotClassResourcePlural}
	return o.dynCli.Resource(VolSnapClassGVR).Get(ctx, volumeSnapshotClass, metav1.GetOptions{})
}

//go:generate mockgen -destination=mocks/mock_application_creator.go -package=mocks . ApplicationCreator
type ApplicationCreator interface {
	CreatePVC(ctx context.Context, args interface{}) (*v1.PersistentVolumeClaim, error)
	CreatePod(ctx context.Context, args interface{}) (*v1.Pod, error)
	WaitForPodReady(ctx context.Context, namespace string, podName string) error
}

type applicationCreate struct {
	cli kubernetes.Interface
}

type CreatePVCArgs struct {
	genName      string
	storageClass string
	namespace    string
	dataSource   *v1.TypedLocalObjectReference
	restoreSize  *resource.Quantity
}

func (c *applicationCreate) CreatePVC(ctx context.Context, argsInterface interface{}) (*v1.PersistentVolumeClaim, error) {
	args, ok := argsInterface.(*CreatePVCArgs)
	if !ok {
		return nil, fmt.Errorf("Invalid args type for CreatePVC (%T)", argsInterface)
	}
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.genName,
			Namespace:    args.namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			StorageClassName: &args.storageClass,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	if args.dataSource != nil {
		pvc.Spec.DataSource = args.dataSource
	}

	if args.restoreSize != nil && !args.restoreSize.IsZero() {
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = *args.restoreSize
	}

	pvcRes, err := c.cli.CoreV1().PersistentVolumeClaims(args.namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return pvc, err
	}

	return pvcRes, nil
}

type CreatePodArgs struct {
	genName        string
	pvcName        string
	namespace      string
	cmd            string
	runAsUser      int64
	containerImage string
}

func (c *applicationCreate) CreatePod(ctx context.Context, argsInterface interface{}) (*v1.Pod, error) {
	args, ok := argsInterface.(*CreatePodArgs)
	if !ok {
		return nil, fmt.Errorf("Invalid args type for CreatePod (%T)", argsInterface)
	}
	if args.containerImage == "" {
		args.containerImage = DefaultPodImage
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.genName,
			Namespace:    args.namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    args.genName,
				Image:   args.containerImage,
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", args.cmd},
				VolumeMounts: []v1.VolumeMount{{
					Name:      "persistent-storage",
					MountPath: "/data",
				}},
			}},
			Volumes: []v1.Volume{{
				Name: "persistent-storage",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: args.pvcName,
					},
				}},
			},
		},
	}

	if args.runAsUser > 0 {
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser: &args.runAsUser,
			FSGroup:   &args.runAsUser,
		}
	}

	podRes, err := c.cli.CoreV1().Pods(args.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return pod, err
	}
	return podRes, nil
}

func (c *applicationCreate) WaitForPodReady(ctx context.Context, namespace string, podName string) error {
	err := kankube.WaitForPodReady(ctx, c.cli, namespace, podName)
	return err
}

// type createSnapshotInterface interface {
// }

//go:generate mockgen -destination=mocks/mock_api_version_fetcher.go -package=mocks . ApiVersionFetcher
type ApiVersionFetcher interface {
	GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error)
}

type apiVersionFetch struct {
	cli kubernetes.Interface
}

func (p *apiVersionFetch) getCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error) {
	groups, _, err := p.cli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Name == SnapGroupName {
			return &group.PreferredVersion, nil
		}
	}
	return nil, fmt.Errorf("Snapshot API group not found")
}

func getDriverNameFromUVSC(vsc unstructured.Unstructured, version string) string {
	var driverName interface{}
	var ok bool
	switch version {
	case alphaVersion:
		driverName, ok = vsc.Object[VolSnapClassAlphaDriverKey]
		if !ok {
			return ""
		}

	case betaVersion:
		driverName, ok = vsc.Object[VolSnapClassBetaDriverKey]
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
