package kubestr

import (
	"context"
	"fmt"

	"github.com/kastenhq/kubestr/pkg/fio"
)

func (p *Kubestr) FIO(ctx context.Context, storageClass string) *TestOutput {
	testName := fmt.Sprintf("FIO test on storage class (%s)", storageClass)
	fioResult, err := p.fio.RunFio(ctx, &fio.RunFIOArgs{StorageClass: storageClass})
	if err != nil {
		return makeTestOutput(testName, StatusError, err.Error(), nil)
	}
	return makeTestOutput(testName, StatusOK, fmt.Sprintf("Valid kubernetes version (%s)", fioResult), fioResult)
}
