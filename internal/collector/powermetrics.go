package collector

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

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

// NewPowermetricsCollector creates a new PowermetricsCollector
func NewPowermetricsCollector() *PowermetricsCollector {
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
}
