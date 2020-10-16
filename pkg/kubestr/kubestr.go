package kubestr

import (
	"github.com/kanisterio/kanister/pkg/kube"
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
}

const Logo = `
**************************************
__  __ ______ _______ _______ ______
|  |/  |  __  |     __|_     _|   __ \
|     <|  __  |__     | |   | |      <
|__|\__|______|_______| |___| |___|__|

**************************************
`

// NewKubestr initializes a new kubestr object to run preflight tests
func NewKubestr() (*Kubestr, error) {
	cli, err := kube.NewClient()
	if err != nil {
		return nil, err
	}
	dynCli, err := getDynCli()
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
	}, nil
}

// getDynCli loads the config and returns a dynamic CLI
func getDynCli() (dynamic.Interface, error) {
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
