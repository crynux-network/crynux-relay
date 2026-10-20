package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
)

func llmTaskArgs(model, revision, dtype, quantization string) string {
	args := fmt.Sprintf(`{"version":"3.7.0","model":%q,"dtype":%q`, model, dtype)
	if revision != "" {
		args += fmt.Sprintf(`,"revision":%q`, revision)
	}
	if quantization != "" {
		args += `,"quantization_config":` + quantization
	}
	return args + `,"messages":[{"role":"user","content":"hi"}]}`
}

func imageTaskArgs(base, baseRevision, dtype, lora, components, quantization string) string {
	baseModel := fmt.Sprintf(`{"name":%q}`, base)
	if baseRevision != "" {
		baseModel = fmt.Sprintf(`{"name":%q,"revision":%q}`, base, baseRevision)
	}
	args := `{"version":"3.7.0","base_model":` + baseModel + fmt.Sprintf(`,"dtype":%q,"prompt":"a cat"`, dtype)
	if lora != "" {
		args += `,"lora":` + lora
	}
	if components != "" {
		args += `,"components":` + components
	}
	if quantization != "" {
		args += `,"quantization_config":` + quantization
	}
	return args + `}`
}

func mustDerive(t *testing.T, args string, taskType TaskType) *TaskModelRequirements {
	t.Helper()
	requirements, err := DeriveTaskModelRequirements(args, taskType)
	if err != nil {
		t.Fatalf("derive failed: %v", err)
	}
	return requirements
}

const bnbConfig = `{"quant_method":"bitsandbytes","components_to_quantize":["transformer"],"quant_kwargs":{"load_in_4bit":true}}`

func TestLLMRequirementsHaveSingleMember(t *testing.T) {
	exact := mustDerive(t, llmTaskArgs("org/llm", "sha1", "auto", ""), TaskTypeLLM)
	if len(exact.Entries) != 1 || len(exact.RequiredModelFileHashes) != 1 || len(exact.RequiredModelFileSourceHashes) != 0 {
		t.Fatalf("unexpected exact requirements: %+v", exact)
	}
	entry := exact.Entries[0]
	if entry.Repository != "org/llm" || entry.Revision != "sha1" || entry.Format != "" || len(entry.FormatKwargs) != 0 {
		t.Fatalf("unexpected llm entry: %+v", entry)
	}
	wantHash, err := entry.ModelFileHash()
	if err != nil || exact.RequiredModelFileHashes[0] != wantHash {
		t.Fatalf("unexpected exact hash: %v, %v", exact.RequiredModelFileHashes, err)
	}

	floating := mustDerive(t, llmTaskArgs("org/llm", "", "auto", ""), TaskTypeLLM)
	if len(floating.RequiredModelFileHashes) != 0 || len(floating.RequiredModelFileSourceHashes) != 1 {
		t.Fatalf("unexpected floating requirements: %+v", floating)
	}
	wantSource, _ := floating.Entries[0].ModelFileSourceHash()
	if floating.RequiredModelFileSourceHashes[0] != wantSource {
		t.Fatal("source hash mismatch")
	}
	if floating.RequiredModelFileSourceHashes[0] != mustHashValue(entry.ModelFileSourceHash()) {
		t.Fatal("revision-less entry must share the source hash of the exact entry")
	}
}

func mustHashValue(h Hash32, err error) Hash32 {
	if err != nil {
		panic(err)
	}
	return h
}

func TestImageRequirementsIncludeBaseAndComponentsButNotLora(t *testing.T) {
	components := `{
		"transformer":{"format":"gguf","repo":"org/quant","revision":"csha","format_kwargs":{"filename":"t.gguf"}},
		"text_encoder":{"format":"safetensors","repo":"org/te","format_kwargs":{}}
	}`
	lora := `{"model":"org/lora","weight_file_name":"l.safetensors","weight":80}`
	requirements := mustDerive(t, imageTaskArgs("org/pipeline", "bsha", "bfloat16", lora, components, ""), TaskTypeSD)

	if len(requirements.Entries) != 3 {
		t.Fatalf("expected base and two components, got %d entries", len(requirements.Entries))
	}
	for _, entry := range requirements.Entries {
		if entry.Repository == "org/lora" {
			t.Fatal("LoRA must not be a required model file")
		}
	}
	if len(requirements.RequiredModelFileHashes) != 2 {
		t.Fatalf("expected exact hashes for base and transformer, got %d", len(requirements.RequiredModelFileHashes))
	}
	if len(requirements.RequiredModelFileSourceHashes) != 1 {
		t.Fatalf("expected one source hash for text_encoder, got %d", len(requirements.RequiredModelFileSourceHashes))
	}

	base := ModelFileEntry{
		SourceType: ModelSourceTypeHuggingFace, Repository: "org/pipeline", Revision: "bsha",
		Format: ImageBaseModelFormat, FormatKwargs: map[string]any{},
	}
	transformer := ModelFileEntry{
		SourceType: ModelSourceTypeHuggingFace, Repository: "org/quant", Revision: "csha",
		Format: "gguf", FormatKwargs: map[string]any{"filename": "t.gguf"},
	}
	want := SortedUniqueHashes([]Hash32{mustHashValue(base.ModelFileHash()), mustHashValue(transformer.ModelFileHash())})
	for i := range want {
		if requirements.RequiredModelFileHashes[i] != want[i] {
			t.Fatalf("exact hash %d mismatch", i)
		}
	}
}

