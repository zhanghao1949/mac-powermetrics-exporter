package collector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"log"
	"os/exec"

	"github.com/prometheus/client_golang/prometheus"
)

// MacMonCollector 定义 Prometheus 指标描述符
type MacMonCollector struct {
	allPower            *prometheus.Desc
	anePower            *prometheus.Desc
	cpuPower            *prometheus.Desc
	gpuPower            *prometheus.Desc
	gpuRAMPower         *prometheus.Desc
	ramPower            *prometheus.Desc
	sysPower            *prometheus.Desc
	cpuTempAvg          *prometheus.Desc
	gpuTempAvg          *prometheus.Desc
	ecpuFrequency       *prometheus.Desc
	ecpuUsagePercent    *prometheus.Desc
	pcpuFrequency       *prometheus.Desc
	pcpuUsagePercent    *prometheus.Desc
	gpuFrequency        *prometheus.Desc
	gpuUsagePercent     *prometheus.Desc
	ramTotalBytes       *prometheus.Desc
	ramUsedBytes        *prometheus.Desc
	swapTotalBytes      *prometheus.Desc
	swapUsedBytes       *prometheus.Desc
}

// NewMacMonCollector 创建新的 Collector 实例
func NewMacMonCollector() *MacMonCollector {
	return &MacMonCollector{
		allPower: prometheus.NewDesc(
			"macmon_all_power_watts",
			"Total power consumption in Watts.",
			nil,
			nil,
		),
		anePower: prometheus.NewDesc(
			"macmon_ane_power_watts",
			"Current ANE power in Watts.",
			nil,
			nil,
		),
		cpuPower: prometheus.NewDesc(
			"macmon_cpu_power_watts",
			"Current CPU power in Watts.",
			nil,
			nil,
		),
		gpuPower: prometheus.NewDesc(
			"macmon_gpu_power_watts",
			"Current GPU power in Watts.",
			nil,
			nil,
		),
		gpuRAMPower: prometheus.NewDesc(
			"macmon_gpu_ram_power_watts",
			"Current GPU RAM power in Watts.",
			nil,
			nil,
		),
		ramPower: prometheus.NewDesc(
			"macmon_ram_power_watts",
			"Current RAM power in Watts.",
			nil,
			nil,
		),
		sysPower: prometheus.NewDesc(
			"macmon_sys_power_watts",
			"Current system power in Watts.",
			nil,
			nil,
		),
		cpuTempAvg: prometheus.NewDesc(
			"macmon_cpu_temperature_celsius",
			"Average CPU temperature in Celsius.",
			nil,
			nil,
		),
		gpuTempAvg: prometheus.NewDesc(
			"macmon_gpu_temperature_celsius",
			"Average GPU temperature in Celsius.",
			nil,
			nil,
		),
		ecpuFrequency: prometheus.NewDesc(
			"macmon_ecpu_frequency_megahertz",
			"Efficiency CPU frequency in Megahertz.",
			nil,
			nil,
		),
		ecpuUsagePercent: prometheus.NewDesc(
			"macmon_ecpu_usage_percent",
			"Efficiency CPU usage percentage.",
			nil,
			nil,
		),
		pcpuFrequency: prometheus.NewDesc(
			"macmon_pcpu_frequency_megahertz",
			"Performance CPU frequency in Megahertz.",
			nil,
			nil,
		),
		pcpuUsagePercent: prometheus.NewDesc(
			"macmon_pcpu_usage_percent",
			"Performance CPU usage percentage.",
			nil,
			nil,
		),
		gpuFrequency: prometheus.NewDesc(
			"macmon_gpu_frequency_megahertz",
			"GPU frequency in Megahertz.",
			nil,
			nil,
		),
		gpuUsagePercent: prometheus.NewDesc(
			"macmon_gpu_usage_percent",
			"GPU usage percentage.",
			nil,
			nil,
		),
		ramTotalBytes: prometheus.NewDesc(
			"macmon_memory_ram_total_bytes",
			"Total RAM size in bytes.",
			nil,
			nil,
		),
		ramUsedBytes: prometheus.NewDesc(
			"macmon_memory_ram_used_bytes",
			"Used RAM size in bytes.",
			nil,
			nil,
		),
		swapTotalBytes: prometheus.NewDesc(
			"macmon_memory_swap_total_bytes",
			"Total swap size in bytes.",
			nil,
			nil,
		),
		swapUsedBytes: prometheus.NewDesc(
			"macmon_memory_swap_used_bytes",
			"Used swap size in bytes.",
			nil,
			nil,
		),
	}
}

