package models

import (
	"bytes"
	"encoding/json"
	"testing"
)

func mustHash(t *testing.T) func(Hash32, error) Hash32 {
	return func(h Hash32, err error) Hash32 {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return h
	}
}

func entryFromJSON(t *testing.T, raw string) ModelFileEntry {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.UseNumber()
	var entry ModelFileEntry
	if err := decoder.Decode(&entry); err != nil {
		t.Fatalf("failed to decode entry: %v", err)
	}
	return entry
}

func TestModelFileHashIgnoresObjectKeyOrder(t *testing.T) {
	must := mustHash(t)
	a := entryFromJSON(t, `{"source_type":"huggingface","repository":"org/repo","revision":"abc","format":"gguf","format_kwargs":{"filename":"m.gguf","nested":{"x":1,"y":2}}}`)
	b := entryFromJSON(t, `{"format_kwargs":{"nested":{"y":2,"x":1},"filename":"m.gguf"},"format":"gguf","revision":"abc","repository":"org/repo","source_type":"huggingface"}`)

	if must(a.ModelFileHash()) != must(b.ModelFileHash()) {
		t.Fatal("model_file_hash depends on key order")
	}
	if must(a.ModelFileSourceHash()) != must(b.ModelFileSourceHash()) {
		t.Fatal("model_file_source_hash depends on key order")
	}
}

func TestModelFileHashIgnoresRepositoryCase(t *testing.T) {
	must := mustHash(t)
	mixed := ModelFileEntry{
		SourceType:   ModelSourceTypeHuggingFace,
		Repository:   "Unsloth/Qwen-Image-2.1-GGUF",
		Revision:     "abc",
		Format:       "gguf",
		FormatKwargs: map[string]any{"filename": "qwen-image-2.1-Q4_K_M.gguf"},
	}
	lower := mixed
	lower.Repository = "unsloth/qwen-image-2.1-gguf"
	if must(mixed.ModelFileHash()) != must(lower.ModelFileHash()) {
		t.Fatal("model_file_hash must ignore repository letter case")
	}
	if must(mixed.ModelFileSourceHash()) != must(lower.ModelFileSourceHash()) {
		t.Fatal("model_file_source_hash must ignore repository letter case")
	}
	file, err := NewModelFile(mixed)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := file.Entry()
	if err != nil {
		t.Fatal(err)
	}
	if entry.Repository != "unsloth/qwen-image-2.1-gguf" {
		t.Fatalf("catalog repository = %q, want lowercase", entry.Repository)
	}
}

