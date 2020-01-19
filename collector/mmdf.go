package collector

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/treydock/gpfs_exporter/config"
)

var (
    mappedSections = []string{"inode","fsTotal","metadata"}
    KbToBytes      = []string{"fsSize","freeBlocks","totalMetadata"}
	dfMap       = map[string]string{
		"inode:usedInodes":      "InodesUsed",
		"inode:freeInodes":      "InodesFree",
		"inode:allocatedInodes": "InodesAllocated",
		"inode:maxInodes":       "InodesTotal",
        "fsTotal:fsSize": "FSTotal",
        "fsTotal:freeBlocks": "FSFree",
        "fsTotal:freeBlocksPct": "FSFreePercent",
        "metadata:totalMetadata": "MetadataTotal",
        "metadata:freeBlocks": "MetadataFree",
        "metadata:freeBlocksPct": "MetadataFreePercent",

	}
	Inodes_used =     prometheus.NewDesc("gpfs_fs_inodes_used", "GPFS filesystem inodes used", []string{"fs"}, nil)
	Inodes_free =      prometheus.NewDesc("gpfs_fs_inodes_free", "GPFS filesystem inodes free", []string{"fs"}, nil)
	Inodes_allocated = prometheus.NewDesc("gpfs_fs_inodes_allocated", "GPFS filesystem inodes allocated", []string{"fs"}, nil)
	Inodes_total =     prometheus.NewDesc("gpfs_fs_inodes_total", "GPFS filesystem inodes total in bytes", []string{"fs"}, nil)
	Fs_total =         prometheus.NewDesc("gpfs_fs_total", "GPFS filesystem total size in bytes", []string{"fs"}, nil)
	Fs_free =         prometheus.NewDesc("gpfs_fs_free", "GPFS filesystem free size in bytes", []string{"fs"}, nil)
	Fs_free_percent =         prometheus.NewDesc("gpfs_fs_free_percent", "GPFS filesystem free percent", []string{"fs"}, nil)
	Metadata_total =         prometheus.NewDesc("gpfs_metadata_total", "GPFS total metadata size in bytes", []string{"fs"}, nil)
	Metadata_free =         prometheus.NewDesc("gpfs_metadata_free", "GPFS metadata free size in bytes", []string{"fs"}, nil)
	Metadata_free_percent =         prometheus.NewDesc("gpfs_metadata_free_percent", "GPFS metadata free percent", []string{"fs"}, nil)
)

type DFMetric struct {
	FS              string
	InodesUsed      int64
	InodesFree      int64
	InodesAllocated int64
	InodesTotal     int64
    FSTotal          int64
    FSFree          int64
    FSFreePercent       int64
    MetadataTotal   int64
    MetadataFree    int64
    MetadataFreePercent int64
}

type MmdfCollector struct{}

func (MmdfCollector) Name() string {
	return "mmdf"
}

func (MmdfCollector) Collect(target config.Target, ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
    for _, fs := range target.MmdfFilesystems {
        out, err := Mmdf(fs)
        if err != nil {
            return err
        }
        dfMetric, err := Parse_mmdf(out)
        if err != nil {
            return err
        }
		ch <- prometheus.MustNewConstMetric(Inodes_used, prometheus.GaugeValue, float64(dfMetric.InodesUsed), fs)
		ch <- prometheus.MustNewConstMetric(Inodes_free, prometheus.GaugeValue, float64(dfMetric.InodesFree), fs)
		ch <- prometheus.MustNewConstMetric(Inodes_allocated, prometheus.GaugeValue, float64(dfMetric.InodesAllocated), fs)
		ch <- prometheus.MustNewConstMetric(Inodes_total, prometheus.GaugeValue, float64(dfMetric.InodesTotal), fs)
		ch <- prometheus.MustNewConstMetric(Fs_total, prometheus.GaugeValue, float64(dfMetric.FSTotal), fs)
		ch <- prometheus.MustNewConstMetric(Fs_free, prometheus.GaugeValue, float64(dfMetric.FSFree), fs)
		ch <- prometheus.MustNewConstMetric(Fs_free_percent, prometheus.GaugeValue, float64(dfMetric.FSFreePercent), fs)
		ch <- prometheus.MustNewConstMetric(Metadata_total, prometheus.GaugeValue, float64(dfMetric.MetadataTotal), fs)
		ch <- prometheus.MustNewConstMetric(Metadata_free, prometheus.GaugeValue, float64(dfMetric.MetadataFree), fs)
		ch <- prometheus.MustNewConstMetric(Metadata_free_percent, prometheus.GaugeValue, float64(dfMetric.MetadataFreePercent), fs)
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmdf")
	return nil
}

func mmlsfs() (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmlsfs", "all", "-Y", "-T")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func parse_mmlsfs(out string) ([]string, error) {
	var filesystems []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		items := strings.Split(line, ":")
		if len(items) < 7 {
			continue
		}
		if items[2] == "HEADER" {
			continue
		}
		filesystems = append(filesystems, items[6])
	}
	return filesystems, nil
}

func Mmdf(fs string) (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmdf", fs, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func Parse_mmdf(out string) (DFMetric, error) {
    var dfMetrics DFMetric
	headers := make(map[string][]string)
	values := make(map[string][]string)
	lines := strings.Split(out, "\n")
	for _, l := range lines {
        if ! strings.HasPrefix(l, "mmdf") {
            continue
        }
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
        if ! sliceContains(mappedSections, items[1]) {
            continue
        }
		if items[2] == "HEADER" {
			for _, i := range items {
				headers[items[1]] = append(headers[items[1]], i)
			}
		} else {
			for _, i := range items {
				values[items[1]] = append(values[items[1]], i)
			}
		}
    }
    ps := reflect.ValueOf(&dfMetrics) // pointer to struct - addressable
	s := ps.Elem()                  // struct
	for k, vals := range headers {
		for i, v := range vals {
			mapKey := fmt.Sprintf("%s:%s", k, v)
            value := values[k][i]
			if field, ok := dfMap[mapKey]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(value)
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(value, 10, 64); err == nil {
                        if sliceContains(KbToBytes, v) {
                            val = val * 1024
                        }
						f.SetInt(val)
					} else {
						log.Errorf("Error parsing %s value %s: %s", mapKey, value, err.Error())
					}
				}
			}
		}
	}
	return dfMetrics, nil
}
