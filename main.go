package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Register collectors
	prometheus.MustRegister(newPowermetricsCollector())
	prometheus.MustRegister(newVmStatCollector())

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :9127")
	log.Fatal(http.ListenAndServe(":9127", nil))
}

// PowermetricsCollector collects powermetrics information
type PowermetricsCollector struct {
	cpuFrequency       *prometheus.Desc
	cpuTemperature     *prometheus.Desc
	cpuPower           *prometheus.Desc
	gpuPower           *prometheus.Desc
	cpuActiveResidency *prometheus.Desc
	cpuIdleResidency   *prometheus.Desc
	gpuActiveResidency *prometheus.Desc
	gpuIdleResidency   *prometheus.Desc
}

// newPowermetricsCollector creates a new PowermetricsCollector
func newPowermetricsCollector() *PowermetricsCollector {
	return &PowermetricsCollector{
		cpuFrequency: prometheus.NewDesc(
			"powermetrics_cpu_frequency_hertz",
			"Current CPU frequency in Hertz.",
			[]string{"core"}, // frequency per core
			nil,
		),
		cpuTemperature: prometheus.NewDesc(
			"powermetrics_cpu_temperature_celsius",
			"Current CPU temperature in Celsius.",
			[]string{"sensor_id"}, // temperature per sensor ID
			nil,
		),
		cpuPower: prometheus.NewDesc(
			"powermetrics_cpu_power_milliwatts",
			"Current CPU power in milliwatts.",
			nil, // total CPU power
			nil,
		),
		gpuPower: prometheus.NewDesc(
			"powermetrics_gpu_power_milliwatts",
			"Current GPU power in milliwatts.",
			nil, // total GPU power
			nil,
		),
		cpuActiveResidency: prometheus.NewDesc(
			"powermetrics_cpu_active_residency_percent",
			"Current CPU active residency percentage.",
			[]string{"core"},
			nil,
		),
		cpuIdleResidency: prometheus.NewDesc(
			"powermetrics_cpu_idle_residency_percent",
			"Current CPU idle residency percentage.",
			[]string{"core"},
			nil,
		),
		gpuActiveResidency: prometheus.NewDesc(
			"powermetrics_gpu_active_residency_percent",
			"Current GPU active residency percentage.",
			nil,
			nil,
		),
		gpuIdleResidency: prometheus.NewDesc(
			"powermetrics_gpu_idle_residency_percent",
			"Current GPU idle residency percentage.",
			nil,
			nil,
		),
	}
}

// Describe describes metrics to Prometheus
func (collector *PowermetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.cpuFrequency
	ch <- collector.cpuTemperature
	ch <- collector.cpuPower
	ch <- collector.gpuPower
	ch <- collector.cpuActiveResidency
	ch <- collector.cpuIdleResidency
	ch <- collector.gpuActiveResidency
	ch <- collector.gpuIdleResidency
}

// Partial plist structure definitions
type PowerMetricsOutput struct {
	XMLName xml.Name  `xml:"plist"`
	Dict    PlistDict `xml:"dict"`
}

type PlistDict struct {
	Keys   []string     `xml:"key"`
	Values []PlistValue `xml:"array>dict"` // simplified to target only arrays of dictionaries
}

type PlistValue struct {
	Keys         []string    `xml:"key"`
	Reals        []string    `xml:"real"`    // for temperature values
	Ints         []string    `xml:"integer"` // for frequency values
	Strings      []string    `xml:"string"`
	NestedDicts  []PlistDict `xml:"dict"`
	ArrayOfDicts []PlistDict `xml:"array>dict"` // added for <array><dict>...</dict></array> structure
}

