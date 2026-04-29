package service

import "sync"

type nodeJoinLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

var globalNodeJoinLockRegistry = &nodeJoinLockRegistry{
	locks: make(map[string]*sync.Mutex),
}

func lockNodeJoinByAddress(address string) func() {
	globalNodeJoinLockRegistry.mu.Lock()
	lock, ok := globalNodeJoinLockRegistry.locks[address]
	if !ok {
		lock = &sync.Mutex{}
		globalNodeJoinLockRegistry.locks[address] = lock
	}
	globalNodeJoinLockRegistry.mu.Unlock()

	lock.Lock()
	return func() {
		lock.Unlock()
	}
}

func LockNodeJoinByAddress(address string) func() {
	return lockNodeJoinByAddress(address)
}
