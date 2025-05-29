package collector

import (
	"bufio"
	"bytes"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
)

// VmStatCollector collects vm_stat information
type VmStatCollector struct {
	freePages        *prometheus.Desc
	activePages      *prometheus.Desc
	inactivePages    *prometheus.Desc
	speculativePages *prometheus.Desc
	throttledPages   *prometheus.Desc
	wiredPages       *prometheus.Desc
	purgeablePages   *prometheus.Desc
	copyOnWrite      *prometheus.Desc
	zeroFilled       *prometheus.Desc
	reactivated      *prometheus.Desc
	purged           *prometheus.Desc
	fileBacked       *prometheus.Desc
	anonymous        *prometheus.Desc
	uncompressed     *prometheus.Desc
	compressor       *prometheus.Desc
	decompressed     *prometheus.Desc
	compressed       *prometheus.Desc
	pageIns          *prometheus.Desc
	pageOuts         *prometheus.Desc
	faults           *prometheus.Desc
	swapIns          *prometheus.Desc
	swapOuts         *prometheus.Desc
	pageSize         *prometheus.Desc
}

// NewVmStatCollector creates a new VmStatCollector
func NewVmStatCollector() *VmStatCollector {
	return &VmStatCollector{
		freePages: prometheus.NewDesc(
			"vmstat_pages_free_count",
			"Number of free pages.",
			nil, nil,
		),
		activePages: prometheus.NewDesc(
			"vmstat_pages_active_count",
			"Number of active pages.",
			nil, nil,
		),
		inactivePages: prometheus.NewDesc(
			"vmstat_pages_inactive_count",
			"Number of inactive pages.",
			nil, nil,
		),
		speculativePages: prometheus.NewDesc(
			"vmstat_pages_speculative_count",
			"Number of speculative pages.",
			nil, nil,
		),
		throttledPages: prometheus.NewDesc(
			"vmstat_pages_throttled_count",
			"Number of throttled pages.",
			nil, nil,
		),
		wiredPages: prometheus.NewDesc(
			"vmstat_pages_wired_count",
			"Number of wired down pages.",
			nil, nil,
		),
		purgeablePages: prometheus.NewDesc(
			"vmstat_pages_purgeable_count",
			"Number of purgeable pages.",
			nil, nil,
		),
		copyOnWrite: prometheus.NewDesc(
			"vmstat_pages_cow_faults_total",
			"Number of copy-on-write faults.",
			nil, nil,
		),
		zeroFilled: prometheus.NewDesc(
			"vmstat_pages_zero_filled_total",
			"Number of pages zero filled.",
			nil, nil,
		),
		reactivated: prometheus.NewDesc(
			"vmstat_pages_reactivated_total",
			"Number of pages reactivated.",
			nil, nil,
		),
		purged: prometheus.NewDesc(
			"vmstat_pages_purged_total",
			"Number of pages purged.",
			nil, nil,
		),
		fileBacked: prometheus.NewDesc(
			"vmstat_pages_file_backed_count",
			"Number of pages file-backed.",
			nil, nil,
		),
		anonymous: prometheus.NewDesc(
			"vmstat_pages_anonymous_count",
			"Number of pages anonymous.",
			nil, nil,
		),
		uncompressed: prometheus.NewDesc(
			"vmstat_pages_uncompressed_total",
			"Number of pages uncompressed.",
			nil, nil,
		),
		compressor: prometheus.NewDesc(
			"vmstat_pages_compressor_count",
			"Number of pages used by compressor.",
			nil, nil,
		),
		decompressed: prometheus.NewDesc(
			"vmstat_pages_decompressed_total",
			"Number of pages decompressed.",
			nil, nil,
		),
		compressed: prometheus.NewDesc(
			"vmstat_pages_compressed_total",
			"Number of pages compressed.",
			nil, nil,
		),
		pageIns: prometheus.NewDesc(
			"vmstat_page_ins_total",
			"Number of pageins.",
			nil, nil,
		),
		pageOuts: prometheus.NewDesc(
			"vmstat_page_outs_total",
			"Number of pageouts.",
			nil, nil,
		),
		faults: prometheus.NewDesc(
			"vmstat_faults_total",
			"Number of page faults.",
			nil, nil,
		),
		swapIns: prometheus.NewDesc(
			"vmstat_swap_ins_total",
			"Number of swapins.",
			nil, nil,
		),
		swapOuts: prometheus.NewDesc(
			"vmstat_swap_outs_total",
			"Number of swapouts.",
			nil, nil,
		),
		pageSize: prometheus.NewDesc(
			"vmstat_page_size_bytes",
			"Size of pages in bytes.",
			nil, nil,
		),
	}
}

