package fio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/kastenhq/kubestr/pkg/common"
	"github.com/pkg/errors"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	sv1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func Test(t *testing.T) { TestingT(t) }

type FIOTestSuite struct{}

var _ = Suite(&FIOTestSuite{})

func (s *FIOTestSuite) TestRunner(c *C) {
	ctx := context.Background()
	runner := &FIOrunner{
		Cli: nil,
	}
	_, err := runner.RunFio(ctx, nil)
	c.Check(err, NotNil)
}

func (s *FIOTestSuite) TestRunFioHelper(c *C) {
	ctx := context.Background()
	for i, tc := range []struct {
		cli           kubernetes.Interface
		stepper       *fakeFioStepper
		args          *RunFIOArgs
		expectedSteps []string
		checker       Checker
		expectedCM    string
		expectedSC    string
		expectedSize  string
		expectedTFN   string
		expectedPVC   string
	}{
		{ // invalid args (storageclass)
			cli:     fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{},
			args:    &RunFIOArgs{},
			checker: NotNil,
		},
		{ // invalid args (size)
			cli:     fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{},
			args: &RunFIOArgs{
				StorageClass: "sc",
			},
			checker: NotNil,
		},
		{ // invalid args (namespace)
			cli:     fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
			},
			checker: NotNil,
		},
		{ // namespace doesn't exist
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				vnErr: fmt.Errorf("namespace Err"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN"},
		},
		{ // node name doesn't exist and is not empty
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				vnoErr: fmt.Errorf("node Err"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO"},
		},
		{ // storageclass not found
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				sceErr: fmt.Errorf("storageclass Err"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE"},
		},
		{ // success
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
					},
				},
				cPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "PVC",
					},
				},
				cPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Pod",
					},
				},
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       IsNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM", "CPVC", "CPOD", "RFIOC", "DPOD", "DPVC", "DCM"},
			expectedSC:    "sc",
			expectedSize:  DefaultPVCSize,
			expectedTFN:   "testfile.fio",
			expectedCM:    "CM1",
			expectedPVC:   "PVC",
		},
		{ // fio test error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
					},
				},
				cPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "PVC",
					},
				},
				cPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Pod",
					},
				},
				rFIOErr: fmt.Errorf("run fio error"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM", "CPVC", "CPOD", "RFIOC", "DPOD", "DPVC", "DCM"},
		},
		{ // create pod error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
					},
				},
				cPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "PVC",
					},
				},
				cPod: &v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "Pod",
					},
				},
				cPodErr: fmt.Errorf("pod create error"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM", "CPVC", "CPOD", "DPVC", "DCM"},
		},
		{ // create PVC error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
					},
				},
				cPVCErr: fmt.Errorf("pvc create error"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM", "CPVC", "DCM"},
		},
		{ // testfilename retrieval error, more than one provided
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":  "testfiledata",
						"testfile.fio2": "testfiledata",
					},
				},
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM", "DCM"},
		},
		{ // load configmap error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmErr: fmt.Errorf("failed to load configmap"),
			},
			args: &RunFIOArgs{
				StorageClass: "sc",
				Size:         "100Gi",
				Namespace:    "foo",
			},
			checker:       NotNil,
			expectedSteps: []string{"VN", "VNO", "SCE", "LCM"},
		},
	} {
		c.Log(i)
		fio := &FIOrunner{
			Cli:      tc.cli,
			fioSteps: tc.stepper,
		}
		_, err := fio.RunFioHelper(ctx, tc.args)
		c.Check(err, tc.checker)
		c.Assert(tc.stepper.steps, DeepEquals, tc.expectedSteps)
		if err == nil {
			c.Assert(tc.expectedSC, Equals, tc.stepper.cPVCExpSC)
			c.Assert(tc.expectedSize, Equals, tc.stepper.cPVCExpSize)
			c.Assert(tc.expectedTFN, Equals, tc.stepper.cPodExpFN)
			c.Assert(tc.expectedCM, Equals, tc.stepper.cPodExpCM)
			c.Assert(tc.expectedPVC, Equals, tc.stepper.cPodExpPVC)
		}
	}
}

