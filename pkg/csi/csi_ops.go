package csi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/kanisterio/kanister/pkg/kube"
	kankube "github.com/kanisterio/kanister/pkg/kube"
	kansnapshot "github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/kastenhq/kubestr/pkg/csi/types"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	pf "k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_argument_validator.go -package=mocks . ArgumentValidator
type ArgumentValidator interface {
	//Rename
	ValidatePVC(ctx context.Context, pvcName, namespace string) (*v1.PersistentVolumeClaim, error)
	FetchPV(ctx context.Context, pvName string) (*v1.PersistentVolume, error)
	ValidateNamespace(ctx context.Context, namespace string) error
	ValidateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error)
	ValidateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error)
}

type validateOperations struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (o *validateOperations) ValidatePVC(ctx context.Context, pvcName, namespace string) (*v1.PersistentVolumeClaim, error) {
	if o.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	return o.kubeCli.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
}

func (o *validateOperations) FetchPV(ctx context.Context, pvName string) (*v1.PersistentVolume, error) {
	if o.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	return o.kubeCli.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
}

func (o *validateOperations) ValidateNamespace(ctx context.Context, namespace string) error {
	if o.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	_, err := o.kubeCli.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err
}

func (o *validateOperations) ValidateStorageClass(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	if o.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	return o.kubeCli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
}

func (o *validateOperations) ValidateVolumeSnapshotClass(ctx context.Context, volumeSnapshotClass string, groupVersion *metav1.GroupVersionForDiscovery) (*unstructured.Unstructured, error) {
	if o.dynCli == nil {
		return nil, fmt.Errorf("dynCli not initialized")
	}
	VolSnapClassGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: groupVersion.Version, Resource: common.VolumeSnapshotClassResourcePlural}
	return o.dynCli.Resource(VolSnapClassGVR).Get(ctx, volumeSnapshotClass, metav1.GetOptions{})
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_application_creator.go -package=mocks . ApplicationCreator
type ApplicationCreator interface {
	CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error)
	CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error)
	WaitForPodReady(ctx context.Context, namespace string, podName string) error
}

type applicationCreate struct {
	kubeCli kubernetes.Interface
}

func (c *applicationCreate) CreatePVC(ctx context.Context, args *types.CreatePVCArgs) (*v1.PersistentVolumeClaim, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.GenerateName,
			Namespace:    args.Namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			StorageClassName: &args.StorageClass,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	if args.DataSource != nil {
		pvc.Spec.DataSource = args.DataSource
	}

	if args.RestoreSize != nil && !args.RestoreSize.IsZero() {
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = *args.RestoreSize
	}

	pvcRes, err := c.kubeCli.CoreV1().PersistentVolumeClaims(args.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return pvc, err
	}

	return pvcRes, nil
}

func (c *applicationCreate) CreatePod(ctx context.Context, args *types.CreatePodArgs) (*v1.Pod, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	if args.ContainerImage == "" {
		args.ContainerImage = common.DefaultPodImage
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.GenerateName,
			Namespace:    args.Namespace,
			Labels: map[string]string{
				createdByLabel: "yes",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    args.GenerateName,
				Image:   args.ContainerImage,
				Command: args.Command,
				Args:    args.ContainerArgs,
				VolumeMounts: []v1.VolumeMount{{
					Name:      "persistent-storage",
					MountPath: args.MountPath,
				}},
			}},
			Volumes: []v1.Volume{{
				Name: "persistent-storage",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: args.PVCName,
					},
				}},
			},
		},
	}

	if args.RunAsUser > 0 {
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser: &args.RunAsUser,
			FSGroup:   &args.RunAsUser,
		}
	}

	podRes, err := c.kubeCli.CoreV1().Pods(args.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return pod, err
	}
	return podRes, nil
}

func (c *applicationCreate) WaitForPodReady(ctx context.Context, namespace string, podName string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	err := kankube.WaitForPodReady(ctx, c.kubeCli, namespace, podName)
	return err
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_snapshot_creator.go -package=mocks . SnapshotCreator
type SnapshotCreator interface {
	NewSnapshotter() (kansnapshot.Snapshotter, error)
	CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*snapv1.VolumeSnapshot, error)
	CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error
}

type snapshotCreate struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (c *snapshotCreate) NewSnapshotter() (kansnapshot.Snapshotter, error) {
	if c.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	if c.dynCli == nil {
		return nil, fmt.Errorf("dynCli not initialized")
	}
	return kansnapshot.NewSnapshotter(c.kubeCli, c.dynCli)
}

func (c *snapshotCreate) CreateSnapshot(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateSnapshotArgs) (*snapv1.VolumeSnapshot, error) {
	if snapshotter == nil || args == nil {
		return nil, fmt.Errorf("snapshotter or args are empty")
	}
	if err := args.Validate(); err != nil {
		return nil, err
	}
	err := snapshotter.Create(ctx, args.SnapshotName, args.Namespace, args.PVCName, &args.VolumeSnapshotClass, true, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "CSI Driver failed to create snapshot for PVC (%s) in Namespace (%s)", args.PVCName, args.Namespace)
	}
	snap, err := snapshotter.Get(ctx, args.SnapshotName, args.Namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get CSI snapshot (%s) in Namespace (%s)", args.SnapshotName, args.Namespace)
	}
	return snap, nil
}

