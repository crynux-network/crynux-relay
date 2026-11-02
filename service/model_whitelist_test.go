package service

import (
	"context"
	"crynux_relay/config"
	"crynux_relay/models"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newModelWhitelistTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelWhitelist{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestNormalizeModelWhitelistName(t *testing.T) {
	name, err := NormalizeModelWhitelistName("Qwen/Qwen3.5-9B")
	if err != nil {
		t.Fatalf("normalize should succeed: %v", err)
	}
	if name != "qwen/qwen3.5-9b" {
		t.Fatalf("unexpected normalized name: %s", name)
	}

	wildcard, err := NormalizeModelWhitelistName("Qwen/*")
	if err != nil {
		t.Fatalf("wildcard normalize should succeed: %v", err)
	}
	if wildcard != "qwen/*" {
		t.Fatalf("unexpected normalized wildcard: %s", wildcard)
	}

	_, err = NormalizeModelWhitelistName("   ")
	if !errors.Is(err, ErrInvalidModelWhitelistName) {
		t.Fatalf("expected ErrInvalidModelWhitelistName, got %v", err)
	}

	for _, invalid := range []string{"*", "*/repo", "qwen/repo*", "qwen/*/extra", "qw*en/*", "https://example.com/*"} {
		if _, err := NormalizeModelWhitelistName(invalid); !errors.Is(err, ErrInvalidModelWhitelistName) {
			t.Fatalf("expected ErrInvalidModelWhitelistName for %q, got %v", invalid, err)
		}
	}
}

func TestModelWhitelistOrganizationWildcard(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)

	if err := AddModelWhitelistName(ctx, db, "Qwen/*"); err != nil {
		t.Fatalf("add wildcard: %v", err)
	}
	if err := AddModelWhitelistName(ctx, db, "deepseek-ai/DeepSeek-R1"); err != nil {
		t.Fatalf("add exact: %v", err)
	}

	cacheNames := getModelWhitelistCacheSnapshotForTest()
	wantCache := []string{"deepseek-ai/deepseek-r1", "qwen/*"}
	if len(cacheNames) != len(wantCache) {
		t.Fatalf("unexpected cache: %#v", cacheNames)
	}
	for i := range wantCache {
		if cacheNames[i] != wantCache[i] {
			t.Fatalf("unexpected cache: %#v", cacheNames)
		}
	}

	mustAllowed := []string{"qwen/qwen3.5-9b", "Qwen/Qwen-Image-2.1", "deepseek-ai/DeepSeek-R1"}
	for _, name := range mustAllowed {
		ok, err := IsModelWhitelisted(name)
		if err != nil {
			t.Fatalf("whitelist check for %s: %v", name, err)
		}
		if !ok {
			t.Fatalf("expected %s to be allowed", name)
		}
	}

	mustRejected := []string{"qwen2/model", "meta-llama/Llama-3.1-8B", "https://example.com/model"}
	for _, name := range mustRejected {
		ok, err := IsModelWhitelisted(name)
		if err != nil {
			t.Fatalf("whitelist check for %s: %v", name, err)
		}
		if ok {
			t.Fatalf("expected %s to be rejected", name)
		}
	}

	if err := DeleteModelWhitelistName(ctx, db, "qwen/qwen3.5-9b"); !errors.Is(err, ErrModelWhitelistNameMissing) {
		t.Fatalf("deleting exact repo must not remove organization wildcard: %v", err)
	}
	ok, err := IsModelWhitelisted("qwen/still-allowed")
	if err != nil {
		t.Fatalf("whitelist check after exact delete attempt: %v", err)
	}
	if !ok {
		t.Fatal("organization wildcard should still allow qwen models")
	}

	if err := DeleteModelWhitelistName(ctx, db, "Qwen/*"); err != nil {
		t.Fatalf("delete wildcard: %v", err)
	}
	ok, err = IsModelWhitelisted("qwen/qwen3.5-9b")
	if err != nil {
		t.Fatalf("whitelist check after wildcard delete: %v", err)
	}
	if ok {
		t.Fatal("qwen models should be rejected after wildcard delete")
	}
	ok, err = IsModelWhitelisted("deepseek-ai/DeepSeek-R1")
	if err != nil {
		t.Fatalf("exact check after wildcard delete: %v", err)
	}
	if !ok {
		t.Fatal("exact entry should remain after wildcard delete")
	}
}

func TestCheckTaskModelsWhitelistedAllowsOrganizationWildcard(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/*"); err != nil {
		t.Fatalf("add wildcard: %v", err)
	}

	llmTask := &models.InferenceTask{
		TaskType:  models.TaskTypeLLM,
		ModelName: "Qwen/Qwen3.5-9B",
	}
	if err := CheckTaskModelsWhitelisted(llmTask); err != nil {
		t.Fatalf("LLM task should be allowed: %v", err)
	}

	sdTask := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"version":"3.7.0","base_model":{"name":"qwen/qwen-image-2.1"},"components":{"vae":{"repo":"qwen/qwen-image-vae","format":"diffusers"}},"dtype":"float16","prompt":"a cat"}`,
	}
	if err := CheckTaskModelsWhitelisted(sdTask); err != nil {
		t.Fatalf("SD task with organization components should be allowed: %v", err)
	}

	sdTaskMixed := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"version":"3.7.0","base_model":{"name":"qwen/qwen-image-2.1"},"components":{"vae":{"repo":"evil/bad-vae","format":"diffusers"}},"dtype":"float16","prompt":"a cat"}`,
	}
	if err := CheckTaskModelsWhitelisted(sdTaskMixed); !errors.Is(err, ErrModelNotWhitelisted) {
		t.Fatalf("expected ErrModelNotWhitelisted for non-org component, got %v", err)
	}
}

func TestInitModelWhitelistCacheRejectsInvalidEntry(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)
	if err := db.Create(&models.ModelWhitelist{ModelName: "qwen/repo*"}).Error; err != nil {
		t.Fatalf("seed create failed: %v", err)
	}
	if err := InitModelWhitelistCache(ctx, db); err == nil {
		t.Fatal("expected init to reject invalid stored entry")
	}
}

func TestModelWhitelistAddListDeleteAndCache(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)

	if err := AddModelWhitelistName(ctx, db, "Qwen/Qwen3.5-9B"); err != nil {
		t.Fatalf("add should succeed: %v", err)
	}

	cacheNames := getModelWhitelistCacheSnapshotForTest()
	if len(cacheNames) != 1 || cacheNames[0] != "qwen/qwen3.5-9b" {
		t.Fatalf("unexpected cache after add: %#v", cacheNames)
	}

	if err := AddModelWhitelistName(ctx, db, "qwen/qwen3.5-9b"); !errors.Is(err, ErrModelWhitelistNameExists) {
		t.Fatalf("expected ErrModelWhitelistNameExists, got %v", err)
	}

	names, err := ListModelWhitelistNames(ctx, db)
	if err != nil {
		t.Fatalf("list should succeed: %v", err)
	}
	if len(names) != 1 || names[0] != "qwen/qwen3.5-9b" {
		t.Fatalf("unexpected list result: %#v", names)
	}

	ok, err := IsModelWhitelisted("Qwen/Qwen3.5-9B")
	if err != nil {
		t.Fatalf("whitelist check failed: %v", err)
	}
	if !ok {
		t.Fatal("model should be allowed")
	}

	if err := DeleteModelWhitelistName(ctx, db, "missing/model"); !errors.Is(err, ErrModelWhitelistNameMissing) {
		t.Fatalf("expected ErrModelWhitelistNameMissing, got %v", err)
	}

	if err := DeleteModelWhitelistName(ctx, db, "Qwen/Qwen3.5-9B"); err != nil {
		t.Fatalf("delete should succeed: %v", err)
	}

	cacheNames = getModelWhitelistCacheSnapshotForTest()
	if len(cacheNames) != 0 {
		t.Fatalf("cache should be empty after delete: %#v", cacheNames)
	}
}

func TestIsModelWhitelistedRequiresInit(t *testing.T) {
	resetModelWhitelistCacheForTest()
	_, err := IsModelWhitelisted("qwen/qwen3.5-9b")
	if !errors.Is(err, ErrModelWhitelistNotInitialized) {
		t.Fatalf("expected ErrModelWhitelistNotInitialized, got %v", err)
	}
}

func TestInitModelWhitelistCacheLoadsFromDB(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)
	if err := db.Create(&models.ModelWhitelist{ModelName: "qwen/qwen3.5-9b"}).Error; err != nil {
		t.Fatalf("seed create failed: %v", err)
	}
	if err := InitModelWhitelistCache(ctx, db); err != nil {
		t.Fatalf("init should succeed: %v", err)
	}
	ok, err := IsModelWhitelisted("qwen/qwen3.5-9b")
	if err != nil {
		t.Fatalf("whitelist check failed: %v", err)
	}
	if !ok {
		t.Fatal("seeded model should be allowed after init")
	}
}

func TestTaskModelNamesForWhitelistIncludesSDComponents(t *testing.T) {
	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"version":"3.7.0","base_model":{"name":"Qwen/Qwen-Image-2.1"},"components":{"vae":{"repo":"Evil/Bad-VAE","format":"diffusers"},"text":{"repo":"Org/TE","format":"diffusers"}},"dtype":"float16","prompt":"a cat"}`,
	}
	names, err := taskModelNamesForWhitelist(task)
	if err != nil {
		t.Fatalf("taskModelNamesForWhitelist: %v", err)
	}
	want := []string{"evil/bad-vae", "org/te", "qwen/qwen-image-2.1"}
	if len(names) != len(want) {
		t.Fatalf("unexpected names: %#v", names)
	}
	got := append([]string(nil), names...)
	sort.Strings(got)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected names: %#v", names)
		}
	}
}

func TestCheckTaskModelsWhitelistedRejectsNonWhitelistedComponent(t *testing.T) {
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := newModelWhitelistTestDB(t)
	if err := AddModelWhitelistName(ctx, db, "qwen/qwen-image-2.1"); err != nil {
		t.Fatalf("add base: %v", err)
	}

	task := &models.InferenceTask{
		TaskType: models.TaskTypeSD,
		TaskArgs: `{"version":"3.7.0","base_model":{"name":"qwen/qwen-image-2.1"},"components":{"vae":{"repo":"evil/bad-vae","format":"diffusers"}},"dtype":"float16","prompt":"a cat"}`,
	}
	if err := CheckTaskModelsWhitelisted(task); !errors.Is(err, ErrModelNotWhitelisted) {
		t.Fatalf("expected ErrModelNotWhitelisted, got %v", err)
	}

	if err := AddModelWhitelistName(ctx, db, "evil/bad-vae"); err != nil {
		t.Fatalf("add component: %v", err)
	}
	if err := CheckTaskModelsWhitelisted(task); err != nil {
		t.Fatalf("expected allow when base and component are whitelisted: %v", err)
	}
}

func TestCreateTaskRejectsNonWhitelistedModel(t *testing.T) {
	initTaskPricingTestStore(t)
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(&models.ModelWhitelist{}); err != nil {
		t.Fatalf("migrate model whitelist: %v", err)
	}
	if err := InitModelWhitelistCache(ctx, db); err != nil {
		t.Fatalf("init model whitelist: %v", err)
	}
	config.GetConfig().Task.ModelWhitelistEnabled = true
	t.Cleanup(func() { config.GetConfig().Task.ModelWhitelistEnabled = false })

	fee := big.NewInt(1_000_000_000)
	task := &models.InferenceTask{
		TaskIDCommitment: "0xmodel-whitelist-reject",
		Creator:          "0xcreator",
		TaskType:         models.TaskTypeLLM,
		TaskArgs:         llmTaskArgs("pearl-ai/bad-model", ""),
		MinVRAM:          16,
		TaskFee:          models.BigInt{Int: *fee},
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}
	if err := CreateTask(ctx, db, task); !errors.Is(err, ErrModelNotWhitelisted) {
		t.Fatalf("expected ErrModelNotWhitelisted, got %v", err)
	}
}

func TestCreateTaskAllowsWhitelistedModel(t *testing.T) {
	initTaskPricingTestStore(t)
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(
		&models.ModelWhitelist{},
		&models.InferenceTask{},
		&models.NetworkTaskNumber{},
		&models.RelayAccountEvent{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if err := AddModelWhitelistName(ctx, db, "qwen/qwen3.5-9b"); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}
	config.GetConfig().Task.ModelWhitelistEnabled = true
	t.Cleanup(func() { config.GetConfig().Task.ModelWhitelistEnabled = false })

	creator := "0xcreator"
	fee := big.NewInt(1_000_000_000)
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0).Set(fee)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})

	task := &models.InferenceTask{
		TaskIDCommitment: "0xmodel-whitelist-allow",
		Creator:          creator,
		TaskType:         models.TaskTypeLLM,
		TaskArgs:         llmTaskArgs("Qwen/Qwen3.5-9B", ""),
		MinVRAM:          16,
		TaskFee:          models.BigInt{Int: *fee},
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}
	setTaskModelRequirements(t, task)
	if err := CreateTask(ctx, db, task); err != nil {
		t.Fatalf("create should succeed: %v", err)
	}
	if task.Status != models.TaskQueued {
		t.Fatalf("expected queued status, got %d", task.Status)
	}
	if task.ModelName != "qwen/qwen3.5-9b" {
		t.Fatalf("unexpected model name: %s", task.ModelName)
	}
}

func TestCreateTaskSkipsWhitelistWhenDisabled(t *testing.T) {
	initTaskPricingTestStore(t)
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(
		&models.ModelWhitelist{},
		&models.InferenceTask{},
		&models.NetworkTaskNumber{},
		&models.RelayAccountEvent{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if err := InitModelWhitelistCache(ctx, db); err != nil {
		t.Fatalf("init model whitelist: %v", err)
	}
	config.GetConfig().Task.ModelWhitelistEnabled = false

	creator := "0xcreator"
	fee := big.NewInt(1_000_000_000)
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0).Set(fee)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})

	task := &models.InferenceTask{
		TaskIDCommitment: "0xmodel-whitelist-disabled",
		Creator:          creator,
		TaskType:         models.TaskTypeLLM,
		TaskArgs:         llmTaskArgs("pearl-ai/bad-model", ""),
		MinVRAM:          16,
		TaskFee:          models.BigInt{Int: *fee},
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}
	setTaskModelRequirements(t, task)
	if err := CreateTask(ctx, db, task); err != nil {
		t.Fatalf("create should succeed when whitelist disabled: %v", err)
	}
}

func TestRunMatchingRoundAbortsNonWhitelistedComponent(t *testing.T) {
	initMatchingTestConfig(t)
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(
		&models.ModelWhitelist{},
		&models.InferenceTask{},
		&models.RelayAccountEvent{},
		&models.Event{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if err := AddModelWhitelistName(ctx, db, "qwen/qwen-image-2.1"); err != nil {
		t.Fatalf("add base whitelist: %v", err)
	}
	config.GetConfig().Task.ModelWhitelistEnabled = true
	t.Cleanup(func() { config.GetConfig().Task.ModelWhitelistEnabled = false })

	creator := "0xcreator"
	fee := big.NewInt(100)
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})

	task := &models.InferenceTask{
		TaskIDCommitment: "0xmodel-whitelist-component-abort",
		Creator:          creator,
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeSD,
		ModelName:        "qwen/qwen-image-2.1",
		TaskArgs:         `{"version":"3.7.0","base_model":{"name":"qwen/qwen-image-2.1"},"components":{"vae":{"repo":"evil/bad-vae","format":"diffusers"}},"dtype":"float16","prompt":"a cat"}`,
		TaskFee:          models.BigInt{Int: *fee},
		Priority:         models.BigInt{Int: *big.NewInt(1)},
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create queued task: %v", err)
	}

	matched, err := runMatchingRound(ctx)
	if err != nil {
		t.Fatalf("matching round failed: %v", err)
	}
	if matched != 0 {
		t.Fatalf("expected no matches, got %d", matched)
	}

	var stored models.InferenceTask
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != models.TaskEndAborted {
		t.Fatalf("expected TaskEndAborted, got %d", stored.Status)
	}
	if stored.AbortReason != models.TaskAbortModelNotAllowed {
		t.Fatalf("expected TaskAbortModelNotAllowed, got %d", stored.AbortReason)
	}
}

func TestRunMatchingRoundAbortsNonWhitelistedModel(t *testing.T) {
	initMatchingTestConfig(t)
	resetModelWhitelistCacheForTest()
	ctx := context.Background()
	db := config.GetDB()
	if err := db.AutoMigrate(
		&models.ModelWhitelist{},
		&models.InferenceTask{},
		&models.RelayAccountEvent{},
		&models.Event{},
	); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}
	if err := InitModelWhitelistCache(ctx, db); err != nil {
		t.Fatalf("init model whitelist: %v", err)
	}
	config.GetConfig().Task.ModelWhitelistEnabled = true
	t.Cleanup(func() { config.GetConfig().Task.ModelWhitelistEnabled = false })

	creator := "0xcreator"
	fee := big.NewInt(100)
	relayAccountCache.mu.Lock()
	relayAccountCache.accounts = map[string]*big.Int{creator: big.NewInt(0)}
	relayAccountCache.mu.Unlock()
	t.Cleanup(func() {
		relayAccountCache.mu.Lock()
		relayAccountCache.accounts = make(map[string]*big.Int)
		relayAccountCache.mu.Unlock()
	})

	task := &models.InferenceTask{
		TaskIDCommitment: "0xmodel-whitelist-match-abort",
		Creator:          creator,
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeLLM,
		ModelName:        "pearl-ai/bad-model",
		TaskFee:          models.BigInt{Int: *fee},
		Priority:         models.BigInt{Int: *big.NewInt(1)},
		CreateTime:       sql.NullTime{Time: time.Now(), Valid: true},
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create queued task: %v", err)
	}

	matched, err := runMatchingRound(ctx)
	if err != nil {
		t.Fatalf("matching round failed: %v", err)
	}
	if matched != 0 {
		t.Fatalf("expected no matches, got %d", matched)
	}

	var stored models.InferenceTask
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != models.TaskEndAborted {
		t.Fatalf("expected TaskEndAborted, got %d", stored.Status)
	}
	if stored.AbortReason != models.TaskAbortModelNotAllowed {
		t.Fatalf("expected TaskAbortModelNotAllowed, got %d", stored.AbortReason)
	}
}
