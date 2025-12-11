package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// runPassiveListeningTest connects and prints frames observed on the socket without sending polls.
func runPassiveListeningTest(ctx context.Context, np NPortConfig, collector *SlaveCollector, storage *StorageManager, storeCoord *StoreCoordinator, subMode string) {
	addr := fmt.Sprintf("%s:%d", np.Host, np.Port)
	idleGap := deriveIdleGap(np, 5*time.Millisecond)
	reconnectDelay := durationOrDefault(np.ReconnectDelayMS, 2*time.Second)
	dialTimeout := durationOrDefault(np.DialTimeoutMS, 2*time.Second)
	readBufSize := np.ReadBufferBytes
	if readBufSize <= 0 {
		readBufSize = 1024
	}
	maxFrame := np.MaxFrameBytes
	if maxFrame <= 0 {
		maxFrame = 4096
	}

	handler := newFrameHandler(np, subMode)

	for {
		if ctx.Err() != nil {
			return
		}

		conn, err := net.DialTimeout("tcp", addr, dialTimeout)
		if err != nil {
			fmt.Printf("[%s] dial failed: %v (retrying in %s)\n", np.Name, err, reconnectDelay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnectDelay):
				continue
			}
		}

		fmt.Printf("[%s] connected to %s\n", np.Name, addr)
		if err := streamFrames(ctx, conn, np, idleGap, readBufSize, maxFrame, collector, storage, storeCoord, handler); err != nil {
			fmt.Printf("[%s] connection closed: %v\n", np.Name, err)
		}

		_ = conn.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// streamFrames groups incoming bytes into frames separated by idleGap and logs them.
func streamFrames(ctx context.Context, conn net.Conn, np NPortConfig, idleGap time.Duration, readBufSize int, maxFrame int, collector *SlaveCollector, storage *StorageManager, storeCoord *StoreCoordinator, handler FrameHandler) error {
	buf := make([]byte, readBufSize)
	var frame []byte

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		_ = conn.SetReadDeadline(time.Now().Add(idleGap))
		n, err := conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if len(frame) > 0 {
					summary := logFrame(np, frame, collector, storage, storeCoord)
					if handler != nil {
						handler.HandleFrame(frame, summary)
					}
					frame = frame[:0]
				} else if np.ConnectionKeepLog {
					fmt.Printf("[%s] idle\n", np.Name)
				}
				continue
			}
			if errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrDeadlineExceeded) {
				return err
			}
			return fmt.Errorf("read error: %w", err)
		}

		frame = append(frame, buf[:n]...)
		if len(frame) >= maxFrame {
			summary := logFrame(np, frame, collector, storage, storeCoord)
			if handler != nil {
				handler.HandleFrame(frame, summary)
			}
			frame = frame[:0]
		}
	}
}

func logFrame(np NPortConfig, frame []byte, collector *SlaveCollector, storage *StorageManager, storeCoord *StoreCoordinator) FrameSummary {
	if len(frame) == 0 {
		return FrameSummary{}
	}
	summary := summarizeFrame(frame)
	if np.SkipInvalidCRC && (summary.CRCValid == nil || !*summary.CRCValid) {
		return summary
	}
	if collector != nil && summary.CRCValid != nil && *summary.CRCValid {
		collector.Record(np.Name, summary.SlaveID)
	}
	var (
		dataDec        string
		parserLines    []string
		registerLines  []string
		registerValues []RegisterValue
		slaveName      string
	)
	if summary.CRCValid != nil && *summary.CRCValid && len(frame) > 4 {
		start := 2 // skip addr, func
		if summary.ByteCount != nil {
			start = 3 // also skip byte count
		}
		if start < len(frame)-2 {
			data := frame[start : len(frame)-2] // exclude CRC16
			dataDec = fmt.Sprintf("%v", bytesToDecimal(data))
			if summary.ByteCount != nil && *summary.ByteCount == len(data) && len(data)%2 == 0 {
				parserLines = registerParserLines(data)
				slaveName, registerLines, registerValues = decodeKnownRegisters(np, summary.SlaveID, data)
			}
		}
	}
	header := fmt.Sprintf("[%s] frame: %s", np.Name, formatSummary(summary))
	if np.LogFrameHex {
		header += " | hex: " + toHex(frame)
	}

	var lines []string
	lines = append(lines, header)
	if dataDec != "" {
		lines = append(lines, "  data_dec: "+dataDec)
	}
	if len(parserLines) > 0 {
		lines = append(lines, "  parsers:")
		for _, p := range parserLines {
			lines = append(lines, "    "+p)
		}
	}
	if len(registerLines) > 0 {
		lines = append(lines, "  registers:")
		for _, r := range registerLines {
			lines = append(lines, "    "+r)
		}
	}

	fmt.Println(strings.Join(lines, "\n"))

	if len(registerValues) > 0 {
		if storeCoord != nil {
			storeCoord.Record(np.Name, summary.SlaveID, slaveName, registerValues)
		} else if storage != nil {
			storage.Store(np.Name, summary.SlaveID, slaveName, registerValues, time.Now().UTC())
		}
	}
	return summary
}

func durationOrDefault(ms int, fallback time.Duration) time.Duration {
	if ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	return fallback
}

// deriveIdleGap computes idle gap from serial settings if idle_gap_ms is not provided.
func deriveIdleGap(np NPortConfig, fallback time.Duration) time.Duration {
	if np.IdleGapMS > 0 {
		return time.Duration(np.IdleGapMS) * time.Millisecond
	}
	baud := np.Serial.Baud
	if baud <= 0 {
		return fallback
	}
	dataBits := np.Serial.DataBits
	if dataBits <= 0 {
		dataBits = 8
	}
	stopBits := np.Serial.StopBits
	if stopBits <= 0 {
		stopBits = 1
	}
	parityBits := 0.0
	switch strings.ToLower(np.Serial.Parity) {
	case "even", "odd", "mark", "space":
		parityBits = 1
	default:
		parityBits = 0
	}
	bitsPerChar := 1.0 + float64(dataBits) + parityBits + stopBits
	tCharSec := bitsPerChar / float64(baud)
	idle := tCharSec * 3.5
	d := time.Duration(idle * float64(time.Second))
	if d <= 0 {
		return fallback
	}
	return d
}
