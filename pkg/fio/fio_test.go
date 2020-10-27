package fio

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/pkg/errors"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
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
	for _, tc := range []struct {
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
		{
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
				StorageClass:  "sc",
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       IsNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC", "CPOD", "DPOD", "DPVC"},
			expectedSC:    "sc",
			expectedSize:  DefaultPVCSize,
			expectedTFN:   "testfile.fio",
			expectedCM:    "CM1",
			expectedPVC:   "PVC",
		},
		{ // storage class provided by config map
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
						ConfigMapSCKey: "sc",
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
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       IsNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC", "CPOD", "DPOD", "DPVC"},
			expectedSC:    "sc",
			expectedSize:  DefaultPVCSize,
			expectedTFN:   "testfile.fio",
			expectedCM:    "CM1",
			expectedPVC:   "PVC",
		},
		{ // storage class provided by config map overided by args
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio": "testfiledata",
						ConfigMapSCKey: "SC2",
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
				ConfigMapName: "CM1",
				JobName:       "job",
				StorageClass:  "SC1", // use this one
			},
			checker:       IsNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC", "CPOD", "DPOD", "DPVC"},
			expectedSC:    "SC1",
			expectedSize:  DefaultPVCSize,
			expectedTFN:   "testfile.fio",
			expectedCM:    "CM1",
			expectedPVC:   "PVC",
		},
		{ // use size provided by Configmap
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						ConfigMapSCKey:   "SC2",
						ConfigMapSizeKey: "10Gi",
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
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       IsNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC", "CPOD", "DPOD", "DPVC"},
			expectedSC:    "SC2",
			expectedSize:  "10Gi",
			expectedTFN:   "testfile.fio",
			expectedCM:    "CM1",
			expectedPVC:   "PVC",
		},
		{ // create pod error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						ConfigMapSCKey:   "SC2",
						ConfigMapSizeKey: "10Gi",
					},
				},
				cPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "PVC",
					},
				},
				cPodErr: fmt.Errorf("pod create error"),
			},
			args: &RunFIOArgs{
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       NotNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC", "CPOD", "DPVC"},
		},
		{ // create PVC error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						ConfigMapSCKey:   "SC2",
						ConfigMapSizeKey: "10Gi",
					},
				},
				cPVCErr: fmt.Errorf("pvc create error"),
			},
			args: &RunFIOArgs{
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       NotNil,
			expectedSteps: []string{"LCM", "SCE", "CPVC"},
		},
		{ // storageclass not found
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						ConfigMapSCKey:   "SC2",
						ConfigMapSizeKey: "10Gi",
					},
				},
				sceErr: fmt.Errorf("storageclass not found error"),
			},
			args: &RunFIOArgs{
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       NotNil,
			expectedSteps: []string{"LCM", "SCE"},
		},
		{ // testfilename retrieval error, more than one provided
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						"testfile.fio2":  "testfiledata",
						ConfigMapSCKey:   "SC2",
						ConfigMapSizeKey: "10Gi",
					},
				},
			},
			args: &RunFIOArgs{
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       NotNil,
			expectedSteps: []string{"LCM"},
		},
		{ // storageclass not provided in args or configmap
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmConfigMap: &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "CM1",
					},
					Data: map[string]string{
						"testfile.fio":   "testfiledata",
						ConfigMapSizeKey: "10Gi",
					},
				},
				cPVC: &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "PVC",
					},
				},
			},
			args: &RunFIOArgs{
				ConfigMapName: "CM1",
				JobName:       "job",
			},
			checker:       NotNil,
			expectedSteps: []string{"LCM"},
		},
		{ // load configmap error
			cli: fake.NewSimpleClientset(),
			stepper: &fakeFioStepper{
				lcmErr: fmt.Errorf("failed to load configmap"),
			},
			args:          nil,
			checker:       NotNil,
			expectedSteps: []string{"LCM"},
		},
	} {
		fio := &FIOrunner{
			Cli:      tc.cli,
			fioSteps: tc.stepper,
		}
		_, err := fio.RunFioHelper(ctx, tc.args)
		c.Check(err, tc.checker)
		c.Assert(tc.stepper.steps, DeepEquals, tc.expectedSteps)
		if err == nil {
			c.Assert(tc.expectedSC, Equals, tc.stepper.sceExpSC)
			c.Assert(tc.expectedCM, Equals, tc.stepper.lcmExpCM)
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

	sceExpSC string
	sceErr   error

	lcmExpCM     string
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
}

func (f *fakeFioStepper) storageClassExists(ctx context.Context, storageClass string) error {
	f.steps = append(f.steps, "SCE")
	f.sceExpSC = storageClass
	return f.sceErr
}
func (f *fakeFioStepper) loadConfigMap(ctx context.Context, configMapName, jobName string) (*v1.ConfigMap, error) {
	f.steps = append(f.steps, "LCM")
	f.lcmExpCM = configMapName
	return f.lcmConfigMap, f.lcmErr
}
func (f *fakeFioStepper) createPVC(ctx context.Context, storageclass, size string) (*v1.PersistentVolumeClaim, error) {
	f.steps = append(f.steps, "CPVC")
	f.cPVCExpSC = storageclass
	f.cPVCExpSize = size
	return f.cPVC, f.cPVCErr
}
func (f *fakeFioStepper) deletePVC(ctx context.Context, pvcName string) error {
	f.steps = append(f.steps, "DPVC")
	return f.dPVCErr
}
func (f *fakeFioStepper) createPod(ctx context.Context, pvcName, configMapName, testFileName string) (*v1.Pod, error) {
	f.steps = append(f.steps, "CPOD")
	f.cPodExpCM = configMapName
	f.cPodExpFN = testFileName
	f.cPodExpPVC = pvcName
	return f.cPod, f.cPodErr
}
func (f *fakeFioStepper) deletePod(ctx context.Context, podName string) error {
	f.steps = append(f.steps, "DPOD")
	return f.dPodErr
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
		err := stepper.storageClassExists(ctx, tc.storageClass)
		c.Check(err, tc.checker)
	}
}

