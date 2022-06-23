package fio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	kankube "github.com/kanisterio/kanister/pkg/kube"
	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
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
	// KubestrFIOJobGenName describes the generate name
	KubestrFIOJobGenName = "kubestr-fio"
	// ConfigMapJobKey is the default fio job key
	ConfigMapJobKey = "fiojob"
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
	// ConfigMapMountPath is the path where we mount the configmap
	ConfigMapMountPath = "/etc/fio-config"
	// VolumeMountPath is the path where we mount the volume
	VolumeMountPath = "/dataset"
	// CreatedByFIOLabel is the key that desrcibes the label used to mark configmaps
	CreatedByFIOLabel = "createdbyfio"
)

// NodeAffinityRegexp is used to validate node affinity arguments
var NodeAffinityRegexp = regexp.MustCompile(`^(?P<Label>!?[^=><!]+)(?P<Values>(?:=|!=|>|<).+)?$`)

// TolerationRegexp is used to validate toleration arguments
var TolerationRegexp = regexp.MustCompile(`^(?P<Key>[^=:]+)(?P<Value>=[^:]+)?(?P<Effect>:(?:NoSchedule|NoExecute|PreferNoSchedule))?$`)

// FIO is an interface that represents FIO related commands
type FIO interface {
	RunFio(ctx context.Context, args *RunFIOArgs) (*RunFIOResult, error) // , test config
}

// FIOrunner implments FIO
type FIOrunner struct {
	Cli      kubernetes.Interface
	fioSteps fioSteps
}

type RunFIOArgs struct {
	StorageClass   string
	Size           string
	Namespace      string
	FIOJobFilepath string
	FIOJobName     string
	Image          string
	NodeAffinities []string
	Tolerations    []string
}

func (a *RunFIOArgs) Validate() error {
	if a.StorageClass == "" || a.Size == "" || a.Namespace == "" {
		return fmt.Errorf("Require fields are missing. (StorageClass, Size, Namespace)")
	}
	return nil
}

type RunFIOResult struct {
	Size         string            `json:"size,omitempty"`
	StorageClass *sv1.StorageClass `json:"storageClass,omitempty"`
	FioConfig    string            `json:"fioConfig,omitempty"`
	Result       FioResult         `json:"result,omitempty"`
}

func (f *FIOrunner) RunFio(ctx context.Context, args *RunFIOArgs) (*RunFIOResult, error) {
	f.fioSteps = &fioStepper{
		cli:          f.Cli,
		podReady:     &podReadyChecker{cli: f.Cli},
		kubeExecutor: &kubeExecutor{cli: f.Cli},
	}
	return f.RunFioHelper(ctx, args)

}

func (f *FIOrunner) RunFioHelper(ctx context.Context, args *RunFIOArgs) (*RunFIOResult, error) {
	// create a configmap with test parameters
	if f.Cli == nil { // for UT purposes
		return nil, fmt.Errorf("cli uninitialized")
	}

	if err := args.Validate(); err != nil {
		return nil, err
	}

	if err := f.fioSteps.validateNamespace(ctx, args.Namespace); err != nil {
		return nil, errors.Wrapf(err, "Unable to find namespace (%s)", args.Namespace)
	}

	if err := f.fioSteps.validateNodeAffinities(ctx, args.NodeAffinities); err != nil {
		return nil, errors.Wrapf(err, "Unable to validate node affinities (%s)", args.NodeAffinities)
	}

	if err := f.fioSteps.validateTolerations(ctx, args.Tolerations); err != nil {
		return nil, errors.Wrapf(err, "Unable to validate tolerations (%s)", args.Tolerations)
	}

	sc, err := f.fioSteps.storageClassExists(ctx, args.StorageClass)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot find StorageClass")
	}

	configMap, err := f.fioSteps.loadConfigMap(ctx, args)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create a ConfigMap")
	}
	defer func() {
		_ = f.fioSteps.deleteConfigMap(context.TODO(), configMap, args.Namespace)
	}()

	testFileName, err := fioTestFilename(configMap.Data)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get test file name.")
	}

	pvc, err := f.fioSteps.createPVC(ctx, args.StorageClass, args.Size, args.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create PVC")
	}
	defer func() {
		_ = f.fioSteps.deletePVC(context.TODO(), pvc.Name, args.Namespace)
	}()
	fmt.Println("PVC created", pvc.Name)

	pod, err := f.fioSteps.createPod(ctx, pvc.Name, configMap.Name, testFileName, args.Namespace, args.Image, args.NodeAffinities, args.Tolerations)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create POD")
	}
	defer func() {
		_ = f.fioSteps.deletePod(context.TODO(), pod.Name, args.Namespace)
	}()
	fmt.Println("Pod created", pod.Name)
	fmt.Printf("Running FIO test (%s) on StorageClass (%s) with a PVC of Size (%s)\n", testFileName, args.StorageClass, args.Size)
	fioOutput, err := f.fioSteps.runFIOCommand(ctx, pod.Name, ContainerName, testFileName, args.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed while running FIO test.")
	}
	return &RunFIOResult{
		Size:         args.Size,
		StorageClass: sc,
		FioConfig:    configMap.Data[testFileName],
		Result:       fioOutput,
	}, nil
}