type fakeFioStepper struct {
	steps []string

	vnErr error

	vnoErr error

	sceSC  *sv1.StorageClass
	sceErr error

	lcmConfigMap *v1.ConfigMap
	lcmErr       error

	cPVCExpSC   string
	cPVCExpSize string
	cPVC        *v1.PersistentVolumeClaim
	cPVCErr     error

	dPVCErr error

	cPodExpFN  string
	cPodExpCM  string
	cPodExpPVC string
	cPod       *v1.Pod
	cPodErr    error

	dPodErr error

	rFIOout FioResult
	rFIOErr error
}

func (f *fakeFioStepper) validateNamespace(ctx context.Context, namespace string) error {
	f.steps = append(f.steps, "VN")
	return f.vnErr
}
func (f *fakeFioStepper) validateNode(ctx context.Context, node string) error {
	f.steps = append(f.steps, "VNO")
	return f.vnoErr
}
func (f *fakeFioStepper) storageClassExists(ctx context.Context, storageClass string) (*sv1.StorageClass, error) {
	f.steps = append(f.steps, "SCE")
	return f.sceSC, f.sceErr
}
func (f *fakeFioStepper) loadConfigMap(ctx context.Context, args *RunFIOArgs) (*v1.ConfigMap, error) {
	f.steps = append(f.steps, "LCM")
	return f.lcmConfigMap, f.lcmErr
}
func (f *fakeFioStepper) createPVC(ctx context.Context, storageclass, size, namespace string) (*v1.PersistentVolumeClaim, error) {
	f.steps = append(f.steps, "CPVC")
	f.cPVCExpSC = storageclass
	f.cPVCExpSize = size
	return f.cPVC, f.cPVCErr
}
func (f *fakeFioStepper) deletePVC(ctx context.Context, pvcName, namespace string) error {
	f.steps = append(f.steps, "DPVC")
	return f.dPVCErr
}
func (f *fakeFioStepper) createPod(ctx context.Context, pvcName, configMapName, testFileName, namespace string, image string, node string) (*v1.Pod, error) {
	f.steps = append(f.steps, "CPOD")
	f.cPodExpCM = configMapName
	f.cPodExpFN = testFileName
	f.cPodExpPVC = pvcName
	return f.cPod, f.cPodErr
}
func (f *fakeFioStepper) deletePod(ctx context.Context, podName, namespace string) error {
	f.steps = append(f.steps, "DPOD")
	return f.dPodErr
}
func (f *fakeFioStepper) runFIOCommand(ctx context.Context, podName, containerName, testFileName, namespace string) (FioResult, error) {
	f.steps = append(f.steps, "RFIOC")
	return f.rFIOout, f.rFIOErr
}
func (f *fakeFioStepper) deleteConfigMap(ctx context.Context, configMap *v1.ConfigMap, namespace string) error {
	f.steps = append(f.steps, "DCM")
	return nil
}

func (s *FIOTestSuite) TestStorageClassExists(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli          kubernetes.Interface
		storageClass string
		checker      Checker
	}{
		{
			cli:          fake.NewSimpleClientset(),
			storageClass: "sc",
			checker:      NotNil,
		},
		{
			cli:          fake.NewSimpleClientset(&scv1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc"}}),
			storageClass: "sc",
			checker:      IsNil,
		},
	} {
		stepper := &fioStepper{cli: tc.cli}
		_, err := stepper.storageClassExists(ctx, tc.storageClass)
		c.Check(err, tc.checker)
	}
}

