package fio

import "fmt"

type FioResult struct {
	FioVersion    string           `json:"fio version,omitempty"`
	Timestamp     int64            `json:"timestamp,omitempty"`
	TimestampMS   int64            `json:"timestamp_ms,omitempty"`
	Time          string           `json:"time,omitempty"`
	GlobalOptions FioGlobalOptions `json:"global options,omitempty"`
	Jobs          []FioJobs        `json:"jobs,omitempty"`
	DiskUtil      []FioDiskUtil    `json:"disk_util,omitempty"`
}

func (f FioResult) Print() string {
	var res string
	res += fmt.Sprintf("FIO version - %s\n", f.FioVersion)
	res += fmt.Sprintf("Global options - %s\n\n", f.GlobalOptions.Print())
	for _, job := range f.Jobs {
		res += fmt.Sprintf("%s\n", job.Print())
	}
	res += "Disk stats (read/write):\n"
	for _, du := range f.DiskUtil {
		res += fmt.Sprintf("%s\n", du.Print())
	}

	return res
}

type FioGlobalOptions struct {
	Directory  string `json:"directory,omitempty"`
	RandRepeat string `json:"randrepeat,omitempty"`
	Verify     string `json:"verify,omitempty"`
	IOEngine   string `json:"ioengine,omitempty"`
	Direct     string `json:"direct,omitempty"`
	GtodReduce string `json:"gtod_reduce,omitempty"`
}

func (g FioGlobalOptions) Print() string {
	return fmt.Sprintf("ioengine=%s verify=%s direct=%s gtod_reduce=%s", g.IOEngine, g.Verify, g.Direct, g.GtodReduce)
}

type FioJobs struct {
	JobName           string        `json:"jobname,omitempty"`
	GroupID           int           `json:"groupid,omitempty"`
	Error             int           `json:"error,omitempty"`
	Eta               int           `json:"eta,omitempty"`
	Elapsed           int           `json:"elapsed,omitempty"`
	JobOptions        FioJobOptions `json:"job options,omitempty"`
	Read              FioStats      `json:"read,omitempty"`
	Write             FioStats      `json:"write,omitempty"`
	Trim              FioStats      `json:"trim,omitempty"`
	Sync              FioStats      `json:"sync,omitempty"`
	JobRuntime        int32         `json:"job_runtime,omitempty"`
	UsrCpu            float32       `json:"usr_cpu,omitempty"`
	SysCpu            float32       `json:"sys_cpu,omitempty"`
	Ctx               int32         `json:"ctx,omitempty"`
	MajF              int32         `json:"majf,omitempty"`
	MinF              int32         `json:"minf,omitempty"`
	IoDepthLevel      FioDepth      `json:"iodepth_level,omitempty"`
	IoDepthSubmit     FioDepth      `json:"iodepth_submit,omitempty"`
	IoDepthComplete   FioDepth      `json:"iodepth_complete,omitempty"`
	LatencyNs         FioLatency    `json:"latency_ns,omitempty"`
	LatencyUs         FioLatency    `json:"latency_us,omitempty"`
	LatencyMs         FioLatency    `json:"latency_ms,omitempty"`
	LatencyDepth      int32         `json:"latency_depth,omitempty"`
	LatencyTarget     int32         `json:"latency_target,omitempty"`
	LatencyPercentile float32       `json:"latency_percentile,omitempty"`
	LatencyWindow     int32         `json:"latency_window,omitempty"`
}

func (j FioJobs) Print() string {
	var job string
	job += fmt.Sprintf("%s\n", j.JobOptions.Print())
	if j.Read.Iops != 0 || j.Read.BW != 0 {
		job += fmt.Sprintf("read:\n%s\n", j.Read.Print())
	}
	if j.Write.Iops != 0 || j.Write.BW != 0 {
		job += fmt.Sprintf("write:\n%s\n", j.Write.Print())
	}
	return job
}

type FioJobOptions struct {
	Name     string `json:"name,omitempty"`
	BS       string `json:"bs,omitempty"`
	IoDepth  string `json:"iodepth,omitempty"`
	Size     string `json:"size,omitempty"`
	RW       string `json:"rw,omitempty"`
	RampTime string `json:"ramp_time,omitempty"`
	RunTime  string `json:"runtime,omitempty"`
}

func (o FioJobOptions) Print() string {
	return fmt.Sprintf("JobName: %s\n  blocksize=%s filesize=%s iodepth=%s rw=%s", o.Name, o.BS, o.Size, o.IoDepth, o.RW)
}

