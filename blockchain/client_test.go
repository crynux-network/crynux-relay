package blockchain_test

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/time/rate"
)

func TestGetErrorMessageFromReceipt(t *testing.T) {
	t.Skip("requires a live RPC endpoint and a known reverted transaction")

	ctx := context.Background()
	client, err := ethclient.Dial("https://json-rpc.base-sepolia.crynux.io")
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	benefitAddressInstance, err := bindings.NewBenefitAddress(common.HexToAddress("0x06aCfA4867C94F97F55De91B257a28480DE8D3b1"), client)
	if err != nil {
		t.Fatalf("Failed to new benefit address instance: %v", err)
	}

	nodeStakingInstance, err := bindings.NewNodeStaking(common.HexToAddress("0xE15b5DD09f9867C8dD0FbC0f57216b440300c99d"), client)
	if err != nil {
		t.Fatalf("Failed to new node staking instance: %v", err)
	}

	creditsInstance, err := bindings.NewCredits(common.HexToAddress("0xB47E277aE7Cbb93949D7202b6e29e33f541EC262"), client)
	if err != nil {
		t.Fatalf("Failed to new credits instance: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, common.HexToAddress("0x56572715E0eb7149a6465870f59ef3fa3d4887C8"))
	if err != nil {
		t.Fatalf("Failed to get nonce: %v", err)
	}

	blockchainClient := &blockchain.BlockchainClient{
		RpcClient:                      client,
		BenefitAddressContractInstance: benefitAddressInstance,
		NodeStakingContractInstance:    nodeStakingInstance,
		CreditsContractInstance:        creditsInstance,
		ChainID:                        big.NewInt(1313161574),
		GasPrice:                       big.NewInt(700000000),
		GasLimit:                       8000000,
		Address:                        "0x56572715E0eb7149a6465870f59ef3fa3d4887C8",
		PrivateKey:                     "0440cb8b2962699e5ce6835170ba86a085d67477e5581e398674a59feb8e7b9c",
		Nonce:                          &nonce,
		NonceMu:                        sync.Mutex{},
		Limiter:                        rate.NewLimiter(rate.Limit(10), int(10)),
		SentTransactionCountLimit:      1,
	}

	txHash := common.HexToHash("0xc27ae2faa27080354fae3f35a7166704eaba12a095eea1610b62b762ae5f0814")
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		t.Fatalf("Failed to get receipt: %v", err)
	}

	errMsg, err := blockchainClient.GetErrorMessageFromReceipt(ctx, receipt)
	if err != nil {
		t.Fatalf("Failed to get error message: %v", err)
	}

	fmt.Println(errMsg)
}

func TestGetTransactionHashesFromBlockUsesHexBlockNumber(t *testing.T) {
	const txHash = "0x00000000000000000000000000000000000000000000000000000000000000aa"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     int               `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Method != "eth_getBlockByNumber" {
			t.Errorf("expected eth_getBlockByNumber, got %s", request.Method)
		}
		if len(request.Params) != 2 {
			t.Errorf("expected two params, got %d", len(request.Params))
		} else {
			var blockNumber string
			var fullTransactions bool
			if err := json.Unmarshal(request.Params[0], &blockNumber); err != nil {
				t.Errorf("failed to decode block number: %v", err)
			}
			if err := json.Unmarshal(request.Params[1], &fullTransactions); err != nil {
				t.Errorf("failed to decode full transaction flag: %v", err)
			}
			if blockNumber != "0xa" {
				t.Errorf("expected hex block number 0xa, got %s", blockNumber)
			}
			if fullTransactions {
				t.Error("expected hash-only transaction enumeration")
			}
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result": map[string]interface{}{
				"transactions": []string{txHash},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	blockchainClient := &blockchain.BlockchainClient{RpcEndpoint: server.URL}
	hashes, err := blockchainClient.GetTransactionHashesFromBlock(context.Background(), big.NewInt(10))
	if err != nil {
		t.Fatalf("GetTransactionHashesFromBlock failed: %v", err)
	}
	if len(hashes) != 1 || hashes[0] != txHash {
		t.Fatalf("expected hash %s, got %#v", txHash, hashes)
	}
}

func TestGetTransactionTransferReadsRawFields(t *testing.T) {
	txHash := common.HexToHash("0x00000000000000000000000000000000000000000000000000000000000000aa")
	fromAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	toAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     int               `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.Method != "eth_getTransactionByHash" {
			t.Errorf("expected eth_getTransactionByHash, got %s", request.Method)
		}
		if len(request.Params) != 1 {
			t.Errorf("expected one param, got %d", len(request.Params))
		} else {
			var requestedHash string
			if err := json.Unmarshal(request.Params[0], &requestedHash); err != nil {
				t.Errorf("failed to decode tx hash: %v", err)
			}
			if requestedHash != txHash.Hex() {
				t.Errorf("expected tx hash %s, got %s", txHash.Hex(), requestedHash)
			}
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"result": map[string]interface{}{
				"hash":  txHash.Hex(),
				"from":  fromAddress.Hex(),
				"to":    toAddress.Hex(),
				"value": "0x7b",
				"input": "0x",
				"type":  "0x7e",
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	blockchainClient := &blockchain.BlockchainClient{RpcEndpoint: server.URL}
	transfer, err := blockchainClient.GetTransactionTransfer(context.Background(), txHash)
	if err != nil {
		t.Fatalf("GetTransactionTransfer failed: %v", err)
	}
	if transfer.Hash != txHash {
		t.Fatalf("expected hash %s, got %s", txHash.Hex(), transfer.Hash.Hex())
	}
	if transfer.From != fromAddress {
		t.Fatalf("expected from address %s, got %s", fromAddress.Hex(), transfer.From.Hex())
	}
	if transfer.To == nil || *transfer.To != toAddress {
		t.Fatalf("expected to address %s, got %v", toAddress.Hex(), transfer.To)
	}
	if transfer.Value.Cmp(big.NewInt(123)) != 0 {
		t.Fatalf("expected value 123, got %s", transfer.Value.String())
	}
	if len(transfer.Input) != 0 {
		t.Fatalf("expected empty input, got %#x", transfer.Input)
	}
}

func TestBuildCallMsgFromTransactionUsesStoredFields(t *testing.T) {
	fromAddress := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	toAddress := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	blockchainClient := &blockchain.BlockchainClient{
		GasLimit: 12345,
		GasPrice: big.NewInt(678),
	}
	transaction := &models.BlockchainTransaction{
		FromAddress: fromAddress.Hex(),
		ToAddress:   toAddress.Hex(),
		Value:       "123",
		Data:        sql.NullString{String: "0x1234", Valid: true},
	}

	msg, err := blockchainClient.BuildCallMsgFromTransaction(transaction)
	if err != nil {
		t.Fatalf("BuildCallMsgFromTransaction failed: %v", err)
	}
	if msg.From != fromAddress {
		t.Fatalf("expected from address %s, got %s", fromAddress.Hex(), msg.From.Hex())
	}
	if msg.To == nil || *msg.To != toAddress {
		t.Fatalf("expected to address %s, got %v", toAddress.Hex(), msg.To)
	}
	if msg.Value.Cmp(big.NewInt(123)) != 0 {
		t.Fatalf("expected value 123, got %s", msg.Value.String())
	}
	if msg.Gas != 12345 {
		t.Fatalf("expected gas 12345, got %d", msg.Gas)
	}
	if msg.GasPrice.Cmp(big.NewInt(678)) != 0 {
		t.Fatalf("expected gas price 678, got %s", msg.GasPrice.String())
	}
	if string(msg.Data) != string([]byte{0x12, 0x34}) {
		t.Fatalf("expected data 0x1234, got %#x", msg.Data)
	}
}

func TestIsUnsupportedTransactionTypeError(t *testing.T) {
	if !blockchain.IsUnsupportedTransactionTypeError(fmt.Errorf("transaction type not supported: 0x7e")) {
		t.Fatal("expected Arbitrum custom transaction type error to be detected")
	}
	if blockchain.IsUnsupportedTransactionTypeError(fmt.Errorf("connection reset by peer")) {
		t.Fatal("expected unrelated RPC error not to be treated as unsupported transaction type")
	}
}
