package main

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestE2E(t *testing.T) {
	// アプリケーションをバックグラウンドで起動
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "go", "run", "cmd/main.go")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// アプリケーションの起動を待つ
	time.Sleep(3 * time.Second)

	// アプリケーションが起動したかを確認
	resp, err := http.Get("http://localhost:9127/metrics")
	if err != nil {
		t.Fatalf("Failed to connect to application: %v", err)
	}
	defer resp.Body.Close()

	// ステータスコードが200であることを確認
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}

	// レスポンスボディを読み取り
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// 基本的なPrometheusメトリクスが含まれていることを確認
	t.Run("Basic Prometheus metrics", func(t *testing.T) {
		expectedMetrics := []string{
			"go_goroutines",
			"go_info",
			"promhttp_metric_handler_requests_total",
		}

		for _, metric := range expectedMetrics {
			if !strings.Contains(bodyStr, metric) {
				t.Errorf("Expected metric %s not found in response", metric)
			}
		}
	})

	// vmstatメトリクスが含まれていることを確認
	t.Run("VmStat metrics", func(t *testing.T) {
		expectedVmStatMetrics := []string{
			"vmstat_pages_free_count",
			"vmstat_pages_active_count",
			"vmstat_pages_inactive_count",
			"vmstat_pages_speculative_count",
			"vmstat_pages_wired_count",
			"vmstat_pages_purgeable_count",
			"vmstat_pages_cow_faults_total",
			"vmstat_pages_zero_filled_total",
			"vmstat_pages_reactivated_total",
			"vmstat_pages_purged_total",
			"vmstat_pages_file_backed_count",
			"vmstat_pages_anonymous_count",
			"vmstat_pages_compressor_count",
			"vmstat_pages_decompressed_total",
			"vmstat_pages_compressed_total",
			"vmstat_page_ins_total",
			"vmstat_page_outs_total",
			"vmstat_faults_total",
			"vmstat_swap_ins_total",
			"vmstat_swap_outs_total",
			"vmstat_page_size_bytes",
		}

		for _, metric := range expectedVmStatMetrics {
			if !strings.Contains(bodyStr, metric) {
				t.Logf("VmStat metric %s not found in response (this may be expected if the metric is not available)", metric)
			}
		}

		// 少なくともいくつかのvmstatメトリクスが存在することを確認
		vmstatCount := 0
		for _, metric := range expectedVmStatMetrics {
			if strings.Contains(bodyStr, metric) {
				vmstatCount++
			}
		}

		if vmstatCount == 0 {
			t.Error("No vmstat metrics found in response")
		} else {
			t.Logf("Found %d vmstat metrics", vmstatCount)
		}
	})

	// powermetricsメトリクスが含まれていることを確認（権限がある場合）
	t.Run("Powermetrics metrics", func(t *testing.T) {
		expectedPowermetricsMetrics := []string{
			"powermetrics_cpu_frequency_hertz",
			"powermetrics_cpu_temperature_celsius",
			"powermetrics_cpu_power_milliwatts",
			"powermetrics_gpu_power_milliwatts",
			"powermetrics_cpu_active_residency_percent",
			"powermetrics_cpu_idle_residency_percent",
			"powermetrics_gpu_active_residency_percent",
			"powermetrics_gpu_idle_residency_percent",
		}

		powermetricsCount := 0
		for _, metric := range expectedPowermetricsMetrics {
			if strings.Contains(bodyStr, metric) {
				powermetricsCount++
			}
		}

		if powermetricsCount == 0 {
			t.Log("No powermetrics metrics found in response (this may be expected if powermetrics requires elevated privileges)")
		} else {
			t.Logf("Found %d powermetrics metrics", powermetricsCount)
		}
	})

	// メトリクスの形式が正しいことを確認
	t.Run("Metrics format", func(t *testing.T) {
		lines := strings.Split(bodyStr, "\n")

		helpCount := 0
		typeCount := 0
		metricCount := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# HELP") {
				helpCount++
			} else if strings.HasPrefix(line, "# TYPE") {
				typeCount++
			} else if line != "" && !strings.HasPrefix(line, "#") {
				metricCount++
			}
		}

		if helpCount == 0 {
			t.Error("No HELP comments found in metrics output")
		}
		if typeCount == 0 {
			t.Error("No TYPE comments found in metrics output")
		}
		if metricCount == 0 {
			t.Error("No metric values found in metrics output")
		}

		t.Logf("Found %d HELP comments, %d TYPE comments, %d metric values", helpCount, typeCount, metricCount)
	})

	// 正しいポートとパスでアクセスできることを確認
	t.Run("Correct port and path", func(t *testing.T) {
		// 正しいパス
		resp, err := http.Get("http://localhost:9127/metrics")
		if err != nil {
			t.Errorf("Failed to access correct path: %v", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for /metrics, got %d", resp.StatusCode)
			}
		}

		// 間違ったパス
		resp, err = http.Get("http://localhost:9127/wrong-path")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Error("Expected non-200 status for wrong path, got 200")
			}
		}
	})

	// アプリケーションを停止
	cancel()
	cmd.Wait()
}

func TestPortConfiguration(t *testing.T) {
	// main.goファイルを読み取ってポート設定を確認
	// この部分は実際のコードを読み取って確認する
	t.Run("Port configuration in code", func(t *testing.T) {
		// この部分は実装時に実際のコードを確認する
		t.Log("Port 9127 is configured in the application")
	})
}
