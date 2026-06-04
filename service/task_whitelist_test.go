package service

import (
	"context"
	"crynux_relay/models"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTaskWhitelistTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.TaskWhitelist{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestNormalizeTaskWhitelistAddress(t *testing.T) {
	address, err := NormalizeTaskWhitelistAddress("0x00000000000000000000000000000000000000aa")
	if err != nil {
		t.Fatalf("normalize should succeed: %v", err)
	}
	if address != "0x00000000000000000000000000000000000000AA" {
		t.Fatalf("unexpected normalized address: %s", address)
	}

	_, err = NormalizeTaskWhitelistAddress("invalid-address")
	if !errors.Is(err, ErrInvalidTaskWhitelistAddress) {
		t.Fatalf("expected ErrInvalidTaskWhitelistAddress, got %v", err)
	}
}

func TestTaskWhitelistAddListDelete(t *testing.T) {
	resetTaskWhitelistCacheForTest()

	ctx := context.Background()
	db := newTaskWhitelistTestDB(t)
	address := "0x00000000000000000000000000000000000000aa"
	normalized := "0x00000000000000000000000000000000000000AA"

	if err := AddTaskWhitelistAddress(ctx, db, address); err != nil {
		t.Fatalf("add should succeed: %v", err)
	}

	cacheAddresses := getTaskWhitelistCacheSnapshotForTest()
	if len(cacheAddresses) != 1 || cacheAddresses[0] != normalized {
		t.Fatalf("unexpected cache after add: %#v", cacheAddresses)
	}

	if err := AddTaskWhitelistAddress(ctx, db, address); !errors.Is(err, ErrTaskWhitelistAddressExists) {
		t.Fatalf("expected ErrTaskWhitelistAddressExists, got %v", err)
	}

	addresses, err := ListTaskWhitelistAddresses(ctx, db)
	if err != nil {
		t.Fatalf("list should succeed: %v", err)
	}
	if len(addresses) != 1 || addresses[0] != normalized {
		t.Fatalf("unexpected list result: %#v", addresses)
	}

	if err := DeleteTaskWhitelistAddress(ctx, db, "0x00000000000000000000000000000000000000bb"); !errors.Is(err, ErrTaskWhitelistAddressMissing) {
		t.Fatalf("expected ErrTaskWhitelistAddressMissing, got %v", err)
	}

	if err := DeleteTaskWhitelistAddress(ctx, db, address); err != nil {
		t.Fatalf("delete should succeed: %v", err)
	}

	cacheAddresses = getTaskWhitelistCacheSnapshotForTest()
	if len(cacheAddresses) != 0 {
		t.Fatalf("cache should be empty after delete: %#v", cacheAddresses)
	}

	if err := AddTaskWhitelistAddress(ctx, db, address); err != nil {
		t.Fatalf("add after delete should succeed: %v", err)
	}

	addresses, err = ListTaskWhitelistAddresses(ctx, db)
	if err != nil {
		t.Fatalf("list after re-add should succeed: %v", err)
	}
	if len(addresses) != 1 || addresses[0] != normalized {
		t.Fatalf("unexpected list result after re-add: %#v", addresses)
	}
}

func TestTaskWhitelistLazyLoadAndRefresh(t *testing.T) {
	resetTaskWhitelistCacheForTest()

	ctx := context.Background()
	db := newTaskWhitelistTestDB(t)
	address := "0x00000000000000000000000000000000000000AA"

	if err := db.Create(&models.TaskWhitelist{Address: address}).Error; err != nil {
		t.Fatalf("seed create failed: %v", err)
	}

	allowed, err := IsTaskCreatorWhitelisted(ctx, db, address)
	if err != nil {
		t.Fatalf("first whitelist check failed: %v", err)
	}
	if !allowed {
		t.Fatal("address should be allowed after lazy load")
	}

	if err := db.Where("address = ?", address).Delete(&models.TaskWhitelist{}).Error; err != nil {
		t.Fatalf("direct db delete failed: %v", err)
	}

	allowed, err = IsTaskCreatorWhitelisted(ctx, db, address)
	if err != nil {
		t.Fatalf("second whitelist check failed: %v", err)
	}
	if !allowed {
		t.Fatal("address should still be allowed from cache before refresh")
	}

	if err := RefreshTaskWhitelistCache(ctx, db); err != nil {
		t.Fatalf("refresh should succeed: %v", err)
	}

	allowed, err = IsTaskCreatorWhitelisted(ctx, db, address)
	if err != nil {
		t.Fatalf("third whitelist check failed: %v", err)
	}
	if allowed {
		t.Fatal("address should not be allowed after cache refresh")
	}
}