func (s *FIOTestSuite) TestValidateNamespace(c *C) {
	ctx := context.Background()
	stepper := &fioStepper{cli: fake.NewSimpleClientset()}
	err := stepper.validateNamespace(ctx, "ns")
	c.Assert(err, NotNil)
	stepper = &fioStepper{cli: fake.NewSimpleClientset(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ns",
		},
	})}
	err = stepper.validateNamespace(ctx, "ns")
	c.Assert(err, IsNil)
}

func (s *FIOTestSuite) TestValidateNode(c *C) {
	ctx := context.Background()
	stepper := &fioStepper{cli: fake.NewSimpleClientset()}
	err := stepper.validateNode(ctx, "")
	c.Assert(err, IsNil)
	err = stepper.validateNode(ctx, "node")
	c.Assert(err, NotNil)
	stepper = &fioStepper{cli: fake.NewSimpleClientset(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node",
		},
	})}
	err = stepper.validateNode(ctx, "node")
	c.Assert(err, IsNil)
}

func (s *FIOTestSuite) TestLoadConfigMap(c *C) {
	ctx := context.Background()
	file, err := ioutil.TempFile("", "tempTLCfile")
	c.Check(err, IsNil)
	defer os.Remove(file.Name())
	for i, tc := range []struct {
		cli           kubernetes.Interface
		configMapName string
		jobName       string
		args          *RunFIOArgs
		cmChecker     Checker
		errChecker    Checker
		failCreates   bool
		hasLabel      bool
	}{
		{ // provided file name not found
			cli: fake.NewSimpleClientset(),
			args: &RunFIOArgs{
				FIOJobFilepath: "nonexistantfile",
			},
			cmChecker:  IsNil,
			errChecker: NotNil,
		},
		{ // specified config map found
			cli: fake.NewSimpleClientset(),
			args: &RunFIOArgs{
				FIOJobFilepath: file.Name(),
				FIOJobName:     "random", // won't use this case
			},
			cmChecker:  NotNil,
			errChecker: IsNil,
		},
		{ // specified job name, not found
			cli: fake.NewSimpleClientset(),
			args: &RunFIOArgs{
				FIOJobName: "random",
			},
			cmChecker:  IsNil,
			errChecker: NotNil,
		},
		{ // specified job name, found
			cli: fake.NewSimpleClientset(),
			args: &RunFIOArgs{
				FIOJobName: DefaultFIOJob,
			},
			cmChecker:  NotNil,
			errChecker: IsNil,
		},
		{ // use default job
			cli:        fake.NewSimpleClientset(),
			args:       &RunFIOArgs{},
			cmChecker:  NotNil,
			errChecker: IsNil,
		},
		{ // Fails to create configMap
			cli:         fake.NewSimpleClientset(),
			cmChecker:   IsNil,
			errChecker:  NotNil,
			args:        &RunFIOArgs{},
			failCreates: true,
		},
	} {
		c.Log(i)
		stepper := &fioStepper{cli: tc.cli}
		if tc.failCreates {
			stepper.cli.(*fake.Clientset).Fake.PrependReactor("create", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		cm, err := stepper.loadConfigMap(ctx, tc.args)
		c.Check(err, tc.errChecker)
		c.Check(cm, tc.cmChecker)
		if cm != nil {
			_, ok := cm.Labels[CreatedByFIOLabel]
			c.Assert(ok, Equals, true)
		}
	}
}

func (s *FIOTestSuite) TestCreatePVC(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli          kubernetes.Interface
		storageclass string
		size         string
		errChecker   Checker
		pvcChecker   Checker
		failCreates  bool
	}{
		{
			cli:          fake.NewSimpleClientset(),
			storageclass: "fakesc",
			size:         "20Gi",
			errChecker:   IsNil,
			pvcChecker:   NotNil,
		},
		{ // Fails to create pvc
			cli:          fake.NewSimpleClientset(),
			storageclass: "fakesc",
			size:         "10Gi",
			pvcChecker:   IsNil,
			errChecker:   NotNil,
			failCreates:  true,
		},
		{ // parse error
			cli:          fake.NewSimpleClientset(),
			storageclass: "fakesc",
			size:         "Not a quantity",
			pvcChecker:   IsNil,
			errChecker:   NotNil,
		},
	} {
		stepper := &fioStepper{cli: tc.cli}
		if tc.failCreates {
			stepper.cli.(*fake.Clientset).Fake.PrependReactor("create", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		pvc, err := stepper.createPVC(ctx, tc.storageclass, tc.size, DefaultNS)
		c.Check(err, tc.errChecker)
		c.Check(pvc, tc.pvcChecker)
		if pvc != nil {
			c.Assert(pvc.GenerateName, Equals, PVCGenerateName)
			c.Assert(*pvc.Spec.StorageClassName, Equals, tc.storageclass)
			value, ok := pvc.Spec.Resources.Requests.Storage().AsInt64()
			c.Assert(ok, Equals, true)
			c.Assert(value, Equals, int64(21474836480))
		}
	}
}

func (s *FIOTestSuite) TestDeletePVC(c *C) {
	ctx := context.Background()
	stepper := &fioStepper{cli: fake.NewSimpleClientset(&v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc",
			Namespace: DefaultNS,
		}})}
	err := stepper.deletePVC(ctx, "pvc", DefaultNS)
	c.Assert(err, IsNil)
	err = stepper.deletePVC(ctx, "pvc", DefaultNS)
	c.Assert(err, NotNil)
}

func (s *FIOTestSuite) TestCreatPod(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		pvcName       string
		configMapName string
		testFileName  string
		image         string
		node          string
		reactor       []k8stesting.Reactor
		podReadyErr   error
		errChecker    Checker
	}{
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "testfile",
			errChecker:    IsNil,
		},
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "testfile",
			errChecker:    NotNil,
			reactor: []k8stesting.Reactor{
				&k8stesting.SimpleReactor{
					Verb:     "create",
					Resource: "*",
					Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}, nil
					},
				},
				&k8stesting.SimpleReactor{
					Verb:     "get",
					Resource: "*",
					Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("Error getting object")
					},
				},
			},
		},
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "testfile",
			errChecker:    NotNil,
			reactor: []k8stesting.Reactor{
				&k8stesting.SimpleReactor{
					Verb:     "create",
					Resource: "*",
					Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}, nil
					},
				},
			},
			podReadyErr: fmt.Errorf("pod ready error"),
		},
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "testfile",
			errChecker:    NotNil,
			reactor: []k8stesting.Reactor{
				&k8stesting.SimpleReactor{
					Verb:     "create",
					Resource: "*",
					Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, fmt.Errorf("pod create error")
					},
				},
			},
		},
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "",
			image:         "someotherimage",
			errChecker:    NotNil,
		},
		{
			pvcName:       "",
			configMapName: "cm",
			testFileName:  "asdf",
			errChecker:    NotNil,
		},
		{
			pvcName:       "pvc",
			configMapName: "",
			testFileName:  "asd",
			errChecker:    NotNil,
		},
	} {
		stepper := &fioStepper{
			cli:      fake.NewSimpleClientset(),
			podReady: &fakePodReadyChecker{prcErr: tc.podReadyErr},
		}
		if tc.reactor != nil {
			stepper.cli.(*fake.Clientset).Fake.ReactionChain = tc.reactor
		}
		pod, err := stepper.createPod(ctx, tc.pvcName, tc.configMapName, tc.testFileName, DefaultNS, tc.image, tc.node)
		c.Check(err, tc.errChecker)
		if err == nil {
			c.Assert(pod.GenerateName, Equals, PodGenerateName)
			c.Assert(len(pod.Spec.Volumes), Equals, 2)
			for _, vol := range pod.Spec.Volumes {
				switch vol.Name {
				case "persistent-storage":
					c.Assert(vol.VolumeSource.PersistentVolumeClaim.ClaimName, Equals, tc.pvcName)
				case "config-map":
					c.Assert(vol.VolumeSource.ConfigMap.Name, Equals, tc.configMapName)
				}
			}
			c.Assert(len(pod.Spec.Containers), Equals, 1)
			c.Assert(pod.Spec.Containers[0].Name, Equals, ContainerName)
			c.Assert(pod.Spec.Containers[0].Command, DeepEquals, []string{"/bin/sh"})
			c.Assert(pod.Spec.Containers[0].Args, DeepEquals, []string{"-c", "tail -f /dev/null"})
			c.Assert(pod.Spec.Containers[0].VolumeMounts, DeepEquals, []v1.VolumeMount{
				{Name: "persistent-storage", MountPath: VolumeMountPath},
				{Name: "config-map", MountPath: ConfigMapMountPath},
			})
			if tc.image == "" {
				c.Assert(pod.Spec.Containers[0].Image, Equals, common.DefaultPodImage)
			} else {
				c.Assert(pod.Spec.Containers[0].Image, Equals, tc.image)
			}
		}
	}
}

