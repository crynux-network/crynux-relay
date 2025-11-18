package blockchain

import (
	"bytes"
	"context"
	"crynux_relay/blockchain/bindings"
	"crynux_relay/config"
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"sync"
	"time"

	"crynux_relay/models"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

type BlockchainClient struct {
	Network                        string
	RpcClient                      *ethclient.Client
	BenefitAddressContractInstance *bindings.BenefitAddress
	NodeStakingContractInstance    *bindings.NodeStaking
	CreditsContractInstance        *bindings.Credits
	UserStakingContractInstance    *bindings.UserStaking
	ChainID                        *big.Int
	GasPrice                       *big.Int
	GasLimit                       uint64
	Address                        string
	PrivateKey                     string
	Nonce                          *uint64
	NonceMu                        sync.Mutex
	Limiter                        *rate.Limiter
	SentTransactionCountLimit      uint64
}

var blockchainClients = make(map[string]*BlockchainClient)
var pattern *regexp.Regexp = regexp.MustCompile(`(?i)nonce`)
var ErrBlockchainNotFound = errors.New("blockchain not found")

func GetBlockchainClient(network string) (*BlockchainClient, error) {
	client, exists := blockchainClients[network]
	if !exists {
		return nil, ErrBlockchainNotFound
	}
	return client, nil
}

func initBlockchainClient(ctx context.Context, network string) error {
	appConfig := config.GetConfig()
	blockchain, exists := appConfig.Blockchains[network]
	if !exists {
		return ErrBlockchainNotFound
	}

	client, err := ethclient.Dial(blockchain.RpcEndpoint)
	if err != nil {
		return err
	}

	benefitAddressInstance, err := bindings.NewBenefitAddress(common.HexToAddress(blockchain.Contracts.BenefitAddress), client)
	if err != nil {
		return err
	}

	nodeStakingInstance, err := bindings.NewNodeStaking(common.HexToAddress(blockchain.Contracts.NodeStaking), client)
	if err != nil {
		return err
	}

	creditsInstance, err := bindings.NewCredits(common.HexToAddress(blockchain.Contracts.Credits), client)
	if err != nil {
		return err
	}

	userStakingInstance, err := bindings.NewUserStaking(common.HexToAddress(blockchain.Contracts.UserStaking), client)
	if err != nil {
		return err
	}

	gasPrice, err := initSuggestGasPrice(ctx, client, blockchain.GasPrice)
	if err != nil {
		return err
	}

	chainID, err := initChainID(ctx, client, blockchain.ChainID)
	if err != nil {
		return err
	}

	nonce, err := initNonce(ctx, client, blockchain.Account.Address)
	if err != nil {
		return err
	}

	limiter := rate.NewLimiter(rate.Limit(blockchain.RPS), int(blockchain.RPS))

	blockchainClients[network] = &BlockchainClient{
		Network:                        network,
		RpcClient:                      client,
		BenefitAddressContractInstance: benefitAddressInstance,
		NodeStakingContractInstance:    nodeStakingInstance,
		CreditsContractInstance:        creditsInstance,
		UserStakingContractInstance:    userStakingInstance,
		ChainID:                        chainID,
		GasPrice:                       gasPrice,
		GasLimit:                       blockchain.GasLimit,
		Address:                        blockchain.Account.Address,
		PrivateKey:                     blockchain.Account.PrivateKey,
		Nonce:                          &nonce,
		NonceMu:                        sync.Mutex{},
		Limiter:                        limiter,
		SentTransactionCountLimit:      blockchain.SentTransactionCountLimit,
	}
	return nil
}

func Init(ctx context.Context) error {
	appConfig := config.GetConfig()
	for network := range appConfig.Blockchains {
		if err := initBlockchainClient(ctx, network); err != nil {
			return err
		}
	}
	return nil
}

func initSuggestGasPrice(ctx context.Context, client *ethclient.Client, gasPriceNum uint64) (*big.Int, error) {
	var gasPrice *big.Int
	if gasPriceNum > 0 {
		gasPrice = big.NewInt(0).SetUint64(gasPriceNum)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		p, err := client.SuggestGasPrice(callCtx)
		if err != nil {
			return nil, err
		}
		log.Debugln("Estimated gas price from blockchain: " + p.String())
		gasPrice = p
	}
	return gasPrice, nil
}

func initChainID(ctx context.Context, client *ethclient.Client, chainIDNum uint64) (*big.Int, error) {
	var chainID *big.Int
	if chainIDNum > 0 {
		chainID = big.NewInt(0).SetUint64(chainIDNum)
	} else {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		id, err := client.ChainID(callCtx)
		if err != nil {
			return nil, err
		}
		chainID = id
	}
	return chainID, nil
}

func initNonce(ctx context.Context, client *ethclient.Client, address string) (uint64, error) {
	nonce, err := client.PendingNonceAt(ctx, common.HexToAddress(address))
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (client *BlockchainClient) GetNonce(ctx context.Context) (uint64, error) {
	if client.Nonce == nil {
		nonce, err := initNonce(ctx, client.RpcClient, client.Address)
		if err != nil {
			return 0, err
		}
		client.Nonce = &nonce
	}
	return *client.Nonce, nil
}

func (client *BlockchainClient) IncrementNonce() {
	*client.Nonce++
}

func matchNonceError(errStr string) bool {
	res := pattern.FindStringSubmatch(errStr)
	return res != nil
}

func (client *BlockchainClient) processSendingTxError(err error) error {
	if ok := matchNonceError(err.Error()); ok {
		client.Nonce = nil
	}
	return err
}

func (client *BlockchainClient) BalanceAt(ctx context.Context, address common.Address) (*big.Int, error) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return client.RpcClient.BalanceAt(callCtx, address, nil)
}

func (client *BlockchainClient) GetAuth(ctx context.Context) (*bind.TransactOpts, error) {
	privateKey, err := crypto.HexToECDSA(client.PrivateKey)
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, client.ChainID)
	if err != nil {
		return nil, err
	}

	log.Debugln("Set gas limit to:" + strconv.FormatUint(client.GasLimit, 10))

	auth.Value = big.NewInt(0)
	auth.GasLimit = client.GasLimit
	auth.GasPrice = client.GasPrice

	return auth, nil
}

