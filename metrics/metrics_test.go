package metrics

import (
	"crynux_relay/models"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestVramTierLabel(t *testing.T) {
	InitVramTiers([]uint64{8, 16, 24, 32, 48})
	defer InitVramTiers(nil)

	cases := []struct {
		minVram uint64
		want    string
	}{
		{0, "0-8"},
		{7, "0-8"},
		{8, "8-16"},
		{16, "16-24"},
		{24, "24-32"},
		{40, "32-48"},
		{48, "48+"},
		{100, "48+"},
	}
	for _, c := range cases {
		if got := VramTierLabel(c.minVram); got != c.want {
			t.Errorf("VramTierLabel(%d) = %q, want %q", c.minVram, got, c.want)
		}
	}
}

func TestVramTierLabelWithoutTiers(t *testing.T) {
	InitVramTiers(nil)
	if got := VramTierLabel(16); got != "all" {
		t.Errorf("VramTierLabel(16) = %q, want %q", got, "all")
	}
}

func TestGPULabel(t *testing.T) {
	if got := GPULabel(""); got != "any" {
		t.Errorf("GPULabel(\"\") = %q, want %q", got, "any")
	}
	if got := GPULabel("NVIDIA GeForce RTX 4090"); got != "NVIDIA GeForce RTX 4090" {
		t.Errorf("GPULabel returned %q", got)
	}
}

func TestMetricsEndpointServesRegisteredMetrics(t *testing.T) {
	TasksCreated.WithLabelValues("sd", "0xabc", "8-16").Inc()
	TasksDispatched.WithLabelValues("sd").Inc()
	TasksDelivered.Inc()
	TasksErrorReported.Inc()
	TasksTerminal.WithLabelValues("success", "sd", "8-16").Inc()
	TasksAborted.WithLabelValues("timeout", "TaskStartedUndelivered", "sd", "8-16").Inc()
	TaskQueueWaitSeconds.WithLabelValues("sd", "8-16").Observe(1.5)
	TaskExecutionSeconds.WithLabelValues("sd", "8-16").Observe(60)
	NodeSelectionCandidates.WithLabelValues("sd", "8-16", "any").Observe(10)
	SetNodeSelectionEmptyPoolTasks(map[SelectionLabels]int{
		{TaskType: "sd", VramTier: "8-16", GPU: "any"}: 1,
	})
	NodeHealthPenalties.Inc()
	NodeEvents.WithLabelValues("join").Inc()
	TaskQueueDepth.Set(3)
	Nodes.WithLabelValues(NodeStatusLabel(models.NodeStatusAvailable)).Set(5)
	NodesFailing30m.Set(1)
	NodesAlive.Set(5)

	server := httptest.NewServer(promhttp.HandlerFor(Registry, promhttp.HandlerOpts{}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("failed to scrape metrics: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read metrics response: %v", err)
	}
	output := string(body)

	expectedMetrics := []string{
		"relay_tasks_created_total",
		"relay_tasks_dispatched_total",
		"relay_tasks_delivered_total",
		"relay_tasks_error_reported_total",
		"relay_tasks_terminal_total",
		"relay_tasks_aborted_total",
		"relay_task_queue_wait_seconds",
		"relay_task_execution_seconds",
		"relay_node_selection_candidates",
		"relay_node_selection_empty_pool_tasks",
		"relay_node_health_penalties_total",
		"relay_node_events_total",
		"relay_task_queue_depth",
		"relay_nodes",
		"relay_nodes_failing_30m",
		"relay_nodes_alive",
	}
	for _, name := range expectedMetrics {
		if !strings.Contains(output, name) {
			t.Errorf("metrics output does not contain %q", name)
		}
	}
}
