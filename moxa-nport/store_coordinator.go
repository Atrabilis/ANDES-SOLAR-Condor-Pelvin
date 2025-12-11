package main

import (
	"context"
	"sync"
	"time"
)

// StoreCoordinator keeps track of which slaves have produced valid frames and triggers a single store when all are seen.
type StoreCoordinator struct {
	expected map[string]map[uint8]struct{}
	last     map[string]map[uint8]StoredFrame
	storage  *StorageManager
	cancel   context.CancelFunc
	mu       sync.Mutex
	done     bool
}

type StoredFrame struct {
	slaveName string
	values    []RegisterValue
	ts        time.Time
}

func NewStoreCoordinator(cfg Config, storage *StorageManager, cancel context.CancelFunc) *StoreCoordinator {
	if storage == nil {
		return nil
	}
	expected := make(map[string]map[uint8]struct{})
	for _, np := range cfg.NPorts {
		if len(np.DetectedSlaves) == 0 {
			continue
		}
		set := make(map[uint8]struct{}, len(np.DetectedSlaves))
		for _, s := range np.DetectedSlaves {
			set[s] = struct{}{}
		}
		expected[np.Name] = set
	}
	if len(expected) == 0 {
		return nil
	}
	return &StoreCoordinator{
		expected: expected,
		last:     make(map[string]map[uint8]StoredFrame),
		storage:  storage,
		cancel:   cancel,
	}
}

// Record saves the latest values for a slave and triggers storage when all expected slaves are seen.
func (sc *StoreCoordinator) Record(port string, slaveID uint8, slaveName string, values []RegisterValue) {
	if sc == nil || len(values) == 0 {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.done {
		return
	}

	expectedSlaves, ok := sc.expected[port]
	if !ok || len(expectedSlaves) == 0 {
		return
	}
	if _, wanted := expectedSlaves[slaveID]; !wanted {
		return
	}

	if sc.last[port] == nil {
		sc.last[port] = make(map[uint8]StoredFrame)
	}
	sc.last[port][slaveID] = StoredFrame{
		slaveName: slaveName,
		values:    values,
		ts:        time.Now().UTC(),
	}

	if sc.allSeen() {
		sc.flush()
		sc.done = true
		if sc.cancel != nil {
			sc.cancel()
		}
	}
}

func (sc *StoreCoordinator) allSeen() bool {
	for port, expectedSlaves := range sc.expected {
		lastForPort := sc.last[port]
		for slave := range expectedSlaves {
			if lastForPort == nil {
				return false
			}
			if _, ok := lastForPort[slave]; !ok {
				return false
			}
		}
	}
	return true
}

func (sc *StoreCoordinator) flush() {
	for port, slaves := range sc.last {
		for slaveID, frame := range slaves {
			sc.storage.Store(port, slaveID, frame.slaveName, frame.values, frame.ts)
		}
	}
}
