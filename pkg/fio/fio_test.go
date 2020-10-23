package fio

import (
	"context"
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