func (s *FIOTestSuite) TestLoadConfigMap(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		cli           kubernetes.Interface
		configMapName string
		jobName       string
		cmChecker     Checker
		errChecker    Checker
		failCreates   bool
	}{
		{ // provided cm name not found
			cli:           fake.NewSimpleClientset(),
			configMapName: "nonexistantcm",
			cmChecker:     IsNil,
			errChecker:    NotNil,
		},
		{ // specified config map found
			cli: fake.NewSimpleClientset(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "CM1",
					Namespace: "default",
				},
				Data: map[string]string{},
			}),
			configMapName: "CM1",
			cmChecker:     NotNil,
			errChecker:    IsNil,
		},
		{ // specified config map not found in namespace
			cli: fake.NewSimpleClientset(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "CM1",
					Namespace: "badns",
				},
				Data: map[string]string{},
			}),
			configMapName: "CM1",
			cmChecker:     IsNil,
			errChecker:    NotNil,
		},
		{ // creates the default job ConfigMap
			cli:        fake.NewSimpleClientset(),
			cmChecker:  NotNil,
			errChecker: IsNil,
		},
		{ // job doesn't exist.
			cli:        fake.NewSimpleClientset(),
			cmChecker:  IsNil,
			errChecker: NotNil,
			jobName:    "nonExistentJob",
		},
		{ // Fails to create default job
			cli:         fake.NewSimpleClientset(),
			cmChecker:   IsNil,
			errChecker:  NotNil,
			failCreates: true,
		},
	} {
		stepper := &fioStepper{cli: tc.cli}
		if tc.failCreates {
			stepper.cli.(*fake.Clientset).Fake.PrependReactor("create", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, errors.New("Error creating object")
			})
		}
		cm, err := stepper.loadConfigMap(ctx, tc.configMapName, tc.jobName)
		c.Check(err, tc.errChecker)
		c.Check(cm, tc.cmChecker)
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
		pvc, err := stepper.createPVC(ctx, tc.storageclass, tc.size)
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
			Namespace: GetPodNamespace(),
		}})}
	err := stepper.deletePVC(ctx, "pvc")
	c.Assert(err, IsNil)
	err = stepper.deletePVC(ctx, "pvc")
	c.Assert(err, NotNil)
}

func (s *FIOTestSuite) TestCreatPod(c *C) {
	ctx := context.Background()
	for _, tc := range []struct {
		pvcName          string
		configMapName    string
		testFileName     string
		reactor          []k8stesting.Reactor
		podReadyErr      error
		podSpecMergerErr error
		errChecker       Checker
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
			pvcName:          "pvc",
			configMapName:    "cm",
			testFileName:     "sdf",
			errChecker:       NotNil,
			podSpecMergerErr: fmt.Errorf("podspecmerger error"),
		},
		{
			pvcName:       "pvc",
			configMapName: "cm",
			testFileName:  "",
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
			cli:           fake.NewSimpleClientset(),
			podReady:      &fakePodReadyChecker{prcErr: tc.podReadyErr},
			podSpecMerger: &fakePodSpecMerger{psmErr: tc.podSpecMergerErr},
		}
		if tc.reactor != nil {
			stepper.cli.(*fake.Clientset).Fake.ReactionChain = tc.reactor
		}
		pod, err := stepper.createPod(ctx, tc.pvcName, tc.configMapName, tc.testFileName)
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
			c.Assert(pod.Spec.Containers[0].Command, DeepEquals, []string{"fio"})
			c.Assert(pod.Spec.Containers[0].Args, DeepEquals, []string{"--directory", "/dataset", "/etc/fio-config/$(CONFIG_FILE_NAME)"})
			c.Assert(pod.Spec.Containers[0].VolumeMounts, DeepEquals, []v1.VolumeMount{
				{Name: "persistent-storage", MountPath: "/dataset"},
				{Name: "config-map", MountPath: "/etc/configmap"},
			})
			c.Assert(len(pod.Spec.Containers[0].Env), Equals, 1)
			c.Assert(pod.Spec.Containers[0].Env[0].Name, Equals, "CONFIG_FILE_NAME")
			c.Assert(pod.Spec.Containers[0].Env[0].Value, Equals, tc.testFileName)
		}
	}
}