type fioSteps interface {
	validateNamespace(ctx context.Context, namespace string) error
	validateNodeAffinities(ctx context.Context, nodeAffinities []string) error
	validateTolerations(ctx context.Context, tolerations []string) error
	storageClassExists(ctx context.Context, storageClass string) (*sv1.StorageClass, error)
	loadConfigMap(ctx context.Context, args *RunFIOArgs) (*v1.ConfigMap, error)
	createPVC(ctx context.Context, storageclass, size, namespace string) (*v1.PersistentVolumeClaim, error)
	deletePVC(ctx context.Context, pvcName, namespace string) error
	createPod(ctx context.Context, pvcName, configMapName, testFileName, namespace string, image string, nodeAffinities []string, tolerations []string) (*v1.Pod, error)
	deletePod(ctx context.Context, podName, namespace string) error
	runFIOCommand(ctx context.Context, podName, containerName, testFileName, namespace string) (FioResult, error)
	deleteConfigMap(ctx context.Context, configMap *v1.ConfigMap, namespace string) error
}

type fioStepper struct {
	cli          kubernetes.Interface
	podReady     waitForPodReadyInterface
	kubeExecutor kubeExecInterface
}

func (s *fioStepper) validateNamespace(ctx context.Context, namespace string) error {
	if _, err := s.cli.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}

func (s *fioStepper) validateNodeAffinities(ctx context.Context, nodeAffinities []string) error {
	if (nodeAffinities == nil) || (len(nodeAffinities) == 0) {
		return nil
	}

	for _, nodeAffinity := range nodeAffinities {
		if !NodeAffinityRegexp.MatchString(nodeAffinity) {
			return errors.New(fmt.Sprintf("Node affinity %v doesn't match pattern '[!]label[(=|!|!=|>|<)value1,value2]'", nodeAffinity))
		}
	}

	return nil
}

func (s *fioStepper) validateTolerations(ctx context.Context, tolerations []string) error {
	if (tolerations == nil) || (len(tolerations) == 0) {
		return nil
	}

	for _, toleration := range tolerations {
		if !TolerationRegexp.MatchString(toleration) {
			return errors.New(fmt.Sprintf("Toleration %v doesn't match pattern 'key[=value][:effect]'", toleration))
		}
	}

	return nil
}

func (s *fioStepper) storageClassExists(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	return s.cli.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
}