func TestImageRequirementsWithoutComponentsHaveOnlyBase(t *testing.T) {
	requirements := mustDerive(t, imageTaskArgs("org/pipeline", "", "auto", "", "", ""), TaskTypeSD)
	if len(requirements.Entries) != 1 || len(requirements.RequiredModelFileSourceHashes) != 1 || len(requirements.RequiredModelFileHashes) != 0 {
		t.Fatalf("unexpected requirements: %+v", requirements)
	}
}

func TestQuantizationConfigHashTracksOnlyQuantizationConfig(t *testing.T) {
	base := mustDerive(t, llmTaskArgs("org/llm", "", "auto", bnbConfig), TaskTypeLLM)
	changedConfig := mustDerive(t, llmTaskArgs("org/llm", "", "auto",
		`{"quant_method":"bitsandbytes","components_to_quantize":["transformer"],"quant_kwargs":{"load_in_4bit":false}}`), TaskTypeLLM)
	if base.QuantizationConfigHash == changedConfig.QuantizationConfigHash {
		t.Fatal("quantization_config change must alter quantization_config_hash")
	}
	if base.RequestedModelCacheHash == changedConfig.RequestedModelCacheHash {
		t.Fatal("quantization_config change must alter requested_model_cache_hash")
	}

	otherModel := mustDerive(t, llmTaskArgs("org/other", "", "float16", bnbConfig), TaskTypeLLM)
	if base.QuantizationConfigHash != otherModel.QuantizationConfigHash {
		t.Fatal("model and dtype change must not alter quantization_config_hash")
	}

	imageA := mustDerive(t, imageTaskArgs("org/p", "", "auto", "", "", bnbConfig), TaskTypeSD)
	imageB := mustDerive(t, imageTaskArgs("org/p2", "", "bfloat16", `{"model":"org/lora"}`, "", bnbConfig), TaskTypeSD)
	if imageA.QuantizationConfigHash != imageB.QuantizationConfigHash {
		t.Fatal("model, dtype and LoRA change must not alter quantization_config_hash")
	}
}

func TestQuantizationConfigHashIgnoresKeyOrderAndNullHasFixedValue(t *testing.T) {
	a := mustDerive(t, llmTaskArgs("org/llm", "", "auto",
		`{"quant_method":"bitsandbytes","components_to_quantize":["a"],"quant_kwargs":{"x":1,"y":2}}`), TaskTypeLLM)
	b := mustDerive(t, llmTaskArgs("org/llm", "", "auto",
		`{"quant_kwargs":{"y":2,"x":1},"components_to_quantize":["a"],"quant_method":"bitsandbytes"}`), TaskTypeLLM)
	if a.QuantizationConfigHash != b.QuantizationConfigHash {
		t.Fatal("quantization_config key order must not alter its hash")
	}

	absent := mustDerive(t, llmTaskArgs("org/llm", "", "auto", ""), TaskTypeLLM)
	null := mustDerive(t, llmTaskArgs("org/llm", "", "auto", "null"), TaskTypeLLM)
	if absent.QuantizationConfigHash != null.QuantizationConfigHash {
		t.Fatal("absent and null quantization_config must hash the same")
	}
	nullHash, err := QuantizationConfigHash(nil)
	if err != nil || nullHash != absent.QuantizationConfigHash {
		t.Fatalf("null config must use the fixed canonical-null hash: %v", err)
	}
	if nullHash.Hex() != "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b" {
		t.Fatalf("unexpected canonical-null hash: %s", nullHash.Hex())
	}
}