func (s *FIOTestSuite) TestDeletePod(c *C) {
	ctx := context.Background()
	stepper := &fioStepper{cli: fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: DefaultNS,
		}})}
	err := stepper.deletePod(ctx, "pod", DefaultNS)
	c.Assert(err, IsNil)
	err = stepper.deletePod(ctx, "pod", DefaultNS)
	c.Assert(err, NotNil)
}

func (s *FIOTestSuite) TestFioTestFileName(c *C) {
	for _, tc := range []struct {
		configMap  map[string]string
		retVal     string
		errChecker Checker
	}{
		{
			configMap: map[string]string{
				"testfile.fio": "some test data",
			},
			retVal:     "testfile.fio",
			errChecker: IsNil,
		},
		{
			configMap: map[string]string{
				"ConfigMapSCKey":   "storageclass",
				"ConfigMapSizeKey": "10Gi",
				"testfile.fio":     "some test data",
			},
			retVal:     "",
			errChecker: NotNil,
		},
	} {
		ret, err := fioTestFilename(tc.configMap)
		c.Check(err, tc.errChecker)
		c.Assert(ret, Equals, tc.retVal)
	}
}

func (s *FIOTestSuite) TestRunFioCommand(c *C) {
	var parsedout FioResult
	err := json.Unmarshal([]byte(parsableFioOutput), &parsedout)
	c.Assert(err, IsNil)

	ctx := context.Background()
	for _, tc := range []struct {
		executor      *fakeKubeExecutor
		errChecker    Checker
		podName       string
		containerName string
		testFileName  string
		out           FioResult
	}{
		{
			executor: &fakeKubeExecutor{
				keErr:    nil,
				keStrErr: "",
				keStdOut: parsableFioOutput,
			},
			errChecker:    IsNil,
			podName:       "pod",
			containerName: "container",
			testFileName:  "tfName",
			out:           parsedout,
		},
		{
			executor: &fakeKubeExecutor{
				keErr:    nil,
				keStrErr: "",
				keStdOut: "unparsable string",
			},
			errChecker:    NotNil,
			podName:       "pod",
			containerName: "container",
			testFileName:  "tfName",
			out:           FioResult{},
		},
		{
			executor: &fakeKubeExecutor{
				keErr:    fmt.Errorf("kubeexec err"),
				keStrErr: "",
				keStdOut: "unparsable string",
			},
			errChecker:    NotNil,
			podName:       "pod",
			containerName: "container",
			testFileName:  "tfName",
			out:           FioResult{},
		},
		{
			executor: &fakeKubeExecutor{
				keErr:    nil,
				keStrErr: "execution error",
				keStdOut: "unparsable string",
			},
			errChecker:    NotNil,
			podName:       "pod",
			containerName: "container",
			testFileName:  "tfName",
			out:           FioResult{},
		},
	} {
		stepper := &fioStepper{
			kubeExecutor: tc.executor,
		}
		out, err := stepper.runFIOCommand(ctx, tc.podName, tc.containerName, tc.testFileName, DefaultNS)
		c.Check(err, tc.errChecker)
		c.Assert(out, DeepEquals, tc.out)
		c.Assert(tc.executor.keInPodName, Equals, tc.podName)
		c.Assert(tc.executor.keInContainerName, Equals, tc.containerName)
		c.Assert(len(tc.executor.keInCommand), Equals, 5)
		c.Assert(tc.executor.keInCommand[0], Equals, "fio")
		c.Assert(tc.executor.keInCommand[1], Equals, "--directory")
		c.Assert(tc.executor.keInCommand[2], Equals, VolumeMountPath)
		jobFilePath := fmt.Sprintf("%s/%s", ConfigMapMountPath, tc.testFileName)
		c.Assert(tc.executor.keInCommand[3], Equals, jobFilePath)
	}
}