func (s *fioStepper) loadConfigMap(ctx context.Context, args *RunFIOArgs) (*v1.ConfigMap, error) {
	configMap := &v1.ConfigMap{
		Data: make(map[string]string),
	}
	switch {
	case args.FIOJobFilepath != "":
		data, err := ioutil.ReadFile(args.FIOJobFilepath)
		if err != nil {
			return nil, errors.Wrap(err, "File reading error")
		}
		configMap.Data[filepath.Base(args.FIOJobFilepath)] = string(data)
	case args.FIOJobName != "":
		if _, ok := fioJobs[args.FIOJobName]; !ok {
			return nil, fmt.Errorf("FIO job not found- (%s)", args.FIOJobName)
		}
		configMap.Data[args.FIOJobName] = fioJobs[args.FIOJobName]
	default:
		configMap.Data[DefaultFIOJob] = fioJobs[DefaultFIOJob]
	}
	// create
	configMap.GenerateName = KubestrFIOJobGenName
	configMap.Labels = map[string]string{CreatedByFIOLabel: "true"}
	return s.cli.CoreV1().ConfigMaps(args.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
}

func (s *fioStepper) createPVC(ctx context.Context, storageclass, size, namespace string) (*v1.PersistentVolumeClaim, error) {
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
	return s.cli.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func (s *fioStepper) deletePVC(ctx context.Context, pvcName, namespace string) error {
	return s.cli.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
}

func getRegexpGroups(matcher *regexp.Regexp, input string) map[string]string {
	matches := matcher.FindStringSubmatch(input)

	result := make(map[string]string)
	for i, name := range matcher.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = matches[i]
		}
	}

	return result
}

func createNodeSelectorRequirement(nodeAffinity string) v1.NodeSelectorRequirement {
	affinityMetadata := getRegexpGroups(NodeAffinityRegexp, nodeAffinity)

	requirement := &v1.NodeSelectorRequirement{}

	if val, ok := affinityMetadata["Label"]; ok && len(val) > 0 {
		if strings.HasPrefix(val, "!") {
			requirement.Operator = v1.NodeSelectorOpDoesNotExist
			requirement.Key = val[1:]
		} else {
			requirement.Operator = v1.NodeSelectorOpExists
			requirement.Key = val
		}
	}

	if val, ok := affinityMetadata["Values"]; ok && len(val) > 0 {
		if strings.HasPrefix(val, "!=") {
			requirement.Operator = v1.NodeSelectorOpNotIn
			requirement.Values = strings.Split(val[2:], ",")
		} else if strings.HasPrefix(val, ">") {
			requirement.Operator = v1.NodeSelectorOpGt
			requirement.Values = []string{val[1:]}
		} else if strings.HasPrefix(val, "<") {
			requirement.Operator = v1.NodeSelectorOpLt
			requirement.Values = []string{val[1:]}
		} else {
			requirement.Operator = v1.NodeSelectorOpIn
			requirement.Values = strings.Split(val[1:], ",")
		}
	}

	return *requirement
}

func createNodeMatchExpression(nodeAffinities []string) []v1.NodeSelectorRequirement {
	requirements := make([]v1.NodeSelectorRequirement, len(nodeAffinities))
	for index, affinity := range nodeAffinities {
		requirements[index] = createNodeSelectorRequirement(affinity)
	}
	return requirements
}

func createNodeSelectorTerm(nodeAffinities []string) v1.NodeSelectorTerm {
	return v1.NodeSelectorTerm{
		MatchExpressions: createNodeMatchExpression(nodeAffinities),
	}
}

func createToleration(toleration string) v1.Toleration {
	tolerationMetadata := getRegexpGroups(TolerationRegexp, toleration)

	podToleration := &v1.Toleration{}
	podToleration.Key = tolerationMetadata["Key"]

	if val, ok := tolerationMetadata["Effect"]; ok && len(val) > 0 {
		podToleration.Effect = v1.TaintEffect(val[1:])
	}

	if val, ok := tolerationMetadata["Value"]; ok && len(val) > 0 {
		podToleration.Operator = v1.TolerationOpEqual
		podToleration.Value = val[1:]
	} else {
		podToleration.Operator = v1.TolerationOpExists
	}

	return *podToleration
}

func createTolerations(tolerations []string) []v1.Toleration {
	podTolerations := make([]v1.Toleration, len(tolerations))
	for index, toleration := range tolerations {
		podTolerations[index] = createToleration(toleration)
	}
	return podTolerations
}