func (s *FIOTestSuite) TestDeletePod(c *C) {
	ctx := context.Background()
	stepper := &fioStepper{cli: fake.NewSimpleClientset(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: GetPodNamespace(),
		}})}
	err := stepper.deletePod(ctx, "pod")
	c.Assert(err, IsNil)
	err = stepper.deletePod(ctx, "pod")
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
				ConfigMapSCKey:   "storageclass",
				ConfigMapSizeKey: "10Gi",
				"testfile.fio":   "some test data",
			},
			retVal:     "testfile.fio",
			errChecker: IsNil,
		},
		{
			configMap: map[string]string{
				ConfigMapSCKey:   "storageclass",
				ConfigMapSizeKey: "10Gi",
				"testfile.fio":   "some test data",
				"testfile2.fio":  "some test data2", // only support one file
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

func (s *FIOTestSuite) TestMergePodSpec(c *C) {
	ctx := context.Background()
	runAsUserInt64 := int64(1)
	for _, tc := range []struct {
		namespace  string
		inPodSpec  v1.PodSpec
		podName    string
		parentPod  *v1.Pod
		errChecker Checker
	}{
		{
			parentPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podName",
					Namespace: "ns",
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"node": "selector",
					},
					Tolerations: []v1.Toleration{
						{Value: "toleration"},
					},
					Containers: []v1.Container{
						{Image: "Image"},
					},
					SecurityContext: &v1.PodSecurityContext{
						RunAsUser: &runAsUserInt64,
					},
				},
			},
			namespace:  "ns",
			podName:    "podName",
			inPodSpec:  v1.PodSpec{Containers: []v1.Container{{Name: "container1"}}},
			errChecker: IsNil,
		},
		{
			parentPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podName",
					Namespace: "ns",
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"node": "selector",
					},
					Tolerations: []v1.Toleration{
						{Value: "toleration"},
					},
					Containers: []v1.Container{
						{Image: "Image"},
					},
					SecurityContext: &v1.PodSecurityContext{
						RunAsUser: &runAsUserInt64,
					},
				},
			},
			namespace:  "ns",
			podName:    "podName",
			inPodSpec:  v1.PodSpec{Containers: []v1.Container{{Name: "container1"}, {Name: "container2"}}},
			errChecker: NotNil,
		},
		{
			namespace:  "ns",
			podName:    "podName",
			inPodSpec:  v1.PodSpec{Containers: []v1.Container{{Name: "container1"}}},
			errChecker: NotNil,
		},
		{
			namespace:  "ns",
			podName:    "",
			inPodSpec:  v1.PodSpec{Containers: []v1.Container{{Name: "container1"}}},
			errChecker: NotNil,
		},
	} {
		tempHostname := os.Getenv(PodNameEnvKey)
		defer func() {
			os.Setenv(PodNameEnvKey, tempHostname)
		}()
		os.Setenv(PodNameEnvKey, tc.podName)

		cli := fake.NewSimpleClientset()
		if tc.parentPod != nil {
			cli = fake.NewSimpleClientset(tc.parentPod)
		}
		psm := podSpecMerger{cli}
		outPodSpec, err := psm.mergePodSpec(ctx, tc.namespace, tc.inPodSpec)
		c.Check(err, tc.errChecker)
		if err == nil {
			c.Assert(outPodSpec, Not(DeepEquals), tc.inPodSpec)
			c.Assert(outPodSpec.NodeSelector, DeepEquals, tc.parentPod.Spec.NodeSelector)
			c.Assert(outPodSpec.Tolerations, DeepEquals, tc.parentPod.Spec.Tolerations)
			c.Assert(outPodSpec.Containers[0].Image, Equals, tc.parentPod.Spec.Containers[0].Image)
			c.Assert(outPodSpec.SecurityContext, DeepEquals, tc.parentPod.Spec.SecurityContext)
		}
		os.Setenv(PodNameEnvKey, tempHostname)
	}
}

func (s *FIOTestSuite) TestGetPodNamespace(c *C) {
	os.Setenv(PodNamespaceEnvKey, "ns")
	ns := GetPodNamespace()
	c.Assert(ns, Equals, "ns")
	os.Unsetenv(PodNamespaceEnvKey)
}

type fakePodReadyChecker struct {
	prcErr error
}

func (f *fakePodReadyChecker) waitForPodReady(ctx context.Context, namespace, name string) error {
	return f.prcErr
}

type fakePodSpecMerger struct {
	psmErr error
}

func (fm *fakePodSpecMerger) mergePodSpec(ctx context.Context, namespace string, podSpec v1.PodSpec) (v1.PodSpec, error) {
	return podSpec, fm.psmErr
}
