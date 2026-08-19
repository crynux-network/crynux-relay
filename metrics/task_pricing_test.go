package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func gaugeValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()
	value := &dto.Metric{}
	if err := metric.Write(value); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return value.GetGauge().GetValue()
}

func TestCalibrationMetricsKeepBaseUnits(t *testing.T) {
	ResetTaskPricingCalibrationMetrics()
	SetTaskPricingCalibration("sd", "sd-model", "fp16", "auto", 0, 24, 30, 0.00004, [6]float64{})
	SetTaskPricingCalibration("llm", "llm-model", "", "bfloat16", 4, 24, 0, 0, [6]float64{12, 0.002, 0.3, 120, 10, 5})
	SetGPUExecutionCalibration("llm", "A100", 40, "llm-model", "", "bfloat16", 4, 16, 24, 15, 0.00003, [6]float64{10, 0.001, 0.2, 100, 8, 4}, 9)

	assertGauge := func(name string, got, want float64) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: expected %g, got %g", name, want, got)
		}
	}
	assertGauge("aggregate sd overhead", gaugeValue(t, TaskPricingSDOverheadSeconds.WithLabelValues("sd-model", "fp16", "auto", "0", "24")), 30)
	assertGauge("aggregate sd", gaugeValue(t, TaskPricingSecondsPerSDPixelStep.WithLabelValues("sd-model", "fp16", "auto", "0", "24")), 0.00004)
	assertGauge("aggregate llm input", gaugeValue(t, TaskPricingLLMCoefficient.WithLabelValues("text_input", "llm-model", "", "bfloat16", "4", "24")), 0.002)
	assertGauge("aggregate llm image", gaugeValue(t, TaskPricingLLMCoefficient.WithLabelValues("image_count", "llm-model", "", "bfloat16", "4", "24")), 10)
	labels := []string{"llm", "A100", "40", "llm-model", "", "bfloat16", "4", "16", "24"}
	assertGauge("gpu sd overhead", gaugeValue(t, GPUExecutionSDOverheadSeconds.WithLabelValues(labels...)), 15)
	assertGauge("gpu sd", gaugeValue(t, GPUExecutionSecondsPerSDPixelStep.WithLabelValues(labels...)), 0.00003)
	assertGauge("gpu llm output", gaugeValue(t, GPUExecutionLLMCoefficient.WithLabelValues(append([]string{"output"}, labels...)...)), 0.2)
	assertGauge("gpu llm samples", gaugeValue(t, GPUExecutionCalibrationSamples.WithLabelValues(labels...)), 9)
}
