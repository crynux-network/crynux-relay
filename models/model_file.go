package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ModelSourceTypeHuggingFace = "huggingface"
	// ImageBaseModelFormat is the format of a complete Diffusers pipeline repository.
	ImageBaseModelFormat = "diffusers"

	modelHashSize = sha256.Size
)

// Hash32 is a SHA-256 digest.
type Hash32 [modelHashSize]byte

func (h Hash32) Bytes() []byte {
	return h[:]
}

func (h Hash32) Hex() string {
	return hex.EncodeToString(h[:])
}

func Hash32FromBytes(b []byte) (Hash32, error) {
	var h Hash32
	if len(b) != modelHashSize {
		return h, fmt.Errorf("hash must be %d bytes, got %d", modelHashSize, len(b))
	}
	copy(h[:], b)
	return h, nil
}

// ModelFileEntry is one independently downloadable model source. It carries no
// dtype, task-time quantization, LoRA, component usage name, executor, or GPU.
type ModelFileEntry struct {
	SourceType   string         `json:"source_type"`
	Repository   string         `json:"repository"`
	Revision     string         `json:"revision"`
	Format       string         `json:"format"`
	FormatKwargs map[string]any `json:"format_kwargs"`
}

func (e ModelFileEntry) canonicalObject(withRevision bool) map[string]any {
	formatKwargs := e.FormatKwargs
	if formatKwargs == nil {
		formatKwargs = map[string]any{}
	}
	obj := map[string]any{
		"source_type":   e.SourceType,
		"repository":    e.Repository,
		"format":        e.Format,
		"format_kwargs": formatKwargs,
	}
	if withRevision {
		obj["revision"] = e.Revision
	}
	return obj
}

// CanonicalJSON returns the canonical JSON of the exact entry including its revision.
func (e ModelFileEntry) CanonicalJSON() ([]byte, error) {
	if e.Revision == "" {
		return nil, errors.New("model file entry revision is empty")
	}
	return CanonicalJSON(e.canonicalObject(true))
}

// SourceCanonicalJSON returns the canonical JSON of the entry with the revision removed.
func (e ModelFileEntry) SourceCanonicalJSON() ([]byte, error) {
	return CanonicalJSON(e.canonicalObject(false))
}

// ModelFileHash is the SHA-256 of the exact entry canonical JSON.
func (e ModelFileEntry) ModelFileHash() (Hash32, error) {
	data, err := e.CanonicalJSON()
	if err != nil {
		return Hash32{}, err
	}
	return sha256.Sum256(data), nil
}

// ModelFileSourceHash is the SHA-256 of the entry canonical JSON without the revision.
func (e ModelFileEntry) ModelFileSourceHash() (Hash32, error) {
	data, err := e.SourceCanonicalJSON()
	if err != nil {
		return Hash32{}, err
	}
	return sha256.Sum256(data), nil
}

// CanonicalJSON encodes v as JSON with sorted object keys, no insignificant
// whitespace, and no HTML escaping. Numbers keep their decoded textual form.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	writeCanonicalValue(&buf, generic)
	return buf.Bytes(), nil
}

func writeCanonicalValue(buf *bytes.Buffer, v any) {
	switch value := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(value))
	case json.Number:
		buf.WriteString(value.String())
	case string:
		writeCanonicalString(buf, value)
	case []any:
		buf.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalValue(buf, item)
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, key)
			buf.WriteByte(':')
			writeCanonicalValue(buf, value[key])
		}
		buf.WriteByte('}')
	}
}

func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// SortedUniqueHashes returns the hashes in ascending byte order without duplicates.
func SortedUniqueHashes(hashes []Hash32) []Hash32 {
	sorted := make([]Hash32, len(hashes))
	copy(sorted, hashes)
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i][:], sorted[j][:]) < 0
	})
	unique := sorted[:0]
	for i, h := range sorted {
		if i > 0 && h == sorted[i-1] {
			continue
		}
		unique = append(unique, h)
	}
	return unique
}

// EncodeHashList serializes a hash set as the concatenation of its sorted
// 32-byte members, the format stored in LONGBLOB columns.
func EncodeHashList(hashes []Hash32) []byte {
	unique := SortedUniqueHashes(hashes)
	encoded := make([]byte, 0, len(unique)*modelHashSize)
	for _, h := range unique {
		encoded = append(encoded, h[:]...)
	}
	return encoded
}

// DecodeHashList parses a concatenation of 32-byte hashes.
func DecodeHashList(data []byte) ([]Hash32, error) {
	if len(data)%modelHashSize != 0 {
		return nil, fmt.Errorf("hash list length %d is not a multiple of %d", len(data), modelHashSize)
	}
	hashes := make([]Hash32, 0, len(data)/modelHashSize)
	for offset := 0; offset < len(data); offset += modelHashSize {
		var h Hash32
		copy(h[:], data[offset:offset+modelHashSize])
		hashes = append(hashes, h)
	}
	return hashes, nil
}

// RequiredModelFilesHash identifies a set of exact model files.
func RequiredModelFilesHash(hashes []Hash32) Hash32 {
	return sha256.Sum256(EncodeHashList(hashes))
}

// ModelFile is one row of the model_files catalog.
type ModelFile struct {
	ModelFileHash       []byte    `json:"model_file_hash" gorm:"type:binary(32);primaryKey"`
	ModelFileSourceHash []byte    `json:"model_file_source_hash" gorm:"type:binary(32);not null;index"`
	CanonicalJSON       JSONText  `json:"canonical_json" gorm:"type:json;not null"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// NewModelFile builds the catalog row of an exact entry.
func NewModelFile(entry ModelFileEntry) (*ModelFile, error) {
	hash, err := entry.ModelFileHash()
	if err != nil {
		return nil, err
	}
	sourceHash, err := entry.ModelFileSourceHash()
	if err != nil {
		return nil, err
	}
	canonical, err := entry.CanonicalJSON()
	if err != nil {
		return nil, err
	}
	return &ModelFile{
		ModelFileHash:       hash.Bytes(),
		ModelFileSourceHash: sourceHash.Bytes(),
		CanonicalJSON:       canonical,
	}, nil
}

// UpsertModelFiles inserts catalog rows that do not exist yet. The catalog row
// is immutable because its primary key is the hash of its content.
func UpsertModelFiles(ctx context.Context, db *gorm.DB, files []ModelFile) error {
	if len(files) == 0 {
		return nil
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.WithContext(dbCtx).Clauses(clause.OnConflict{DoNothing: true}).Create(&files).Error
}

// GetModelFiles loads catalog rows by exact hash.
func GetModelFiles(ctx context.Context, db *gorm.DB, hashes []Hash32) ([]ModelFile, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	keys := make([][]byte, 0, len(hashes))
	for _, h := range hashes {
		keys = append(keys, h.Bytes())
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var files []ModelFile
	if err := db.WithContext(dbCtx).Where("model_file_hash IN ?", keys).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

// Entry decodes the catalog row back into its exact model file entry.
func (f *ModelFile) Entry() (ModelFileEntry, error) {
	var entry ModelFileEntry
	if err := json.Unmarshal(f.CanonicalJSON, &entry); err != nil {
		return ModelFileEntry{}, fmt.Errorf("decode model file %x: %w", f.ModelFileHash, err)
	}
	return entry, nil
}