func (c *snapshotCreate) CreateFromSourceCheck(ctx context.Context, snapshotter kansnapshot.Snapshotter, args *types.CreateFromSourceCheckArgs, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error {
	if c.dynCli == nil {
		return fmt.Errorf("dynCli not initialized")
	}
	if SnapshotGroupVersion == nil || SnapshotGroupVersion.Version == "" {
		return fmt.Errorf("snapshot group version not provided")
	}
	if snapshotter == nil || args == nil {
		return fmt.Errorf("snapshotter or args are nil")
	}
	if err := args.Validate(); err != nil {
		return err
	}
	targetSnapClassName := clonePrefix + args.VolumeSnapshotClass
	err := snapshotter.CloneVolumeSnapshotClass(ctx, args.VolumeSnapshotClass, targetSnapClassName, kansnapshot.DeletionPolicyRetain, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to clone a VolumeSnapshotClass to use to restore the snapshot")
	}
	defer func() {
		VolSnapClassGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: SnapshotGroupVersion.Version, Resource: common.VolumeSnapshotClassResourcePlural}
		err := c.dynCli.Resource(VolSnapClassGVR).Delete(ctx, targetSnapClassName, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("Delete VSC Error (%s) - (%v)\n", targetSnapClassName, err)
		}
	}()

	snapSrc, err := snapshotter.GetSource(ctx, args.SnapshotName, args.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Failed to get source snapshot source (%s)", args.SnapshotName)
	}
	snapshotCFSCloneName := clonePrefix + args.SnapshotName
	// test the CreateFromSource API
	defer func() {
		_, _ = snapshotter.Delete(context.Background(), snapshotCFSCloneName, args.Namespace)
	}()
	src := &kansnapshot.Source{
		Handle:                  snapSrc.Handle,
		Driver:                  snapSrc.Driver,
		VolumeSnapshotClassName: targetSnapClassName,
	}
	err = snapshotter.CreateFromSource(ctx, src, snapshotCFSCloneName, args.Namespace, true)
	if err != nil {
		return errors.Wrapf(err, "Failed to clone snapshot from source (%s)", snapshotCFSCloneName)
	}
	return nil
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_cleaner.go -package=mocks . Cleaner
type Cleaner interface {
	DeletePVC(ctx context.Context, pvcName string, namespace string) error
	DeletePod(ctx context.Context, podName string, namespace string) error
	DeleteSnapshot(ctx context.Context, snapshotName string, namespace string, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error
}

type cleanse struct {
	kubeCli kubernetes.Interface
	dynCli  dynamic.Interface
}

func (c *cleanse) DeletePVC(ctx context.Context, pvcName string, namespace string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	return c.kubeCli.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
}

func (c *cleanse) DeletePod(ctx context.Context, podName string, namespace string) error {
	if c.kubeCli == nil {
		return fmt.Errorf("kubeCli not initialized")
	}
	return c.kubeCli.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (c *cleanse) DeleteSnapshot(ctx context.Context, snapshotName string, namespace string, SnapshotGroupVersion *metav1.GroupVersionForDiscovery) error {
	if c.dynCli == nil {
		return fmt.Errorf("dynCli not initialized")
	}
	if SnapshotGroupVersion == nil || SnapshotGroupVersion.Version == "" {
		return fmt.Errorf("snapshot group version not provided")
	}
	VolSnapGVR := schema.GroupVersionResource{Group: common.SnapGroupName, Version: SnapshotGroupVersion.Version, Resource: common.VolumeSnapshotResourcePlural}
	return c.dynCli.Resource(VolSnapGVR).Namespace(namespace).Delete(ctx, snapshotName, metav1.DeleteOptions{})
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_api_version_fetcher.go -package=mocks . ApiVersionFetcher
type ApiVersionFetcher interface {
	GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error)
}

type apiVersionFetch struct {
	kubeCli kubernetes.Interface
}

func (p *apiVersionFetch) GetCSISnapshotGroupVersion() (*metav1.GroupVersionForDiscovery, error) {
	if p.kubeCli == nil {
		return nil, fmt.Errorf("kubeCli not initialized")
	}
	groups, _, err := p.kubeCli.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		if group.Name == common.SnapGroupName {
			return &group.PreferredVersion, nil
		}
	}
	return nil, fmt.Errorf("Snapshot API group not found")
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_data_validator.go -package=mocks . DataValidator
type DataValidator interface {
	FetchPodData(podName string, podNamespace string) (string, error)
}

type validateData struct {
	kubeCli kubernetes.Interface
}

func (p *validateData) FetchPodData(podName string, podNamespace string) (string, error) {
	if p.kubeCli == nil {
		return "", fmt.Errorf("kubeCli not initialized")
	}
	stdout, _, err := kankube.Exec(p.kubeCli, podNamespace, podName, "", []string{"sh", "-c", "cat /data/out.txt"}, nil)
	return stdout, err
}

//go:generate go run github.com/golang/mock/mockgen -destination=mocks/mock_port_forwarder.go -package=mocks . PortForwarder
type PortForwarder interface {
	FetchRestConfig() (*rest.Config, error)
	PortForwardAPod(req *types.PortForwardAPodRequest) error
}

type portforward struct{}

func (p *portforward) PortForwardAPod(req *types.PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "https:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})
	fw, err := pf.New(dialer, []string{fmt.Sprintf("%d:%d", req.LocalPort, req.PodPort)}, req.StopCh, req.ReadyCh, &req.OutStream, &req.ErrOutStream)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func (p *portforward) FetchRestConfig() (*rest.Config, error) {
	return kube.LoadConfig()
}