func (s *fioStepper) createPod(ctx context.Context, pvcName, configMapName, testFileName, namespace string, image string, nodeAffinities []string, tolerations []string) (*v1.Pod, error) {
	if pvcName == "" || configMapName == "" || testFileName == "" {
		return nil, fmt.Errorf("Create pod missing required arguments.")
	}

	if image == "" {
		image = common.DefaultPodImage
	}

	podSpec := &v1.PodSpec{
		Containers: []v1.Container{{
			Name:    ContainerName,
			Command: []string{"/bin/sh"},
			Args:    []string{"-c", "tail -f /dev/null"},
			VolumeMounts: []v1.VolumeMount{
				{Name: "persistent-storage", MountPath: VolumeMountPath},
				{Name: "config-map", MountPath: ConfigMapMountPath},
			},
			Image: image,
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
	}

	if len(nodeAffinities) > 0 {
		podSpec.Affinity = &v1.Affinity{
			NodeAffinity: &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{createNodeSelectorTerm(nodeAffinities)},
				},
			},
		}
	}

	if len(tolerations) > 0 {
		podSpec.Tolerations = createTolerations(tolerations)
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: PodGenerateName,
			Namespace:    namespace,
		},
		Spec: *podSpec,
	}
	podRes, err := s.cli.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return podRes, err
	}

	err = s.podReady.waitForPodReady(ctx, namespace, podRes.Name)
	if err != nil {
		return nil, err
	}

	podRes, err = s.cli.CoreV1().Pods(namespace).Get(ctx, podRes.Name, metav1.GetOptions{})
	if err != nil {
		return podRes, err
	}

	return podRes, nil
}

func (s *fioStepper) deletePod(ctx context.Context, podName, namespace string) error {
	return s.cli.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (s *fioStepper) runFIOCommand(ctx context.Context, podName, containerName, testFileName, namespace string) (FioResult, error) {
	jobFilePath := fmt.Sprintf("%s/%s", ConfigMapMountPath, testFileName)
	command := []string{"fio", "--directory", VolumeMountPath, jobFilePath, "--output-format=json"}
	done := make(chan bool, 1)
	var fioOut FioResult
	var stdout string
	var stderr string
	var err error
	timestart := time.Now()
	go func() {
		stdout, stderr, err = s.kubeExecutor.exec(namespace, podName, containerName, command)
		if err != nil || stderr != "" {
			if err == nil {
				err = fmt.Errorf("stderr when running FIO")
			}
			err = errors.Wrapf(err, "Error running command:(%v), stderr:(%s)", command, stderr)
		}
		done <- true
	}()
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	spin.Start()
	<-done
	spin.Stop()
	elapsed := time.Since(timestart)
	fmt.Println("Elapsed time-", elapsed)
	if err != nil {
		return fioOut, err
	}

	err = json.Unmarshal([]byte(stdout), &fioOut)
	if err != nil {
		return fioOut, errors.Wrapf(err, "Unable to parse fio output into json.")
	}

	return fioOut, nil
}

// deleteConfigMap only deletes a config map if it has the label
func (s *fioStepper) deleteConfigMap(ctx context.Context, configMap *v1.ConfigMap, namespace string) error {
	if val, ok := configMap.Labels[CreatedByFIOLabel]; ok && val == "true" {
		return s.cli.CoreV1().ConfigMaps(namespace).Delete(ctx, configMap.Name, metav1.DeleteOptions{})
	}
	return nil
}

func fioTestFilename(configMap map[string]string) (string, error) {
	if len(configMap) != 1 {
		return "", fmt.Errorf("Unable to find fio file in configmap/more than one found %v", configMap)
	}
	var fileName string
	for key := range configMap {
		fileName = key
	}
	return fileName, nil
}

type waitForPodReadyInterface interface {
	waitForPodReady(ctx context.Context, namespace string, name string) error
}

type podReadyChecker struct {
	cli kubernetes.Interface
}

func (p *podReadyChecker) waitForPodReady(ctx context.Context, namespace, name string) error {
	return kankube.WaitForPodReady(ctx, p.cli, namespace, name)
}

type kubeExecInterface interface {
	exec(namespace, podName, containerName string, command []string) (string, string, error)
}

type kubeExecutor struct {
	cli kubernetes.Interface
}

func (k *kubeExecutor) exec(namespace, podName, containerName string, command []string) (string, string, error) {
	return kankube.Exec(k.cli, namespace, podName, containerName, command, nil)
}