// Collect is called by Prometheus when collecting metrics
func (collector *PowermetricsCollector) Collect(ch chan<- prometheus.Metric) {
	// powermetrics --samplers cpu_power,gpu_power -i 1 -n 1
	// Get CPU power and GPU power information (runs as root via LaunchDaemon)
	cmd := exec.Command("powermetrics", "--samplers", "cpu_power,gpu_power", "-i", "1", "-n", "1")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to run powermetrics: %v", err)
		return
	}

	// Flags to prevent duplicate metric submissions
	var cpuPowerSent, gpuPowerSent bool

	// Extract power information and frequency information from text output
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := scanner.Text()

		// Look for CPU Power: 1339 mW format
		if !cpuPowerSent && strings.Contains(line, "CPU Power:") && strings.Contains(line, "mW") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Power:" && i+1 < len(parts) {
					powerStr := parts[i+1]
					if power, err := strconv.ParseFloat(powerStr, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(collector.cpuPower, prometheus.GaugeValue, power)
						cpuPowerSent = true
					}
					break
				}
			}
		}

		// Look for GPU Power: 6 mW format
		if !gpuPowerSent && strings.Contains(line, "GPU Power:") && strings.Contains(line, "mW") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "Power:" && i+1 < len(parts) {
					powerStr := parts[i+1]
					if power, err := strconv.ParseFloat(powerStr, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(collector.gpuPower, prometheus.GaugeValue, power)
						gpuPowerSent = true
					}
					break
				}
			}
		}

		// Extract CPU frequency information
		// Look for CPU 0 frequency: 2064 MHz format
		if strings.Contains(line, "frequency:") && strings.Contains(line, "MHz") && strings.Contains(line, "CPU") {
			parts := strings.Fields(line)
			var cpuCore string
			var freqValue float64

			for i, part := range parts {
				if part == "CPU" && i+1 < len(parts) {
					cpuCore = parts[i+1]
				}
				if part == "frequency:" && i+1 < len(parts) {
					freqStr := parts[i+1]
					if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
						freqValue = freq * 1000000 // Convert MHz to Hz
					}
				}
			}

			if cpuCore != "" && freqValue > 0 {
				ch <- prometheus.MustNewConstMetric(collector.cpuFrequency, prometheus.GaugeValue, freqValue, fmt.Sprintf("cpu%s", cpuCore))
			}
		}

		// Extract CPU active residency
		// Look for CPU 0 active residency:  99.96% format
		if strings.Contains(line, "active residency:") && strings.Contains(line, "%") && strings.Contains(line, "CPU") {
			parts := strings.Fields(line)
			var cpuCore string
			var residencyValue float64

			for i, part := range parts {
				if part == "CPU" && i+1 < len(parts) {
					cpuCore = parts[i+1]
				}
				if part == "residency:" && i+1 < len(parts) {
					residencyStr := strings.TrimSuffix(parts[i+1], "%")
					if residency, err := strconv.ParseFloat(residencyStr, 64); err == nil {
						residencyValue = residency
					}
				}
			}

			if cpuCore != "" && residencyValue >= 0 {
				ch <- prometheus.MustNewConstMetric(collector.cpuActiveResidency, prometheus.GaugeValue, residencyValue, fmt.Sprintf("cpu%s", cpuCore))
			}
		}

		// Extract CPU idle residency
		// Look for CPU 0 idle residency:   0.04% format
		if strings.Contains(line, "idle residency:") && strings.Contains(line, "%") && strings.Contains(line, "CPU") {
			parts := strings.Fields(line)
			var cpuCore string
			var residencyValue float64

			for i, part := range parts {
				if part == "CPU" && i+1 < len(parts) {
					cpuCore = parts[i+1]
				}
				if part == "residency:" && i+1 < len(parts) {
					residencyStr := strings.TrimSuffix(parts[i+1], "%")
					if residency, err := strconv.ParseFloat(residencyStr, 64); err == nil {
						residencyValue = residency
					}
				}
			}

			if cpuCore != "" && residencyValue >= 0 {
				ch <- prometheus.MustNewConstMetric(collector.cpuIdleResidency, prometheus.GaugeValue, residencyValue, fmt.Sprintf("cpu%s", cpuCore))
			}
		}

		// Extract GPU HW active residency
		// Look for GPU HW active residency:   2.25% format
		if strings.Contains(line, "GPU HW active residency:") && strings.Contains(line, "%") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "residency:" && i+1 < len(parts) {
					residencyStr := strings.TrimSuffix(parts[i+1], "%")
					if residency, err := strconv.ParseFloat(residencyStr, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(collector.gpuActiveResidency, prometheus.GaugeValue, residency)
					}
					break
				}
			}
		}

		// Extract GPU idle residency
		// Look for GPU idle residency:  97.75% format
		if strings.Contains(line, "GPU idle residency:") && strings.Contains(line, "%") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "residency:" && i+1 < len(parts) {
					residencyStr := strings.TrimSuffix(parts[i+1], "%")
					if residency, err := strconv.ParseFloat(residencyStr, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(collector.gpuIdleResidency, prometheus.GaugeValue, residency)
					}
					break
				}
			}
		}
	}

	// Temperature information may need to be obtained separately if needed
	// If temperature information is not included in the current powermetrics output,
	// consider using --samplers thermal separately or other methods

	// Dummy data removed
	// ch <- prometheus.MustNewConstMetric(collector.cpuFrequency, prometheus.GaugeValue, 2400000000, "0")
	// ch <- prometheus.MustNewConstMetric(collector.cpuFrequency, prometheus.GaugeValue, 2300000000, "1")
	// ch <- prometheus.MustNewConstMetric(collector.cpuTemperature, prometheus.GaugeValue, 55.5, "T0NI")
}

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

// newVmStatCollector creates a new VmStatCollector
func newVmStatCollector() *VmStatCollector {
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

	// Dummy data removed
	// ch <- prometheus.MustNewConstMetric(collector.freePages, prometheus.GaugeValue, 100000)
	// ch <- prometheus.MustNewConstMetric(collector.activePages, prometheus.GaugeValue, 200000)
	// ch <- prometheus.MustNewConstMetric(collector.inactivePages, prometheus.GaugeValue, 50000)
}
