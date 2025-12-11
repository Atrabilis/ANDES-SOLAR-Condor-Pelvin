package main

import "strings"

// FrameHandler consumes frames for device-specific decoding.
type FrameHandler interface {
	HandleFrame(frame []byte, summary FrameSummary) bool
}

func newFrameHandler(np NPortConfig, subMode string, storage *StorageManager, storeCoord *StoreCoordinator) FrameHandler {
	switch strings.ToLower(strings.TrimSpace(np.DeviceType)) {
	case "dustiq":
		return NewDustIQHandler(np.Name, subMode, storage, storeCoord)
	default:
		return nil
	}
}
