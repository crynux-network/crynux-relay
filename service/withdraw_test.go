package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"math/big"
	"testing"
	"time"
)

func TestWithdrawPersistsAuthorizationWithRecord(t *testing.T) {
	initServiceTestConfig(t)
	appConfig := config.GetConfig()
	appConfig.Blockchains = make(map[string]config.SystemBlockchainConfig)
	appConfig.Blockchains["testnet"] = config.SystemBlockchainConfig{
		WithdrawalMin:        0,
		MaxWithdrawalsPerDay: 10,
	}

	db := config.GetDB()
	if err := db.AutoMigrate(&models.RelayAccount{}, &models.RelayAccountEvent{}, &models.WithdrawRecord{}); err != nil {
		t.Fatalf("migrate withdrawal tables: %v", err)
	}
	address := "0x1234567890123456789012345678901234567890"
	if err := db.Create(&models.RelayAccount{
		Address: address,
		Balance: models.BigInt{Int: *big.NewInt(100)},
	}).Error; err != nil {
		t.Fatalf("create relay account: %v", err)
	}

	timestamp := time.Now().Unix()
	record, err := Withdraw(
		context.Background(),
		db,
		address,
		address,
		big.NewInt(10),
		"testnet",
		timestamp,
		"0xsigned",
	)
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}

	var saved models.WithdrawRecord
	if err := db.First(&saved, record.ID).Error; err != nil {
		t.Fatalf("load withdrawal: %v", err)
	}
	if !saved.Timestamp.Valid || saved.Timestamp.Int64 != timestamp {
		t.Fatalf("timestamp was not persisted: %#v", saved.Timestamp)
	}
	if !saved.Signature.Valid || saved.Signature.String != "0xsigned" {
		t.Fatalf("signature was not persisted: %#v", saved.Signature)
	}
	if saved.RelayAccountEventID == 0 {
		t.Fatal("withdrawal event was not persisted with authorization")
	}
}
