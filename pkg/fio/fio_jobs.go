package fio

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var fioJobs = map[string]*v1.ConfigMap{
	DefaultFIOJob: {
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: DefaultFIOJob,
		},
		Data: map[string]string{},
	},
}