// Describe describes metrics to Prometheus
func (collector *VmStatCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.freePages
	ch <- collector.activePages
	ch <- collector.inactivePages
	ch <- collector.speculativePages
	ch <- collector.throttledPages
	ch <- collector.wiredPages
	ch <- collector.purgeablePages
	ch <- collector.copyOnWrite
	ch <- collector.zeroFilled
	ch <- collector.reactivated
	ch <- collector.purged
	ch <- collector.fileBacked
	ch <- collector.anonymous
	ch <- collector.uncompressed
	ch <- collector.compressor
	ch <- collector.decompressed
	ch <- collector.compressed
	ch <- collector.pageIns
	ch <- collector.pageOuts
	ch <- collector.faults
	ch <- collector.swapIns
	ch <- collector.swapOuts
	ch <- collector.pageSize
}

// Collect is called by Prometheus when collecting metrics
func (collector *VmStatCollector) Collect(ch chan<- prometheus.Metric) {
	// Get page size
	pageSize := syscall.Getpagesize()
	ch <- prometheus.MustNewConstMetric(collector.pageSize, prometheus.GaugeValue, float64(pageSize))

	cmd := exec.Command("vm_stat")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to run vm_stat: %v", err)
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	valueMap := make(map[string]float64)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valueStr := strings.TrimRight(strings.TrimSpace(parts[1]), ".") // Remove trailing period

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			// Ignore header lines like "Mach Virtual Memory Statistics"
			continue
		}
		valueMap[key] = value
	}

	if val, ok := valueMap["Pages free"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.freePages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages active"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.activePages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages inactive"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.inactivePages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages speculative"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.speculativePages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages throttled"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.throttledPages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages wired down"]; ok { // "wired down" is the key
		ch <- prometheus.MustNewConstMetric(collector.wiredPages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages purgeable"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.purgeablePages, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Copy-on-writes"]; ok { // "Copy-on-writes" is the key
		ch <- prometheus.MustNewConstMetric(collector.copyOnWrite, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pages zero filled"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.zeroFilled, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pages reactivated"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.reactivated, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pages purged"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.purged, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["File-backed pages"]; ok { // "File-backed pages" is the key
		ch <- prometheus.MustNewConstMetric(collector.fileBacked, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Anonymous pages"]; ok { // "Anonymous pages" is the key
		ch <- prometheus.MustNewConstMetric(collector.anonymous, prometheus.GaugeValue, val)
	}
	if val, ok := valueMap["Pages stored in compressor"]; ok { // "Pages stored in compressor" is the key
		ch <- prometheus.MustNewConstMetric(collector.compressor, prometheus.GaugeValue, val)
	}
	// if val, ok := valueMap["Pages used by compressor"]; ok { // Need to verify if this key exists in vm_stat output (possible duplication)
	// 	// Consider using collector.compressor or defining a new metric
	// }
	if val, ok := valueMap["Pages decompressed"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.decompressed, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pages compressed"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.compressed, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pageins"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.pageIns, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Pageouts"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.pageOuts, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Swapins"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.swapIns, prometheus.CounterValue, val)
	}
	if val, ok := valueMap["Swapouts"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.swapOuts, prometheus.CounterValue, val)
	}
	// "Faults" is often displayed as "Page faults" in vm_stat output, so consider both
	if val, ok := valueMap["Page faults"]; ok {
		ch <- prometheus.MustNewConstMetric(collector.faults, prometheus.CounterValue, val)
	}
}
