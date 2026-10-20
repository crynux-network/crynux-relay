package models

import "testing"

func TestParseTaskArgsVersion(t *testing.T) {
	got, err := ParseTaskArgsVersion(`{"version":"3.7.0","model":"org/llm"}`)
	if err != nil || got != "3.7.0" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := ParseTaskArgsVersion(`{"model":"org/llm"}`); err == nil {
		t.Fatal("missing version must fail")
	}
	if _, err := ParseTaskArgsVersion(`{"version":"3.7","model":"org/llm"}`); err == nil {
		t.Fatal("non major.minor.patch version must fail")
	}
	got, err = ParseTaskArgsVersion(`{"version":"9.1.0","model":"org/llm"}`)
	if err != nil || got != "9.1.0" {
		t.Fatalf("any formatted version must be accepted: %q, %v", got, err)
	}
}

func TestDeriveTaskSize(t *testing.T) {
	size, err := DeriveTaskSize(`{"version":"3.7.0","model":"org/llm"}`, TaskTypeLLM)
	if err != nil || size != 1 {
		t.Fatalf("llm size = %d, %v", size, err)
	}
	size, err = DeriveTaskSize(`{"version":"3.7.0","task_config":{"num_images_per_prompt":3}}`, TaskTypeSD)
	if err != nil || size != 3 {
		t.Fatalf("sd size = %d, %v", size, err)
	}
	size, err = DeriveTaskSize(`{"version":"3.7.0","task_config":{}}`, TaskTypeSD)
	if err != nil || size != 1 {
		t.Fatalf("sd default size = %d, %v", size, err)
	}
}

func TestDeriveTaskModelIDs(t *testing.T) {
	ids, err := DeriveTaskModelIDs(`{"version":"3.7.0","model":"Org/LLM"}`, TaskTypeLLM)
	if err != nil || len(ids) != 1 || ids[0] != "base:org/llm" {
		t.Fatalf("llm ids = %#v, %v", ids, err)
	}
	ids, err = DeriveTaskModelIDs(
		`{"version":"3.7.0","base_model":{"name":"Org/Base"},"components":{"vae":{"repo":"Org/VAE"},"text":{"repo":"Org/TE"}}}`,
		TaskTypeSD,
	)
	if err != nil || len(ids) != 3 || ids[0] != "base:org/base" || ids[1] != "component:text:org/te" || ids[2] != "component:vae:org/vae" {
		t.Fatalf("sd ids = %#v, %v", ids, err)
	}
}
