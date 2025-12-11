package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yml", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.NPorts) == 0 {
		fmt.Println("config contains no NPorts")
		os.Exit(1)
	}

	mode := strings.ToLower(cfg.Mode)
	if mode == "" {
		mode = "passive-listening"
	}
	subMode := strings.ToLower(cfg.SubMode)
	if subMode == "" {
		subMode = "test"
	}

	if mode != "passive-listening" {
		fmt.Printf("mode %q not implemented (expected passive-listening)\n", cfg.Mode)
		os.Exit(1)
	}
	if subMode != "test" && subMode != "store" {
		fmt.Printf("sub_mode %q not implemented under passive-listening\n", cfg.SubMode)
		os.Exit(1)
	}

	baseCtx := context.Background()
	sigCtx, stop := signal.NotifyContext(baseCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx := sigCtx
	var cancel context.CancelFunc
	if subMode == "store" {
		ctx, cancel = context.WithCancel(sigCtx)
		defer cancel()
	}

	// Optional timer shutdown for test mode
	if subMode == "test" && cfg.TestDurationSeconds > 0 {
		timer := time.After(time.Duration(cfg.TestDurationSeconds) * time.Second)
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-timer:
				fmt.Printf("test duration reached (%ds), shutting down\n", cfg.TestDurationSeconds)
				stop()
			}
		}()
	}

	collector := NewSlaveCollector()
	var storage *StorageManager
	var storeCoord *StoreCoordinator
	if subMode == "store" {
		storage = NewStorageManager(cfg.Storage)
		if storage == nil {
			fmt.Println("no storage destinations configured; exiting")
			return
		}
		storeCoord = NewStoreCoordinator(cfg, storage, cancel)
		if storeCoord == nil {
			fmt.Println("no expected slaves configured for store; exiting")
			return
		}
	}
	var wg sync.WaitGroup
	for _, np := range cfg.NPorts {
		np := np
		if cfg.TestOnlyValidCRC {
			np.SkipInvalidCRC = true
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			runPassiveListeningTest(ctx, np, collector, storage, storeCoord, subMode)
		}()
	}

	wg.Wait()
	if collector != nil {
		outBase := outputSuffixFromConfig(*configPath)
		outPath := fmt.Sprintf("test/slave_ids_detected%s.txt", outBase)
		if err := os.MkdirAll("test", 0755); err != nil {
			fmt.Printf("failed to create test directory: %v\n", err)
		} else if err := collector.WriteFile(outPath); err != nil {
			fmt.Printf("failed to write slave ids file: %v\n", err)
		} else {
			fmt.Println(outPath + " written")
		}
	}
	fmt.Println("shutdown complete")
}
