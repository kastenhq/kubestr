package fio

var fioJobs = map[string]string{
	DefaultFIOJob: testJob1,
	"randrw":      randReadWrite,
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
[job3]
name=read_bw
bs=128K
iodepth=64
size=2G
readwrite=randread
time_based
ramp_time=2s
runtime=15s
[job4]
name=write_bw
bs=128k
iodepth=64
size=2G
readwrite=randwrite
time_based
ramp_time=2s
runtime=15s
`

var randReadWrite = `[global]
randrepeat=0
verify=0
ioengine=libaio
direct=1
gtod_reduce=1
[job1]
name=rand_readwrite
bs=4K
iodepth=64
size=4G
readwrite=randrw
rwmixread=75
time_based
ramp_time=2s
runtime=15s
`