type FioStats struct {
	IOBytes     int64   `json:"io_bytes,omitempty"`
	IOKBytes    int64   `json:"io_kbytes,omitempty"`
	BWBytes     int64   `json:"bw_bytes,omitempty"`
	BW          int64   `json:"bw,omitempty"`
	Iops        float32 `json:"iops,omitempty"`
	Runtime     int64   `json:"runtime,omitempty"`
	TotalIos    int64   `json:"total_ios,omitempty"`
	ShortIos    int64   `json:"short_ios,omitempty"`
	DropIos     int64   `json:"drop_ios,omitempty"`
	SlatNs      FioNS   `json:"slat_ns,omitempty"`
	ClatNs      FioNS   `json:"clat_ns,omitempty"`
	LatNs       FioNS   `json:"lat_ns,omitempty"`
	BwMin       int64   `json:"bw_min,omitempty"`
	BwMax       int64   `json:"bw_max,omitempty"`
	BwAgg       float32 `json:"bw_agg,omitempty"`
	BwMean      float32 `json:"bw_mean,omitempty"`
	BwDev       float32 `json:"bw_dev,omitempty"`
	BwSamples   int32   `json:"bw_samples,omitempty"`
	IopsMin     int32   `json:"iops_min,omitempty"`
	IopsMax     int32   `json:"iops_max,omitempty"`
	IopsMean    float32 `json:"iops_mean,omitempty"`
	IopsStdDev  float32 `json:"iops_stddev,omitempty"`
	IopsSamples int32   `json:"iops_samples,omitempty"`
}

func (s FioStats) Print() string {
	var stats string
	stats += fmt.Sprintf("  IOPS=%f BW(KiB/s)=%d\n", s.Iops, s.BW)
	stats += fmt.Sprintf("  iops: min=%d max=%d avg=%f\n", s.IopsMin, s.IopsMax, s.IopsMean)
	stats += fmt.Sprintf("  bw(KiB/s): min=%d max=%d avg=%f", s.BwMin, s.BwMax, s.BwMean)
	return stats
}

type FioNS struct {
	Min    int64   `json:"min,omitempty"`
	Max    int64   `json:"max,omitempty"`
	Mean   float32 `json:"mean,omitempty"`
	StdDev float32 `json:"stddev,omitempty"`
	N      int64   `json:"N,omitempty"`
}

type FioDepth struct {
	FioDepth0    float32 `json:"0,omitempty"`
	FioDepth1    float32 `json:"1,omitempty"`
	FioDepth2    float32 `json:"2,omitempty"`
	FioDepth4    float32 `json:"4,omitempty"`
	FioDepth8    float32 `json:"8,omitempty"`
	FioDepth16   float32 `json:"16,omitempty"`
	FioDepth32   float32 `json:"32,omitempty"`
	FioDepth64   float32 `json:"64,omitempty"`
	FioDepthGE64 float32 `json:">=64,omitempty"`
}

type FioLatency struct {
	FioLat2      float32 `json:"2,omitempty"`
	FioLat4      float32 `json:"4,omitempty"`
	FioLat10     float32 `json:"10,omitempty"`
	FioLat20     float32 `json:"20,omitempty"`
	FioLat50     float32 `json:"50,omitempty"`
	FioLat100    float32 `json:"100,omitempty"`
	FioLat250    float32 `json:"250,omitempty"`
	FioLat500    float32 `json:"500,omitempty"`
	FioLat750    float32 `json:"750,omitempty"`
	FioLat1000   float32 `json:"1000,omitempty"`
	FioLat2000   float32 `json:"2000,omitempty"`
	FioLatGE2000 float32 `json:">=2000,omitempty"`
}

type FioDiskUtil struct {
	Name        string  `json:"name,omitempty"`
	ReadIos     int64   `json:"read_ios,omitempty"`
	WriteIos    int64   `json:"write_ios,omitempty"`
	ReadMerges  int64   `json:"read_merges,omitempty"`
	WriteMerges int64   `json:"write_merges,omitempty"`
	ReadTicks   int64   `json:"read_ticks,omitempty"`
	WriteTicks  int64   `json:"write_ticks,omitempty"`
	InQueue     int64   `json:"in_queue,omitempty"`
	Util        float32 `json:"util,omitempty"`
}

func (d FioDiskUtil) Print() string {
	//Disk stats (read/write):
	//rbd4: ios=30022/11982, merge=0/313, ticks=1028675/1022768, in_queue=2063740, util=99.67%
	var du string
	du += fmt.Sprintf("  %s: ios=%d/%d merge=%d/%d ticks=%d/%d in_queue=%d, util=%f%%", d.Name, d.ReadIos,
		d.WriteIos, d.ReadMerges, d.WriteMerges, d.ReadTicks, d.WriteTicks, d.InQueue, d.Util)
	return du
}