func TestModelFileCanonicalJSONIsCompactAndSorted(t *testing.T) {
	entry := ModelFileEntry{
		SourceType:   ModelSourceTypeHuggingFace,
		Repository:   "org/repo",
		Revision:     "abc",
		Format:       "gguf",
		FormatKwargs: map[string]any{"filename": "m<&>.gguf"},
	}
	got, err := entry.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"format":"gguf","format_kwargs":{"filename":"m<&>.gguf"},"repository":"org/repo","revision":"abc","source_type":"huggingface"}`
	if string(got) != want {
		t.Fatalf("unexpected canonical json:\n got %s\nwant %s", got, want)
	}
	source, err := entry.SourceCanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	wantSource := `{"format":"gguf","format_kwargs":{"filename":"m<&>.gguf"},"repository":"org/repo","source_type":"huggingface"}`
	if string(source) != wantSource {
		t.Fatalf("unexpected source canonical json:\n got %s\nwant %s", source, wantSource)
	}
}

func TestModelFileRevisionChangesFileHashButNotSourceHash(t *testing.T) {
	must := mustHash(t)
	base := ModelFileEntry{SourceType: ModelSourceTypeHuggingFace, Repository: "org/repo", Revision: "rev1"}
	changed := base
	changed.Revision = "rev2"

	if must(base.ModelFileHash()) == must(changed.ModelFileHash()) {
		t.Fatal("revision change must alter model_file_hash")
	}
	if must(base.ModelFileSourceHash()) != must(changed.ModelFileSourceHash()) {
		t.Fatal("revision change must not alter model_file_source_hash")
	}
}

func TestModelFileFormatChangesAlterHashes(t *testing.T) {
	must := mustHash(t)
	base := ModelFileEntry{
		SourceType:   ModelSourceTypeHuggingFace,
		Repository:   "org/repo",
		Revision:     "rev1",
		Format:       "gguf",
		FormatKwargs: map[string]any{"filename": "a.gguf"},
	}
	variants := map[string]ModelFileEntry{
		"format":   {SourceType: base.SourceType, Repository: base.Repository, Revision: base.Revision, Format: "safetensors", FormatKwargs: base.FormatKwargs},
		"filename": {SourceType: base.SourceType, Repository: base.Repository, Revision: base.Revision, Format: base.Format, FormatKwargs: map[string]any{"filename": "b.gguf"}},
		"kwargs":   {SourceType: base.SourceType, Repository: base.Repository, Revision: base.Revision, Format: base.Format, FormatKwargs: map[string]any{"filename": "a.gguf", "extra": true}},
	}
	for name, variant := range variants {
		if must(base.ModelFileHash()) == must(variant.ModelFileHash()) {
			t.Fatalf("%s change must alter model_file_hash", name)
		}
		if must(base.ModelFileSourceHash()) == must(variant.ModelFileSourceHash()) {
			t.Fatalf("%s change must alter model_file_source_hash", name)
		}
	}
}

func TestModelFileHashRequiresRevision(t *testing.T) {
	entry := ModelFileEntry{SourceType: ModelSourceTypeHuggingFace, Repository: "org/repo"}
	if _, err := entry.ModelFileHash(); err == nil {
		t.Fatal("expected error for exact hash without revision")
	}
	if _, err := entry.ModelFileSourceHash(); err != nil {
		t.Fatalf("source hash must not require revision: %v", err)
	}
}

func TestNilAndEmptyFormatKwargsHashTheSame(t *testing.T) {
	must := mustHash(t)
	a := ModelFileEntry{SourceType: ModelSourceTypeHuggingFace, Repository: "org/repo", Revision: "r"}
	b := a
	b.FormatKwargs = map[string]any{}
	if must(a.ModelFileHash()) != must(b.ModelFileHash()) {
		t.Fatal("nil and empty format_kwargs must hash the same")
	}
}

func TestHashListEncodeDecode(t *testing.T) {
	a, b := Hash32{1}, Hash32{2}
	encoded := EncodeHashList([]Hash32{b, a, b})
	if len(encoded) != 64 {
		t.Fatalf("expected two unique hashes, got %d bytes", len(encoded))
	}
	if !bytes.Equal(encoded[:32], a.Bytes()) || !bytes.Equal(encoded[32:], b.Bytes()) {
		t.Fatal("hash list must be sorted ascending")
	}
	decoded, err := DecodeHashList(encoded)
	if err != nil || len(decoded) != 2 || decoded[0] != a || decoded[1] != b {
		t.Fatalf("unexpected decode result: %v, %v", decoded, err)
	}
	if _, err := DecodeHashList(make([]byte, 33)); err == nil {
		t.Fatal("expected error for invalid length")
	}
	if len(EncodeHashList(nil)) != 0 {
		t.Fatal("empty list must encode to no bytes")
	}
}

func TestNewModelFileMatchesEntryHashes(t *testing.T) {
	must := mustHash(t)
	entry := ModelFileEntry{SourceType: ModelSourceTypeHuggingFace, Repository: "org/repo", Revision: "r1"}
	file, err := NewModelFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(file.ModelFileHash, must(entry.ModelFileHash()).Bytes()) {
		t.Fatal("catalog hash mismatch")
	}
	if !bytes.Equal(file.ModelFileSourceHash, must(entry.ModelFileSourceHash()).Bytes()) {
		t.Fatal("catalog source hash mismatch")
	}
	if !json.Valid(file.CanonicalJSON) {
		t.Fatal("catalog canonical json is not valid json")
	}
}
