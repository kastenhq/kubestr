package kubestr

import (
	"context"
	"fmt"

	"github.com/kastenhq/kubestr/pkg/fio"
)

func (p *Kubestr) FIO(ctx context.Context, storageClass, configMap, jobName string) *TestOutput {
	testName := "FIO test results"
	fioResult, err := p.fio.RunFio(ctx, &fio.RunFIOArgs{
		StorageClass:  storageClass,
		ConfigMapName: configMap,
		JobName:       jobName,
	})
	if err != nil {
		return makeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return makeTestOutput(testName, StatusOK, fmt.Sprintf("\n%s\n", fioResult), fioResult)
}
