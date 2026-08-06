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
	SetTaskPricingCalibration("sd", 24, 0.00004, [3]float64{})
	SetTaskPricingCalibration("llm", 24, 0, [3]float64{12, 0.002, 0.3})
	SetGPUExecutionCalibration("A100", 40, 0.00003, [3]float64{10, 0.001, 0.2}, 7, 9)

	assertGauge := func(name string, got, want float64) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: expected %g, got %g", name, want, got)
		}
	}
	assertGauge("aggregate sd", gaugeValue(t, TaskPricingSecondsPerSDPixelStep.WithLabelValues("24")), 0.00004)
	assertGauge("aggregate llm input", gaugeValue(t, TaskPricingLLMCoefficient.WithLabelValues("input", "24")), 0.002)
	assertGauge("gpu sd", gaugeValue(t, GPUExecutionSecondsPerSDPixelStep.WithLabelValues("A100", "40")), 0.00003)
	assertGauge("gpu llm output", gaugeValue(t, GPUExecutionLLMCoefficient.WithLabelValues("output", "A100", "40")), 0.2)
	assertGauge("gpu llm samples", gaugeValue(t, GPUExecutionCalibrationSamples.WithLabelValues("llm", "A100", "40")), 9)
}
