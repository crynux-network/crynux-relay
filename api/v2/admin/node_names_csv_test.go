package admin

import (
	"crynux_relay/service"
	"reflect"
	"testing"
)

func TestBuildNodeNamesCSVRows(t *testing.T) {
	rows := buildNodeNamesCSVRows([]service.NodeNameCountEntry{
		{
			GPUName:     "A100",
			GPUVram:     40,
			NodeVersion: "1.2.3",
			ActiveCount: 12,
		},
		{
			GPUName:     "RTX 4090",
			GPUVram:     24,
			NodeVersion: "1.3.0",
			ActiveCount: 7,
		},
	})
	expected := [][]string{
		{"A100", "40", "1.2.3", "12"},
		{"RTX 4090", "24", "1.3.0", "7"},
	}
	if !reflect.DeepEqual(rows, expected) {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}
