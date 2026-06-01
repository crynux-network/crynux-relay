package admin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRetryNodesTokenCSVChainRequestSucceedsAfterRetry(t *testing.T) {
	originalWait := nodesTokenCSVChainRequestRetryWait
	nodesTokenCSVChainRequestRetryWait = time.Nanosecond
	defer func() {
		nodesTokenCSVChainRequestRetryWait = originalWait
	}()

	attempts := 0
	result, err := retryNodesTokenCSVChainRequest(context.Background(), "balance", "near", "0x1", func() (string, error) {
		attempts++
		if attempts <= 3 {
			return "", errors.New("temporary rpc error")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result ok, got %q", result)
	}
	if attempts != 4 {
		t.Fatalf("expected 4 attempts, got %d", attempts)
	}
}

func TestRetryNodesTokenCSVChainRequestFailsAfterMaxRetry(t *testing.T) {
	originalWait := nodesTokenCSVChainRequestRetryWait
	nodesTokenCSVChainRequestRetryWait = time.Nanosecond
	defer func() {
		nodesTokenCSVChainRequestRetryWait = originalWait
	}()

	attempts := 0
	_, err := retryNodesTokenCSVChainRequest(context.Background(), "balance", "near", "0x1", func() (string, error) {
		attempts++
		return "", errors.New("temporary rpc error")
	})

	if err == nil {
		t.Fatal("expected retry to fail")
	}
	if attempts != nodesTokenCSVChainRequestMaxRetry+1 {
		t.Fatalf("expected %d attempts, got %d", nodesTokenCSVChainRequestMaxRetry+1, attempts)
	}
	if !strings.Contains(err.Error(), "failed after 10 retries") {
		t.Fatalf("expected max retry error, got: %v", err)
	}
}

func TestRetryNodesTokenCSVChainRequestStopsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	_, err := retryNodesTokenCSVChainRequest(ctx, "balance", "near", "0x1", func() (string, error) {
		attempts++
		return "", errors.New("temporary rpc error")
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}
