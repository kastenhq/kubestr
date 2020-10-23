package fio

import (
	"context"
	"fmt"
	"os"

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
		cli: f.Cli,
	}
	return f.RunFioHelper(ctx, args)

}

func (f *FIOrunner) RunFioHelper(ctx context.Context, args *RunFIOArgs) (string, error) {
	// create a configmap with test parameters

	configMap, err := f.fioSteps.loadConfigMap(ctx, args.ConfigMapName, args.JobName)
	if err != nil {
		return "", errors.Wrap(err, "Unable to create a ConfigMap")
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

	// in memory configmap with size, storageclass and fio config.
	size := configMap.Data[ConfigMapSizeKey]
	if size == "" {
		size = DefaultPVCSize
	}
	// create a pvc
	pvc, err := f.fioSteps.createPVC(ctx, storageClass, size)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create PVC")
	}
	defer func() {
		_ = f.fioSteps.deletePVC(context.TODO(), pvc.Name)
	}()
	// // create a pod
	// if err := f.fioSteps.createPod(ctx); err != nil {
	// 	return "", errors.Wrap(err, "Failed to create POD")
	// }
	// defer func() {
	// 	_ = f.fioSteps.deletePod(context.TODO())
	// }()
	// if err := f.fioSteps.waitForPodReady(ctx); err != nil {
	// 	return "", errors.Wrap(err, "Pod failed to become ready")
	// }
	// // store fio result
	// if err := f.fioSteps.storeResult(ctx); err != nil {
	// 	return "", errors.Wrap(err, "Failed to store result")
	// }
	return "", nil
}

type fioSteps interface {
	storageClassExists(ctx context.Context, storageClass string) error
	loadConfigMap(ctx context.Context, configMapName string, jobName string) (*v1.ConfigMap, error)
	createPVC(ctx context.Context, storageclass, size string) (*v1.PersistentVolumeClaim, error)
	deletePVC(ctx context.Context, pvcName string) error
	// createPod(ctx context.Context) error
	// deletePod(ctx context.Context) error
	// waitForPodReady(ctx context.Context) error
	// storeResult(ctx context.Context) error
}

type fioStepper struct {
	cli kubernetes.Interface
}

func (s *fioStepper) storageClassExists(ctx context.Context, storageClass string) error {
	if _, err := s.cli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}

func (s *fioStepper) loadConfigMap(ctx context.Context, configMapName string, jobName string) (*v1.ConfigMap, error) {
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

// GetPodNamespace gets the pods namespace or returns default
func GetPodNamespace() string {
	if val, ok := os.LookupEnv(PodNamespaceEnvKey); ok {
		return val
	}
	return DefaultNS
}
