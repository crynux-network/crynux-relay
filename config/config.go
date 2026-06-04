package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
)

var appConfig *AppConfig

// InitConfig Init is an exported method that takes the config from the config file
// and unmarshal it into AppConfig struct
func InitConfig(configPath string) error {
	v := viper.New()
	v.SetConfigType("yml")
	v.SetConfigName("config")

	if configPath != "" {
		v.AddConfigPath(configPath)
	} else {
		v.AddConfigPath("/app/config")
		v.AddConfigPath("config")
	}

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	appConfig = &AppConfig{}

	if err := v.Unmarshal(appConfig); err != nil {
		return err
	}

	if appConfig.Environment == EnvTest {
		privKey := GetTestPrivateKey()
		for network := range appConfig.Blockchains {
			blockchain := appConfig.Blockchains[network]
			blockchain.Account.PrivateKey = privKey
			appConfig.Blockchains[network] = blockchain
		}
		appConfig.Http.JWT.SecretKey = GetTestJWTKey()
		appConfig.MAC.SecretKey = GetTestTaskFeeMACKey()
	} else {
		// Load hard-coded private key
		for network := range appConfig.Blockchains {
			blockchain := appConfig.Blockchains[network]
			blockchain.Account.PrivateKey = ReadFromFile(blockchain.Account.PrivateKeyFile)
			appConfig.Blockchains[network] = blockchain
		}
		appConfig.Http.JWT.SecretKey = ReadFromFile(appConfig.Http.JWT.SecretKeyFile)
		appConfig.MAC.SecretKey = ReadFromFile(appConfig.MAC.SecretKeyFile)
	}
	if err := checkBlockchainAccount(); err != nil {
		return err
	}

	return nil
}

func checkBlockchainAccount() error {

	for network, blockchain := range appConfig.Blockchains {
		blockchain.Account.PrivateKey = NormalizePrivateKey(blockchain.Account.PrivateKey)
		appConfig.Blockchains[network] = blockchain

		if blockchain.Account.PrivateKey == "" {
			return errors.New("blockchain account private key not set")
		}

		if blockchain.Account.Address == "" {
			return errors.New("blockchain account address not set")
		}

		// Check private key and address
		privateKey, err := crypto.HexToECDSA(blockchain.Account.PrivateKey)
		if err != nil {
			return err
		}

		publicKey := privateKey.Public()

		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("error casting public key to ECDSA")
		}

		address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

		if address != blockchain.Account.Address {
			return errors.New("account address and private key mismatch")
		}
	}

	return nil
}

func NormalizePrivateKey(privateKey string) string {
	privateKey = strings.TrimSpace(privateKey)
	if len(privateKey) >= 2 && strings.EqualFold(privateKey[:2], "0x") {
		return privateKey[2:]
	}
	return privateKey
}

func ReadFromFile(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(b))
}

func DeleteBlockchainPrivateKeyFilesAfterRead() error {
	if appConfig == nil {
		return nil
	}

	var files []string
	for _, blockchain := range appConfig.Blockchains {
		files = append(files, blockchain.Account.PrivateKeyFile)
	}

	deletedFiles := make(map[string]struct{}, len(files))
	for _, file := range files {
		if _, ok := deletedFiles[file]; ok {
			continue
		}
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("delete blockchain private key file %s: %w", file, err)
		}
		deletedFiles[file] = struct{}{}
	}
	return nil
}

func GetTestPrivateKey() string {
	return ""
}

func GetTestJWTKey() string {
	return ""
}

func GetTestTaskFeeMACKey() string {
	return ""
}

func GetConfig() *AppConfig {
	return appConfig
}