// Describe 方法注册指标到 Prometheus
func (collector *MacMonCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.allPower
	ch <- collector.anePower
	ch <- collector.cpuPower
	ch <- collector.gpuPower
	ch <- collector.gpuRAMPower
	ch <- collector.ramPower
	ch <- collector.sysPower
	ch <- collector.cpuTempAvg
	ch <- collector.gpuTempAvg
	ch <- collector.ecpuFrequency
	ch <- collector.ecpuUsagePercent
	ch <- collector.pcpuFrequency
	ch <- collector.pcpuUsagePercent
	ch <- collector.gpuFrequency
	ch <- collector.gpuUsagePercent
	ch <- collector.ramTotalBytes
	ch <- collector.ramUsedBytes
	ch <- collector.swapTotalBytes
	ch <- collector.swapUsedBytes
}

// 定义 JSON 输出结构体
type MacMonOutput struct {
	AllPower   float64         `json:"all_power"`
	ANEPower   float64         `json:"ane_power"`
	CPUPower   float64         `json:"cpu_power"`
	GPUPower   float64         `json:"gpu_power"`
	GPURAMPower float64        `json:"gpu_ram_power"`
	RAMPower   float64         `json:"ram_power"`
	SysPower   float64         `json:"sys_power"`
	Temp       struct {
		CPUTempAvg float64 `json:"cpu_temp_avg"`
		GPUTempAvg float64 `json:"gpu_temp_avg"`
	} `json:"temp"`
	ECPUsage []float64 `json:"ecpu_usage"` // [frequency(MHz), usage(%)]
	PCPUsage []float64 `json:"pcpu_usage"` // [frequency(MHz), usage(%)]
	GPUUsage []float64 `json:"gpu_usage"`  // [frequency(MHz), usage(%)]
	Memory   struct {
		RAMTotal int64 `json:"ram_total"`
		RAMUsage int64 `json:"ram_usage"`
		SwapTotal int64 `json:"swap_total"`
		SwapUsage int64 `json:"swap_usage"`
	} `json:"memory"`
}

// Collect 方法执行命令并发送数据到 Prometheus
func (collector *MacMonCollector) Collect(ch chan<- prometheus.Metric) {
	cmd := exec.Command("macmon", "pipe", "-s", "1")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to run macmon: %v", err)
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := scanner.Text()

		// 解析 JSON 数据
		var data MacMonOutput
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			log.Printf("Failed to parse JSON: %v", err)
			continue
		}

		// 发送指标
		ch <- prometheus.MustNewConstMetric(collector.allPower, prometheus.GaugeValue, data.AllPower)
		ch <- prometheus.MustNewConstMetric(collector.anePower, prometheus.GaugeValue, data.ANEPower)
		ch <- prometheus.MustNewConstMetric(collector.cpuPower, prometheus.GaugeValue, data.CPUPower)
		ch <- prometheus.MustNewConstMetric(collector.gpuPower, prometheus.GaugeValue, data.GPUPower)
		ch <- prometheus.MustNewConstMetric(collector.gpuRAMPower, prometheus.GaugeValue, data.GPURAMPower)
		ch <- prometheus.MustNewConstMetric(collector.ramPower, prometheus.GaugeValue, data.RAMPower)
		ch <- prometheus.MustNewConstMetric(collector.sysPower, prometheus.GaugeValue, data.SysPower)
		ch <- prometheus.MustNewConstMetric(collector.cpuTempAvg, prometheus.GaugeValue, data.Temp.CPUTempAvg)
		ch <- prometheus.MustNewConstMetric(collector.gpuTempAvg, prometheus.GaugeValue, data.Temp.GPUTempAvg)

		if len(data.ECPUsage) >= 2 {
			ch <- prometheus.MustNewConstMetric(collector.ecpuFrequency, prometheus.GaugeValue, data.ECPUsage[0])
			ch <- prometheus.MustNewConstMetric(collector.ecpuUsagePercent, prometheus.GaugeValue, data.ECPUsage[1])
		}

		if len(data.PCPUsage) >= 2 {
			ch <- prometheus.MustNewConstMetric(collector.pcpuFrequency, prometheus.GaugeValue, data.PCPUsage[0])
			ch <- prometheus.MustNewConstMetric(collector.pcpuUsagePercent, prometheus.GaugeValue, data.PCPUsage[1])
		}

		if len(data.GPUUsage) >= 2 {
			ch <- prometheus.MustNewConstMetric(collector.gpuFrequency, prometheus.GaugeValue, data.GPUUsage[0])
			ch <- prometheus.MustNewConstMetric(collector.gpuUsagePercent, prometheus.GaugeValue, data.GPUUsage[1])
		}

		ch <- prometheus.MustNewConstMetric(collector.ramTotalBytes, prometheus.GaugeValue, float64(data.Memory.RAMTotal))
		ch <- prometheus.MustNewConstMetric(collector.ramUsedBytes, prometheus.GaugeValue, float64(data.Memory.RAMUsage))
		ch <- prometheus.MustNewConstMetric(collector.swapTotalBytes, prometheus.GaugeValue, float64(data.Memory.SwapTotal))
		ch <- prometheus.MustNewConstMetric(collector.swapUsedBytes, prometheus.GaugeValue, float64(data.Memory.SwapUsage))
	}
}