func (client *BlockchainClient) WaitTxReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	deadline, hasDeadline := ctx.Deadline()
	for {
		r, err := func() (*types.Receipt, error) {
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			return client.RpcClient.TransactionReceipt(callCtx, txHash)
		}()
		if err == ethereum.NotFound {
			time.Sleep(time.Second)
			continue
		}
		if hasDeadline && time.Now().Compare(deadline) >= 0 && err == context.DeadlineExceeded {
			log.Errorf("wait receipt of tx %s timeout", txHash.Hex())
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

func (client *BlockchainClient) SendETH(ctx context.Context, to common.Address, amount *big.Int) (*types.Transaction, error) {
	gasLimit := client.GasLimit

	client.NonceMu.Lock()
	defer client.NonceMu.Unlock()

	nonce, err := client.GetNonce(ctx)
	if err != nil {
		return nil, err
	}

	tx := types.NewTransaction(nonce, to, amount, gasLimit, client.GasPrice, nil)

	privateKey, err := crypto.HexToECDSA(client.PrivateKey)
	if err != nil {
		return nil, err
	}

	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(client.ChainID), privateKey)
	if err != nil {
		return nil, err
	}

	if err := client.Limiter.Wait(ctx); err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = client.RpcClient.SendTransaction(callCtx, signedTx)
	if err != nil {
		err = client.processSendingTxError(err)
		return nil, err
	}

	client.IncrementNonce()
	return signedTx, nil
}

// QueueSendETH queues a send ETH transaction to be sent later
func QueueSendETH(ctx context.Context, db *gorm.DB, to common.Address, amount *big.Int, network string) (*models.BlockchainTransaction, error) {
	appConfig := config.GetConfig()
	blockchain, ok := appConfig.Blockchains[network]
	if !ok {
		return nil, ErrBlockchainNotFound
	}

	transaction := &models.BlockchainTransaction{
		Network:     network,
		Type:        "SendETH",
		Status:      models.TransactionStatusPending,
		FromAddress: blockchain.Account.Address,
		ToAddress:   to.Hex(),
		Value:       amount.String(),
		MaxRetries:  blockchain.MaxRetries,
	}

	if err := transaction.Save(ctx, db); err != nil {
		return nil, err
	}

	return transaction, nil
}

func (client *BlockchainClient) GetErrorMessageFromReceipt(ctx context.Context, receipt *types.Receipt) (string, error) {
	ctx1, cancel1 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel1()
	tx, _, err := client.RpcClient.TransactionByHash(ctx1, receipt.TxHash)
	if err != nil {
		return "", err
	}

	msg := ethereum.CallMsg{
		From:     common.HexToAddress(client.Address),
		To:       tx.To(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	}

	blockNumber := big.NewInt(0).Sub(receipt.BlockNumber, big.NewInt(1))

	ctx2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel2()

	res, err := client.RpcClient.CallContract(ctx2, msg, blockNumber)
	if err != nil {
		return "", err
	}

	errMsg, err := unpackError(res)
	if err != nil {
		errMsg = "Unknown tx error" + hexutil.Encode(res)
	}
	return errMsg, err
}

var (
	errorSig     = []byte{0x08, 0xc3, 0x79, 0xa0} // Keccak256("Error(string)")[:4]
	abiString, _ = abi.NewType("string", "", nil)
)

func unpackError(result []byte) (string, error) {
	if len(result) < 4 {
		return "", errors.New("tx result length too short")
	}
	if !bytes.Equal(result[:4], errorSig) {
		return "", errors.New("tx result not of type Error(string)")
	}

	vs, err := abi.Arguments{{Type: abiString}}.UnpackValues(result[4:])
	if err != nil {
		return "", err
	}

	return vs[0].(string), nil
}
