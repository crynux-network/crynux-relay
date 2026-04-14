package blockchain_test

import (
	"context"
	"crynux_relay/blockchain"
	"crynux_relay/blockchain/bindings"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/time/rate"
)

func TestGetErrorMessageFromReceipt(t *testing.T) {
	ctx := context.Background()
	client, err := ethclient.Dial("https://json-rpc.testnet-near.crynux.io")
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
