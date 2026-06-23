package admin

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildNodesTokenCSVHeaderUsesConfiguredNetworks(t *testing.T) {
	header := buildNodesTokenCSVHeader([]string{"base", "near"})
	expected := []string{
		"node address",
		"card name",
		"base node on-chain wallet balance CNX",
		"near node on-chain wallet balance CNX",
		"staking CNX",
		"relay account balance CNX",
		"base benefit address balance CNX",
		"near benefit address balance CNX",
	}

	if !reflect.DeepEqual(header, expected) {
		t.Fatalf("expected header %v, got %v", expected, header)
	}
}

func TestBuildNodesTokenCSVRecordUsesConfiguredNetworkAmounts(t *testing.T) {
	row := nodesTokenCSVRow{
		Address:  "0x1",
		CardName: "RTX 4090",
		ChainBalances: map[string]*big.Int{
			"base": cnx(1),
			"near": cnx(2),
		},
		BenefitAddressBalances: map[string]*big.Int{
			"near": cnx(5),
		},
		Staking:             cnx(4),
		RelayAccountBalance: cnx(3),
	}

	record := buildNodesTokenCSVRecord(row, []string{"base", "near"})
	expected := []string{
		"0x1",
		"RTX 4090",
		"1.00",
		"2.00",
		"4.00",
		"3.00",
		"0.00",
		"5.00",
	}

	if !reflect.DeepEqual(record, expected) {
		t.Fatalf("expected record %v, got %v", expected, record)
	}
}

func TestBuildNodesActiveDelegatedStakingCSVRecord(t *testing.T) {
	header := buildNodesActiveDelegatedStakingCSVHeader()
	expectedHeader := []string{
		"delegator address",
		"node address",
		"network",
		"chain staking balance CNX",
		"chain wallet balance CNX",
	}
	if !reflect.DeepEqual(header, expectedHeader) {
		t.Fatalf("expected header %v, got %v", expectedHeader, header)
	}

	record := buildNodesActiveDelegatedStakingCSVRecord(nodesActiveDelegatedStakingCSVRow{
		DelegatorAddress:    "0x1",
		NodeAddress:         "0x2",
		Network:             "base",
		ChainStakingBalance: cnx(7),
		ChainWalletBalance:  cnx(8),
	})
	expectedRecord := []string{
		"0x1",
		"0x2",
		"base",
		"7.00",
		"8.00",
	}
	if !reflect.DeepEqual(record, expectedRecord) {
		t.Fatalf("expected record %v, got %v", expectedRecord, record)
	}
}

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

func cnx(amount int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(amount), big.NewInt(1_000_000_000_000_000_000))
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
