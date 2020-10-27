package fio

import (
	"context"
	"fmt"
	"os"

	kankube "github.com/kanisterio/kanister/pkg/kube"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultNS describes the default namespace
	DefaultNS = "default"
	// PodNamespaceEnvKey describes the pod namespace env variable
	PodNamespaceEnvKey = "POD_NAMESPACE"
	// DefaultFIOJob describes the default FIO job
	DefaultFIOJob = "default-fio"
	// ConfigMapSCKey describes the storage class key in a config map
	ConfigMapSCKey = "storageclass"
	// ConfigMapSizeKey describes the size key in a config map
	ConfigMapSizeKey = "pvcsize"
	// DefaultPVCSize is the default PVC size
	DefaultPVCSize = "100Gi"
	// PVCGenerateName is the name to generate for the PVC
	PVCGenerateName = "kubestr-fio-pvc-"
	// PodGenerateName is the name to generate for the POD
	PodGenerateName = "kubestr-fio-pod-"
	// ContainerName is the name of the container that runs the job
	ContainerName = "kubestr-fio"
	// PodNameEnvKey is the name of the variable used to get the current pod name
	PodNameEnvKey = "HOSTNAME"
)

// FIO is an interface that represents FIO related commands
type FIO interface {
	RunFio(ctx context.Context, args *RunFIOArgs) (string, error) // , test config
}

// FIOrunner implments FIO
type FIOrunner struct {
	Cli      kubernetes.Interface
	fioSteps fioSteps
}

type RunFIOArgs struct {
	StorageClass  string
	ConfigMapName string
	JobName       string
}

func (f *FIOrunner) RunFio(ctx context.Context, args *RunFIOArgs) (string, error) {
	f.fioSteps = &fioStepper{
		cli:           f.Cli,
		podReady:      &podReadyChecker{cli: f.Cli},
		podSpecMerger: &podSpecMerger{cli: f.Cli},
	}
	return f.RunFioHelper(ctx, args)

}

func (f *FIOrunner) RunFioHelper(ctx context.Context, args *RunFIOArgs) (string, error) {
	// create a configmap with test parameters
	if f.Cli == nil { // for UT purposes
		return "", fmt.Errorf("cli uninitialized")
	}
	if args == nil {
		args = &RunFIOArgs{}
	}
	configMap, err := f.fioSteps.loadConfigMap(ctx, args.ConfigMapName, args.JobName)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create a ConfigMap")
	}

	testFileName, err := fioTestFilename(configMap.Data)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get test file name.")
	}

	size := configMap.Data[ConfigMapSizeKey]
	if size == "" {
		size = DefaultPVCSize
	}

	storageClass := args.StorageClass
	if storageClass == "" {
		if configMap.Data[ConfigMapSCKey] == "" {
			return "", fmt.Errorf("StorageClass must be provided")
		}
		storageClass = configMap.Data[ConfigMapSCKey]
	}

	if err := f.fioSteps.storageClassExists(ctx, storageClass); err != nil {
		return "", errors.Wrap(err, "Cannot find StorageClass")
	}

	pvc, err := f.fioSteps.createPVC(ctx, storageClass, size)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create PVC")
	}
	defer func() {
		_ = f.fioSteps.deletePVC(context.TODO(), pvc.Name)
	}()

	pod, err := f.fioSteps.createPod(ctx, pvc.Name, configMap.Name, testFileName)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create POD")
	}
	defer func() {
		_ = f.fioSteps.deletePod(context.TODO(), pod.Name)
	}()

	// // store fio result
	// if err := f.fioSteps.storeResult(ctx); err != nil {
	// 	return "", errors.Wrap(err, "Failed to store result")
	// }
	return "", nil
}

type fioSteps interface {
	storageClassExists(ctx context.Context, storageClass string) error
	loadConfigMap(ctx context.Context, configMapName, jobName string) (*v1.ConfigMap, error)
	createPVC(ctx context.Context, storageclass, size string) (*v1.PersistentVolumeClaim, error)
	deletePVC(ctx context.Context, pvcName string) error
	createPod(ctx context.Context, pvcName, configMapName, testFileName string) (*v1.Pod, error)
	deletePod(ctx context.Context, podName string) error
	// storeResult(ctx context.Context) error
}

type fioStepper struct {
	cli           kubernetes.Interface
	podReady      waitForPodReady
	podSpecMerger podSpecMerge
}

func (s *fioStepper) storageClassExists(ctx context.Context, storageClass string) error {
	if _, err := s.cli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}

func (s *fioStepper) loadConfigMap(ctx context.Context, configMapName, jobName string) (*v1.ConfigMap, error) {
	if configMapName == "" {
		if jobName == "" {
			jobName = DefaultFIOJob
		}
		cm, ok := fioJobs[jobName]
		if !ok {
			return nil, fmt.Errorf("Predefined job (%s) not found", jobName)
		}
		cmResult, err := s.cli.CoreV1().ConfigMaps(GetPodNamespace()).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "Unable to create configMap for predefined job (%s)", jobName)
		}
		configMapName = cmResult.Name
	}
	// fetch configmap
	configMap, err := s.cli.CoreV1().ConfigMaps(GetPodNamespace()).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to load configMap (%s) in namespace (%s)", configMapName, GetPodNamespace())
	}
	return configMap, nil
}

