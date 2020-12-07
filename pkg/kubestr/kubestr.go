package kubestr

import (
	"github.com/kanisterio/kanister/pkg/kube"
	"github.com/kastenhq/kubestr/pkg/fio"
	"github.com/pkg/errors"
	sv1 "k8s.io/api/storage/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Kubestr is the primary object for running the kubestr tool. It holds all the cluster state information
// as well.
type Kubestr struct {
	cli                     kubernetes.Interface
	dynCli                  dynamic.Interface
	sdsfgValidator          snapshotDataSourceFG
	storageClassList        *sv1.StorageClassList
	volumeSnapshotClassList *unstructured.UnstructuredList
	Fio                     fio.FIO
}

const Logo = `
**************************************
__  __ ______ _______ _______ ______
|  |/  |  __  |     __|_     _|   __ \
|     <|  __  |__     | |   | |      <
|__|\__|______|_______| |___| |___|__|

**************************************
`

var (
	DefaultQPS   = float32(50)
	DefaultBurst = 100
)

// NewKubestr initializes a new kubestr object to run preflight tests
func NewKubestr() (*Kubestr, error) {
	cli, err := LoadKubeCli()
	if err != nil {
		return nil, err
	}
	dynCli, err := LoadDynCli()
	if err != nil {
		return nil, err
	}
	return &Kubestr{
		cli:    cli,
		dynCli: dynCli,
		sdsfgValidator: &snapshotDataSourceFGValidator{
			cli:    cli,
			dynCli: dynCli,
		},
		Fio: &fio.FIOrunner{
			Cli: cli,
		},
	}, nil
}

// LoadDynCli loads the config and returns a dynamic CLI
func LoadDynCli() (dynamic.Interface, error) {
	cfg, err := kube.LoadConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load config for Dynamic client")
	}
	clientset, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create Dynamic client")
	}
	return clientset, nil
}

// LoadKubeCli load the config and returns a kubernetes client
// NewClient returns a k8 client configured by the kanister environment.
func LoadKubeCli() (kubernetes.Interface, error) {
	config, err := kube.LoadConfig()
	if err != nil {
		return nil, err
	}
	config.QPS = DefaultQPS
	config.Burst = DefaultBurst
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
