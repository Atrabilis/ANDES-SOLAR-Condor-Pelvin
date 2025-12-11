package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const dustIQExpectedFrames = 23

type dustIQHandler struct {
	port         string
	mode         string
	cycle        [][]byte
	currentSlave uint8
	storage      *StorageManager
}

type dustIQSpec struct {
	name      string
	converter func([]byte) (float64, bool)
	valueType string
}

var dustIQSpecs = []dustIQSpec{
	{name: "ir_device_type", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_datamodel_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_software_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_batch_number", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_serial_number", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_hardware_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_soiling_ratio_sensor1", converter: dustIQUint16Div10, valueType: "float"},
	{name: "ir_tr_loss_sensor1", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_soiling_ratio_sensor2", converter: dustIQUint16Div10, valueType: "float"},
	{name: "ir_tr_loss_sensor2", converter: dustIQInt16Div10, valueType: "float"},
	{name: "", converter: nil, valueType: ""},
	{name: "ir_backpanel_temp", converter: dustIQBackpanelTemp, valueType: "float"},
	{name: "ir_calibration_year", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_calibration_month", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_calibration_day", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_tilt_x_direction", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_tilt_y_direction", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_calibration_flags", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_device_voltage", converter: dustIQDeviceVoltage, valueType: "float"},
	{name: "ir_operational_mode", converter: dustIQInt16, valueType: "int16"},
	{name: "ir_dust_tilt_sensor_1", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_dust_tilt_sensor_2", converter: dustIQUint16, valueType: "uint16"},
	{name: "placeholder_22", converter: dustIQUint16, valueType: "uint16"},
}

// NewDustIQHandler creates a handler that decodes single-register frames into DustIQ cycles.
func NewDustIQHandler(port string, subMode string, storage *StorageManager, _ *StoreCoordinator) FrameHandler {
	mode := strings.ToLower(strings.TrimSpace(subMode))
	return &dustIQHandler{
		port:    port,
		mode:    mode,
		storage: storage,
		cycle:   make([][]byte, 0, dustIQExpectedFrames),
	}
}

func (h *dustIQHandler) HandleFrame(frame []byte, summary FrameSummary) bool {
	if len(frame) != 7 {
		return false
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		return false
	}
	if summary.ByteCount == nil || *summary.ByteCount != 2 {
		return false
	}

	value := binary.BigEndian.Uint16(frame[3:5])
	if value == 800 {
		if len(h.cycle) > 0 {
			if h.flush() {
				return true
			}
		}
		h.resetCycle()
		h.currentSlave = summary.SlaveID
		h.cycle = append(h.cycle, h.copyFrame(frame))
		return false
	}

	if len(h.cycle) == 0 {
		return false
	}
	if summary.SlaveID != h.currentSlave {
		if h.flush() {
			return true
		}
		h.resetCycle()
		return false
	}
	h.cycle = append(h.cycle, h.copyFrame(frame))
	return false
}

func (h *dustIQHandler) flush() bool {
	if len(h.cycle) == 0 {
		return false
	}
	values, warnings, err := decodeDustIQCycle(h.cycle)
	if err != nil {
		fmt.Printf("[%s] dustiq: %v\n", h.port, err)
		h.resetCycle()
		return false
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf("[%s] dustiq cycle %s\n", h.port, timestamp)
	for _, v := range values {
		fmt.Printf("  %s=%s\n", v.Name, formatDustIQValue(v.Value, v.Type))
	}
	for _, warn := range warnings {
		fmt.Printf("[%s] dustiq warning: %s\n", h.port, warn)
	}

	shouldStop := false
	if h.shouldStore() && len(values) > 0 {
		now := time.Now().UTC()
		if h.storage != nil {
			h.storage.Store(h.port, h.currentSlave, "", values, now)
		}
		shouldStop = true
	}
	h.resetCycle()
	return shouldStop
}

func (h *dustIQHandler) shouldStore() bool {
	return h.mode == "store" && h.storage != nil
}

func (h *dustIQHandler) resetCycle() {
	h.cycle = h.cycle[:0]
	h.currentSlave = 0
}

func (h *dustIQHandler) copyFrame(frame []byte) []byte {
	dup := make([]byte, len(frame))
	copy(dup, frame)
	return dup
}

func decodeDustIQCycle(frames [][]byte) ([]RegisterValue, []string, error) {
	if len(frames) < dustIQExpectedFrames {
		return nil, nil, fmt.Errorf("incomplete cycle: expected %d frames, got %d", dustIQExpectedFrames, len(frames))
	}

	values := make([]RegisterValue, 0, len(dustIQSpecs))
	var warnings []string
	for idx, spec := range dustIQSpecs {
		if spec.name == "" || spec.converter == nil {
			continue
		}
		if idx >= len(frames) {
			warnings = append(warnings, fmt.Sprintf("reg %d (%s) missing frame", idx, spec.name))
			continue
		}
		val, ok := spec.converter(frames[idx])
		if !ok {
			warnings = append(warnings, fmt.Sprintf("reg %d (%s) invalid frame", idx, spec.name))
			continue
		}
		values = append(values, RegisterValue{
			Register: idx,
			Name:     spec.name,
			Type:     spec.valueType,
			Value:    val,
		})
	}
	return values, warnings, nil
}

func dustIQUint16(frame []byte) (float64, bool) {
	if len(frame) < 5 {
		return 0, false
	}
	return float64(binary.BigEndian.Uint16(frame[3:5])), true
}

func dustIQUint16Div10(frame []byte) (float64, bool) {
	val, ok := dustIQUint16(frame)
	if !ok {
		return 0, false
	}
	return val / 10.0, true
}

func dustIQInt16(frame []byte) (float64, bool) {
	if len(frame) < 5 {
		return 0, false
	}
	return float64(int16(binary.BigEndian.Uint16(frame[3:5]))), true
}

func dustIQInt16Div10(frame []byte) (float64, bool) {
	val, ok := dustIQInt16(frame)
	if !ok {
		return 0, false
	}
	return val / 10.0, true
}

func dustIQBackpanelTemp(frame []byte) (float64, bool) {
	val, ok := dustIQUint16(frame)
	if !ok {
		return 0, false
	}
	return val/10.0 - 273.15, true
}

func dustIQDeviceVoltage(frame []byte) (float64, bool) {
	val, ok := dustIQInt16(frame)
	if !ok {
		return 0, false
	}
	return val / 1000.0, true
}

func formatDustIQValue(v float64, typ string) string {
	switch strings.ToLower(typ) {
	case "float":
		return trimFloat(v)
	default:
		return fmt.Sprintf("%d", int64(v))
	}
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.4f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
