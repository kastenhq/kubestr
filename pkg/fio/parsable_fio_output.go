package fio

const parsableFioOutput = `{
	"fio version" : "fio-3.20",
	"timestamp" : 1611952282,
	"timestamp_ms" : 1611952282240,
	"time" : "Fri Jan 29 20:31:22 2021",
	"global options" : {
	  "directory" : "/dataset",
	  "randrepeat" : "0",
	  "verify" : "0",
	  "ioengine" : "libaio",
	  "direct" : "1",
	  "gtod_reduce" : "1"
	},
	"jobs" : [
	  {
		"jobname" : "read_iops",
		"groupid" : 0,
		"error" : 0,
		"eta" : 0,
		"elapsed" : 18,
		"job options" : {
		  "name" : "read_iops",
		  "bs" : "4K",
		  "iodepth" : "64",
		  "size" : "2G",
		  "rw" : "randread",
		  "ramp_time" : "2s",
		  "runtime" : "15s"
		},
		"read" : {
		  "io_bytes" : 61886464,
		  "io_kbytes" : 60436,
		  "bw_bytes" : 4039322,
		  "bw" : 3944,
		  "iops" : 982.050780,
		  "runtime" : 15321,
		  "total_ios" : 15046,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 1919,
		  "bw_max" : 7664,
		  "bw_agg" : 100.000000,
		  "bw_mean" : 3995.000000,
		  "bw_dev" : 1200.820783,
		  "bw_samples" : 30,
		  "iops_min" : 479,
		  "iops_max" : 1916,
		  "iops_mean" : 998.566667,
		  "iops_stddev" : 300.247677,
		  "iops_samples" : 30
		},
		"write" : {
		  "io_bytes" : 0,
		  "io_kbytes" : 0,
		  "bw_bytes" : 0,
		  "bw" : 0,
		  "iops" : 0.000000,
		  "runtime" : 0,
		  "total_ios" : 0,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 0,
		  "bw_max" : 0,
		  "bw_agg" : 0.000000,
		  "bw_mean" : 0.000000,
		  "bw_dev" : 0.000000,
		  "bw_samples" : 0,
		  "iops_min" : 0,
		  "iops_max" : 0,
		  "iops_mean" : 0.000000,
		  "iops_stddev" : 0.000000,
		  "iops_samples" : 0
		},
		"trim" : {
		  "io_bytes" : 0,
		  "io_kbytes" : 0,
		  "bw_bytes" : 0,
		  "bw" : 0,
		  "iops" : 0.000000,
		  "runtime" : 0,
		  "total_ios" : 0,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 0,
		  "bw_max" : 0,
		  "bw_agg" : 0.000000,
		  "bw_mean" : 0.000000,
		  "bw_dev" : 0.000000,
		  "bw_samples" : 0,
		  "iops_min" : 0,
		  "iops_max" : 0,
		  "iops_mean" : 0.000000,
		  "iops_stddev" : 0.000000,
		  "iops_samples" : 0
		},
		"sync" : {
		  "total_ios" : 0,
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  }
		},
		"job_runtime" : 15322,
		"usr_cpu" : 1.109516,
		"sys_cpu" : 3.648349,
		"ctx" : 17991,
		"majf" : 1,
		"minf" : 62,
		"iodepth_level" : {
		  "1" : 0.000000,
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  ">=64" : 100.000000
		},
		"iodepth_submit" : {
		  "0" : 0.000000,
		  "4" : 100.000000,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  "64" : 0.000000,
		  ">=64" : 0.000000
		},
		"iodepth_complete" : {
		  "0" : 0.000000,
		  "4" : 99.993354,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  "64" : 0.100000,
		  ">=64" : 0.000000
		},
		"latency_ns" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000
		},
		"latency_us" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000
		},
		"latency_ms" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000,
		  "2000" : 0.000000,
		  ">=2000" : 0.000000
		},
		"latency_depth" : 64,
		"latency_target" : 0,
		"latency_percentile" : 100.000000,
		"latency_window" : 0
	  },
	  {
		"jobname" : "write_iops",
		"groupid" : 0,
		"error" : 0,
		"eta" : 0,
		"elapsed" : 18,
		"job options" : {
		  "name" : "write_iops",
		  "bs" : "4K",
		  "iodepth" : "64",
		  "size" : "2G",
		  "rw" : "randwrite",
		  "ramp_time" : "2s",
		  "runtime" : "15s"
		},
		"read" : {
		  "io_bytes" : 0,
		  "io_kbytes" : 0,
		  "bw_bytes" : 0,
		  "bw" : 0,
		  "iops" : 0.000000,
		  "runtime" : 0,
		  "total_ios" : 0,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 0,
		  "bw_max" : 0,
		  "bw_agg" : 0.000000,
		  "bw_mean" : 0.000000,
		  "bw_dev" : 0.000000,
		  "bw_samples" : 0,
		  "iops_min" : 0,
		  "iops_max" : 0,
		  "iops_mean" : 0.000000,
		  "iops_stddev" : 0.000000,
		  "iops_samples" : 0
		},
		"write" : {
		  "io_bytes" : 24805376,
		  "io_kbytes" : 24224,
		  "bw_bytes" : 1616406,
		  "bw" : 1578,
		  "iops" : 390.525218,
		  "runtime" : 15346,
		  "total_ios" : 5993,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 512,
		  "bw_max" : 2706,
		  "bw_agg" : 100.000000,
		  "bw_mean" : 1581.066667,
		  "bw_dev" : 476.641189,
		  "bw_samples" : 30,
		  "iops_min" : 128,
		  "iops_max" : 676,
		  "iops_mean" : 395.033333,
		  "iops_stddev" : 119.151738,
		  "iops_samples" : 30
		},
		"trim" : {
		  "io_bytes" : 0,
		  "io_kbytes" : 0,
		  "bw_bytes" : 0,
		  "bw" : 0,
		  "iops" : 0.000000,
		  "runtime" : 0,
		  "total_ios" : 0,
		  "short_ios" : 0,
		  "drop_ios" : 0,
		  "slat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "clat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  },
		  "bw_min" : 0,
		  "bw_max" : 0,
		  "bw_agg" : 0.000000,
		  "bw_mean" : 0.000000,
		  "bw_dev" : 0.000000,
		  "bw_samples" : 0,
		  "iops_min" : 0,
		  "iops_max" : 0,
		  "iops_mean" : 0.000000,
		  "iops_stddev" : 0.000000,
		  "iops_samples" : 0
		},
		"sync" : {
		  "total_ios" : 0,
		  "lat_ns" : {
			"min" : 0,
			"max" : 0,
			"mean" : 0.000000,
			"stddev" : 0.000000,
			"N" : 0
		  }
		},
		"job_runtime" : 15345,
		"usr_cpu" : 0.508309,
		"sys_cpu" : 2.280873,
		"ctx" : 7411,
		"majf" : 1,
		"minf" : 63,
		"iodepth_level" : {
		  "1" : 0.000000,
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  ">=64" : 100.000000
		},
		"iodepth_submit" : {
		  "0" : 0.000000,
		  "4" : 100.000000,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  "64" : 0.000000,
		  ">=64" : 0.000000
		},
		"iodepth_complete" : {
		  "0" : 0.000000,
		  "4" : 99.983317,
		  "8" : 0.000000,
		  "16" : 0.000000,
		  "32" : 0.000000,
		  "64" : 0.100000,
		  ">=64" : 0.000000
		},
		"latency_ns" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000
		},
		"latency_us" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000
		},
		"latency_ms" : {
		  "2" : 0.000000,
		  "4" : 0.000000,
		  "10" : 0.000000,
		  "20" : 0.000000,
		  "50" : 0.000000,
		  "100" : 0.000000,
		  "250" : 0.000000,
		  "500" : 0.000000,
		  "750" : 0.000000,
		  "1000" : 0.000000,
		  "2000" : 0.000000,
		  ">=2000" : 0.000000
		},
		"latency_depth" : 64,
		"latency_target" : 0,
		"latency_percentile" : 100.000000,
		"latency_window" : 0
	  }
	],
	"disk_util" : [
	  {
		"name" : "rbd4",
		"read_ios" : 16957,
		"write_ios" : 6896,
		"read_merges" : 0,
		"write_merges" : 207,
		"read_ticks" : 1072290,
		"write_ticks" : 1043421,
		"in_queue" : 2119036,
		"util" : 99.712875
	  }
	]
  }`
