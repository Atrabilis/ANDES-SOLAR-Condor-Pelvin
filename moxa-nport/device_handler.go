package main

import "strings"

// FrameHandler consumes frames for device-specific decoding.
type FrameHandler interface {
	HandleFrame(frame []byte, summary FrameSummary)
}

func newFrameHandler(np NPortConfig, subMode string) FrameHandler {
	if strings.ToLower(strings.TrimSpace(subMode)) != "test" {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(np.DeviceType)) {
	case "dustiq":
		return NewDustIQHandler(np.Name)
	default:
		return nil
	}
}
