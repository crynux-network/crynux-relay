package huggingface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	ErrModelNotFound      = errors.New("huggingface model not found")
	ErrRevisionNotFound   = errors.New("huggingface revision not found")
	ErrVariantUnavailable = errors.New("huggingface model variant unavailable")
	ErrServiceUnavailable = errors.New("huggingface service unavailable")
)

type ModelKind string

const (
	ModelKindLLM ModelKind = "llm"
	ModelKindSD  ModelKind = "sd"
	// ModelKindRepository only checks that the repository exists.
	ModelKindRepository ModelKind = "repository"
)

type modelInfoResponse struct {
	Siblings []struct {
		RFilename string `json:"rfilename"`
	} `json:"siblings"`
}

type revisionInfoResponse struct {
	SHA string `json:"sha"`
}

type cacheEntry struct {
	err       error
	sha       string
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
	if entry, ok := c.getCached(cacheKey); ok {
		return entry.err
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

func (c *Client) getCached(key string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *Client) putCached(key string, err error) {
	c.putCachedResult(key, "", err)
}

func (c *Client) putCachedResult(key, sha string, err error) {
	if errors.Is(err, ErrServiceUnavailable) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = cacheEntry{
		err:       err,
		sha:       sha,
		expiresAt: time.Now().Add(c.cacheTTL),
	}
}

// ResolveRevision resolves a branch, tag or commit of a repository to its
// commit SHA.
func (c *Client) ResolveRevision(ctx context.Context, repository, revision string) (string, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return "", ErrModelNotFound
	}
	if revision == "" {
		return "", ErrRevisionNotFound
	}
	cacheKey := "revision|" + repository + "|" + revision
	if entry, ok := c.getCached(cacheKey); ok {
		return entry.sha, entry.err
	}

	var info revisionInfoResponse
	endpoint := c.baseURL + "/api/models/" + repository + "/revision/" + url.PathEscape(revision)
	if err := c.fetchJSON(ctx, endpoint, &info); err != nil {
		c.putCachedResult(cacheKey, "", err)
		return "", err
	}
	if info.SHA == "" {
		return "", ErrServiceUnavailable
	}
	c.putCachedResult(cacheKey, info.SHA, nil)
	return info.SHA, nil
}

func (c *Client) fetchModelInfo(ctx context.Context, modelID string) (*modelInfoResponse, error) {
	var info modelInfoResponse
	if err := c.fetchJSON(ctx, c.baseURL+"/api/models/"+modelID, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// FetchRepoFileJSON downloads one repository file as JSON through the Hub
// resolve URL. revision MUST be a branch, tag, or commit; empty revision uses main.
func (c *Client) FetchRepoFileJSON(ctx context.Context, repository, revision, filePath string, target any) error {
	repository = strings.TrimSpace(repository)
	filePath = strings.Trim(strings.TrimSpace(filePath), "/")
	if repository == "" || filePath == "" {
		return ErrModelNotFound
	}
	if strings.TrimSpace(revision) == "" {
		revision = "main"
	}
	endpoint := c.baseURL + "/" + repository + "/resolve/" + url.PathEscape(revision) + "/" + pathEscapeSegments(filePath)
	return c.fetchJSON(ctx, endpoint, target)
}

func pathEscapeSegments(filePath string) string {
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (c *Client) fetchJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrServiceUnavailable
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ErrServiceUnavailable
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return ErrServiceUnavailable
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		if err := json.Unmarshal(body, target); err != nil {
			return ErrServiceUnavailable
		}
		return nil
	case resp.StatusCode == http.StatusNotFound && resp.Header.Get("X-Error-Code") == "RevisionNotFound":
		return ErrRevisionNotFound
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrModelNotFound
	case resp.StatusCode >= 500:
		return ErrServiceUnavailable
	default:
		return fmt.Errorf("%w: unexpected status %d", ErrModelNotFound, resp.StatusCode)
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