func (s *fioStepper) createPVC(ctx context.Context, storageclass, size string) (*v1.PersistentVolumeClaim, error) {
	sizeResource, err := resource.ParseQuantity(size)
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to parse PVC size (%s)", size)
	}
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: PVCGenerateName,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: &storageclass,
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): sizeResource,
				},
			},
		},
	}
	return s.cli.CoreV1().PersistentVolumeClaims(GetPodNamespace()).Create(ctx, pvc, metav1.CreateOptions{})
}

func (s *fioStepper) deletePVC(ctx context.Context, pvcName string) error {
	return s.cli.CoreV1().PersistentVolumeClaims(GetPodNamespace()).Delete(ctx, pvcName, metav1.DeleteOptions{})
}

func (s *fioStepper) createPod(ctx context.Context, pvcName, configMapName, testFileName string) (*v1.Pod, error) {
	if pvcName == "" || configMapName == "" || testFileName == "" {
		return nil, fmt.Errorf("Create pod missing required arguments.")
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: PodGenerateName,
			Namespace:    GetPodNamespace(),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:    ContainerName,
				Command: []string{"fio"},
				Args:    []string{"--directory", "/dataset", "/etc/fio-config/$(CONFIG_FILE_NAME)"},
				VolumeMounts: []v1.VolumeMount{
					{Name: "persistent-storage", MountPath: "/dataset"},
					{Name: "config-map", MountPath: "/etc/configmap"},
				},
				Env: []v1.EnvVar{{
					Name:  "CONFIG_FILE_NAME",
					Value: testFileName,
				}},
			}},
			Volumes: []v1.Volume{
				{
					Name: "persistent-storage",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
					},
				},
				{
					Name: "config-map",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				},
			},
		},
	}
	mergedPodSpec, err := s.podSpecMerger.mergePodSpec(ctx, GetPodNamespace(), pod.Spec)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to merge Pod Spec with parent pod.")
	}
	pod.Spec = mergedPodSpec
	podRes, err := s.cli.CoreV1().Pods(GetPodNamespace()).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	err = s.podReady.waitForPodReady(ctx, GetPodNamespace(), podRes.Name)
	if err != nil {
		return nil, err
	}
	podRes, err = s.cli.CoreV1().Pods(GetPodNamespace()).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return podRes, nil
}

func (s *fioStepper) deletePod(ctx context.Context, podName string) error {
	return s.cli.CoreV1().Pods(GetPodNamespace()).Delete(ctx, podName, metav1.DeleteOptions{})
}

// GetPodNamespace gets the pods namespace or returns default
func GetPodNamespace() string {
	if val, ok := os.LookupEnv(PodNamespaceEnvKey); ok {
		return val
	}
	return DefaultNS
}

func fioTestFilename(configMap map[string]string) (string, error) {
	potentialFilenames := []string{}
	for key := range configMap {
		if key != ConfigMapSCKey && key != ConfigMapSizeKey {
			potentialFilenames = append(potentialFilenames, key)
		}
	}
	if len(potentialFilenames) != 1 {
		return "", fmt.Errorf("Unable to find fio file in configmap/more than one found %v", configMap)
	}
	return potentialFilenames[0], nil
}

type waitForPodReady interface {
	waitForPodReady(ctx context.Context, namespace string, name string) error
}

type podReadyChecker struct {
	cli kubernetes.Interface
}

func (p *podReadyChecker) waitForPodReady(ctx context.Context, namespace, name string) error {
	return kankube.WaitForPodReady(ctx, p.cli, namespace, name)
}

type podSpecMerge interface {
	mergePodSpec(ctx context.Context, namespace string, podSpec v1.PodSpec) (v1.PodSpec, error)
}

type podSpecMerger struct {
	cli kubernetes.Interface
}

func (m *podSpecMerger) mergePodSpec(ctx context.Context, namespace string, podSpec v1.PodSpec) (v1.PodSpec, error) {
	currentPodName := os.Getenv(PodNameEnvKey)
	if currentPodName == "" {
		return podSpec, fmt.Errorf("Unable to retrieve Pod name from environment variable (%s)", PodNameEnvKey)
	}
	currentPod, err := m.cli.CoreV1().Pods(namespace).Get(ctx, currentPodName, metav1.GetOptions{})
	if err != nil {
		return podSpec, fmt.Errorf("Failed to discover pod configuration for Pod (%s): (%s)\n", currentPodName, err.Error())
	}
	if len(podSpec.Containers) != 1 {
		return podSpec, fmt.Errorf("FIO pod doesn't have exactly 1 container.")
	}
	podSpec.NodeSelector = currentPod.Spec.NodeSelector
	podSpec.Tolerations = currentPod.Spec.Tolerations
	podSpec.Containers[0].Image = currentPod.Spec.Containers[0].Image
	podSpec.SecurityContext = currentPod.Spec.SecurityContext
	return podSpec, nil
}
