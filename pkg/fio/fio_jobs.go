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
		Data: map[string]string{
			DefaultFIOJob: testJob1,
		},
	},
}

var testJob1 = `[global]
randrepeat=0
verify=0
ioengine=libaio
direct=1
gtod_reduce=1
[job1]
name=read_iops
bs=4K
iodepth=64
size=2G
readwrite=randread
time_based
ramp_time=2s
runtime=15s
[job2]
name=write_iops
bs=4K
iodepth=64
size=2G
readwrite=randwrite
time_based
ramp_time=2s
runtime=15s
`
