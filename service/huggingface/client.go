package huggingface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	ErrModelNotFound       = errors.New("huggingface model not found")
	ErrVariantUnavailable  = errors.New("huggingface model variant unavailable")
	ErrServiceUnavailable  = errors.New("huggingface service unavailable")
)

type ModelKind string

const (
	ModelKindLLM ModelKind = "llm"
	ModelKindSD  ModelKind = "sd"
)

type modelInfoResponse struct {
	Siblings []struct {
		RFilename string `json:"rfilename"`
	} `json:"siblings"`
}

type cacheEntry struct {
	err       error
	expiresAt time.Time
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cacheTTL   time.Duration

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func NewClient(baseURL string, timeout time.Duration, cacheTTL time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cacheTTL: cacheTTL,
		cache:    make(map[string]cacheEntry),
	}
}

func (c *Client) ValidateBaseModel(ctx context.Context, kind ModelKind, modelID, variant string) error {
	modelID = strings.TrimSpace(modelID)
	variant = strings.ToLower(strings.TrimSpace(variant))
	if modelID == "" {
		return ErrModelNotFound
	}
	cacheKey := string(kind) + "|" + strings.ToLower(modelID) + "|" + variant
	if err, ok := c.getCached(cacheKey); ok {
		return err
	}

	info, err := c.fetchModelInfo(ctx, modelID)
	if err != nil {
		if errors.Is(err, ErrServiceUnavailable) {
			return err
		}
		c.putCached(cacheKey, err)
		return err
	}
	if kind == ModelKindSD {
		if err := validateSDSiblings(info.Siblings, variant); err != nil {
			c.putCached(cacheKey, err)
			return err
		}
	}
	c.putCached(cacheKey, nil)
	return nil
}

func (c *Client) getCached(key string) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.err, true
}

func (c *Client) putCached(key string, err error) {
	if errors.Is(err, ErrServiceUnavailable) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = cacheEntry{
		err:       err,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
}

func (c *Client) fetchModelInfo(ctx context.Context, modelID string) (*modelInfoResponse, error) {
	endpoint := c.baseURL + "/api/models/" + modelID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrServiceUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, ErrServiceUnavailable
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		var info modelInfoResponse
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, ErrServiceUnavailable
		}
		return &info, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrModelNotFound
	case resp.StatusCode >= 500:
		return nil, ErrServiceUnavailable
	default:
		return nil, fmt.Errorf("%w: unexpected status %d", ErrModelNotFound, resp.StatusCode)
	}
}

func validateSDSiblings(siblings []struct {
	RFilename string `json:"rfilename"`
}, variant string) error {
	filenames := make([]string, 0, len(siblings))
	hasModelIndex := false
	for _, sibling := range siblings {
		name := sibling.RFilename
		filenames = append(filenames, name)
		if name == "model_index.json" || strings.HasSuffix(name, "/model_index.json") {
			hasModelIndex = true
		}
	}
	if !hasModelIndex {
		return ErrModelNotFound
	}
	if variant != "" {
		needle := "." + variant + "."
		for _, name := range filenames {
			base := path.Base(name)
			if strings.Contains(base, needle) {
				return nil
			}
		}
		return ErrVariantUnavailable
	}
	for _, name := range filenames {
		base := path.Base(name)
		if isDefaultWeightFile(base) {
			return nil
		}
	}
	return ErrVariantUnavailable
}

func isDefaultWeightFile(filename string) bool {
	lower := strings.ToLower(filename)
	var ext string
	switch {
	case strings.HasSuffix(lower, ".safetensors"):
		ext = ".safetensors"
	case strings.HasSuffix(lower, ".bin"):
		ext = ".bin"
	default:
		return false
	}
	stem := filename[:len(filename)-len(ext)]
	return !strings.Contains(stem, ".")
}