func TestRequestedModelCacheHashIgnoresRevisionAndTracksCacheFields(t *testing.T) {
	base := mustDerive(t, llmTaskArgs("org/llm", "", "auto", ""), TaskTypeLLM)
	pinned := mustDerive(t, llmTaskArgs("org/llm", "sha1", "auto", ""), TaskTypeLLM)
	if base.RequestedModelCacheHash != pinned.RequestedModelCacheHash {
		t.Fatal("revision must not participate in requested_model_cache_hash")
	}
	if base.RequestedModelCacheHash == mustDerive(t, llmTaskArgs("org/llm", "", "float16", ""), TaskTypeLLM).RequestedModelCacheHash {
		t.Fatal("dtype must alter requested_model_cache_hash")
	}
	if base.RequestedModelCacheHash == mustDerive(t, llmTaskArgs("org/other", "", "auto", ""), TaskTypeLLM).RequestedModelCacheHash {
		t.Fatal("model must alter requested_model_cache_hash")
	}

	imageBase := mustDerive(t, imageTaskArgs("org/p", "", "auto", "", "", ""), TaskTypeSD)
	withLora := mustDerive(t, imageTaskArgs("org/p", "", "auto", `{"model":"org/lora"}`, "", ""), TaskTypeSD)
	if imageBase.RequestedModelCacheHash == withLora.RequestedModelCacheHash {
		t.Fatal("LoRA must alter requested_model_cache_hash")
	}
	if len(withLora.Entries) != len(imageBase.Entries) {
		t.Fatal("LoRA must not alter required model files")
	}
	explicitDefault := mustDerive(t, imageTaskArgs("org/p", "", "auto", `{"model":"org/lora","weight_file_name":"","weight":100}`, "", ""), TaskTypeSD)
	if withLora.RequestedModelCacheHash != explicitDefault.RequestedModelCacheHash {
		t.Fatal("LoRA defaults must hash like explicit default values")
	}

	withComponent := mustDerive(t, imageTaskArgs("org/p", "", "auto", "",
		`{"transformer":{"format":"gguf","repo":"org/q","format_kwargs":{"filename":"a.gguf"}}}`, ""), TaskTypeSD)
	if imageBase.RequestedModelCacheHash == withComponent.RequestedModelCacheHash {
		t.Fatal("components must alter requested_model_cache_hash")
	}
	otherUsage := mustDerive(t, imageTaskArgs("org/p", "", "auto", "",
		`{"text_encoder":{"format":"gguf","repo":"org/q","format_kwargs":{"filename":"a.gguf"}}}`, ""), TaskTypeSD)
	if withComponent.RequestedModelCacheHash == otherUsage.RequestedModelCacheHash {
		t.Fatal("component usage name must alter requested_model_cache_hash")
	}
}

func TestDeriveTaskModelRequirementsRejectsInvalidArgs(t *testing.T) {
	cases := map[string]struct {
		args     string
		taskType TaskType
	}{
		"invalid json":       {`{`, TaskTypeLLM},
		"missing llm model":  {`{"version":"3.7.0"}`, TaskTypeLLM},
		"missing base model": {`{"version":"3.7.0"}`, TaskTypeSD},
		"component no repo":  {`{"version":"3.7.0","base_model":{"name":"a/b"},"components":{"t":{"format":"gguf"}}}`, TaskTypeSD},
		"finetune task type": {`{"version":"3.7.0","model":{"name":"a/b"}}`, TaskTypeSDFTLora},
	}
	for name, tc := range cases {
		if _, err := DeriveTaskModelRequirements(tc.args, tc.taskType); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestRequirementsEncodeToHashLists(t *testing.T) {
	requirements := mustDerive(t, imageTaskArgs("org/p", "sha", "auto", "",
		`{"t":{"format":"gguf","repo":"org/q","format_kwargs":{"filename":"a.gguf"}}}`, ""), TaskTypeSD)
	decoded, err := DecodeHashList(EncodeHashList(requirements.RequiredModelFileHashes))
	if err != nil || len(decoded) != 1 {
		t.Fatalf("unexpected decode: %v, %v", decoded, err)
	}
	source, err := DecodeHashList(EncodeHashList(requirements.RequiredModelFileSourceHashes))
	if err != nil || len(source) != 1 {
		t.Fatalf("unexpected source decode: %v, %v", source, err)
	}
	if _, err := json.Marshal(requirements.Entries); err != nil {
		t.Fatalf("entries must be JSON encodable: %v", err)
	}
}

func TestRequestedModelCacheConfigHashesToCacheHash(t *testing.T) {
	task := &InferenceTask{TaskType: TaskTypeLLM, TaskArgs: llmTaskArgs("org/llm", "", "auto", "")}
	config, err := task.RequestedModelCacheConfig()
	if err != nil {
		t.Fatal(err)
	}
	requirements := mustDerive(t, task.TaskArgs, TaskTypeLLM)
	if sha256.Sum256(config) != [32]byte(requirements.RequestedModelCacheHash) {
		t.Fatal("cache config must hash to the requested model cache hash")
	}
}