func (s *FIOTestSuite) TestDeleteConfigMap(c *C) {
	ctx := context.Background()
	defaultNS := "default"
	os.Setenv(PodNamespaceEnvKey, defaultNS)
	for _, tc := range []struct {
		cli        kubernetes.Interface
		cm         *v1.ConfigMap
		errChecker Checker
		lenCMList  int
	}{
		{ // Don't delete it unless it has the label
			cli: fake.NewSimpleClientset(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: defaultNS,
				},
			}),
			cm: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: defaultNS,
				},
			},
			errChecker: IsNil,
			lenCMList:  1,
		},
		{ // Has label delete
			cli: fake.NewSimpleClientset(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: defaultNS,
				},
			}),
			cm: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: defaultNS,
					Labels: map[string]string{
						CreatedByFIOLabel: "true",
					},
				},
			},
			errChecker: IsNil,
			lenCMList:  0,
		},
		{ // No cm exists
			cli: fake.NewSimpleClientset(),
			cm: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm",
					Namespace: defaultNS,
					Labels: map[string]string{
						CreatedByFIOLabel: "true",
					},
				},
			},
			errChecker: NotNil,
		},
	} {
		stepper := &fioStepper{cli: tc.cli}
		err := stepper.deleteConfigMap(ctx, tc.cm, DefaultNS)
		c.Check(err, tc.errChecker)
		if err == nil {
			list, err := stepper.cli.CoreV1().ConfigMaps(defaultNS).List(ctx, metav1.ListOptions{})
			c.Check(err, IsNil)
			c.Assert(len(list.Items), Equals, tc.lenCMList)
		}
	}
	os.Unsetenv(PodNamespaceEnvKey)
}

func (s *FIOTestSuite) TestWaitForPodReady(c *C) {
	ctx := context.Background()
	prChecker := &podReadyChecker{
		cli: fake.NewSimpleClientset(),
	}
	err := prChecker.waitForPodReady(ctx, "somens", "somePod")
	c.Check(err, NotNil)
	prChecker.cli = fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somePod",
			Namespace: "somens",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	})
}

type fakePodReadyChecker struct {
	prcErr error
}

func (f *fakePodReadyChecker) waitForPodReady(ctx context.Context, namespace, name string) error {
	return f.prcErr
}

type fakeKubeExecutor struct {
	keErr             error
	keStdOut          string
	keStrErr          string
	keInNS            string
	keInPodName       string
	keInContainerName string
	keInCommand       []string
}

func (fk *fakeKubeExecutor) exec(namespace, podName, containerName string, command []string) (string, string, error) {
	fk.keInNS = namespace
	fk.keInPodName = podName
	fk.keInContainerName = containerName
	fk.keInCommand = command
	return fk.keStdOut, fk.keStrErr, fk.keErr
}
