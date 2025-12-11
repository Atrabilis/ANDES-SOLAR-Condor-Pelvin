package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const dustIQExpectedFrames = 23

type dustIQHandler struct {
	port  string
	cycle [][]byte
}

type dustIQSpec struct {
	name      string
	converter func([]byte) (float64, bool)
}

type dustIQValue struct {
	name  string
	value float64
}

var dustIQSpecs = []dustIQSpec{
	{name: "ir_device_type", converter: dustIQUint16},
	{name: "ir_datamodel_version", converter: dustIQUint16},
	{name: "ir_software_version", converter: dustIQUint16},
	{name: "ir_batch_number", converter: dustIQUint16},
	{name: "ir_serial_number", converter: dustIQUint16},
	{name: "ir_hardware_version", converter: dustIQUint16},
	{name: "ir_soiling_ratio_sensor1", converter: dustIQUint16Div10},
	{name: "ir_tr_loss_sensor1", converter: dustIQInt16Div10},
	{name: "ir_soiling_ratio_sensor2", converter: dustIQUint16Div10},
	{name: "ir_tr_loss_sensor2", converter: dustIQInt16Div10},
	{name: "", converter: nil}, // reserved register in the current cycle mapping
	{name: "ir_backpanel_temp_c", converter: dustIQBackpanelTemp},
	{name: "ir_calibration_year", converter: dustIQUint16},
	{name: "ir_calibration_month", converter: dustIQUint16},
	{name: "ir_calibration_day", converter: dustIQUint16},
	{name: "ir_tilt_x_direction", converter: dustIQInt16Div10},
	{name: "ir_tilt_y_direction", converter: dustIQInt16Div10},
	{name: "ir_calibration_flags", converter: dustIQUint16},
	{name: "ir_device_voltage", converter: dustIQDeviceVoltage},
	{name: "ir_operational_mode", converter: dustIQInt16},
	{name: "ir_dust_tilt_sensor_1", converter: dustIQUint16},
	{name: "ir_dust_tilt_sensor_2", converter: dustIQUint16},
	{name: "placeholder_22", converter: dustIQUint16},
}

// NewDustIQHandler creates a handler that decodes single-register frames into DustIQ cycles.
func NewDustIQHandler(port string) FrameHandler {
	return &dustIQHandler{
		port:  port,
		cycle: make([][]byte, 0, dustIQExpectedFrames),
	}
}

func (h *dustIQHandler) HandleFrame(frame []byte, summary FrameSummary) {
	if len(frame) != 7 {
		return
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		return
	}
	if summary.ByteCount == nil || *summary.ByteCount != 2 {
		return
	}

	value := binary.BigEndian.Uint16(frame[3:5])
	if value == 800 {
		if len(h.cycle) > 0 {
			h.flush()
		}
		h.cycle = h.cycle[:0]
		h.cycle = append(h.cycle, h.copyFrame(frame))
		return
	}

	if len(h.cycle) == 0 {
		return
	}
	h.cycle = append(h.cycle, h.copyFrame(frame))
}

func (h *dustIQHandler) flush() {
	if len(h.cycle) == 0 {
		return
	}
	values, warnings, err := decodeDustIQCycle(h.cycle)
	if err != nil {
		fmt.Printf("[%s] dustiq: %v\n", h.port, err)
		h.cycle = h.cycle[:0]
		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf("[%s] dustiq cycle %s\n", h.port, timestamp)
	for _, v := range values {
		fmt.Printf("  %s=%s\n", v.name, formatDustIQValue(v.value))
	}
	for _, warn := range warnings {
		fmt.Printf("[%s] dustiq warning: %s\n", h.port, warn)
	}
	h.cycle = h.cycle[:0]
}

func (h *dustIQHandler) copyFrame(frame []byte) []byte {
	dup := make([]byte, len(frame))
	copy(dup, frame)
	return dup
}

func decodeDustIQCycle(frames [][]byte) ([]dustIQValue, []string, error) {
	if len(frames) < dustIQExpectedFrames {
		return nil, nil, fmt.Errorf("incomplete cycle: expected %d frames, got %d", dustIQExpectedFrames, len(frames))
	}

	values := make([]dustIQValue, 0, dustIQExpectedFrames)
	var warnings []string
	for idx, spec := range dustIQSpecs {
		if spec.name == "" || spec.converter == nil {
			continue
		}
		val, ok := spec.converter(frames[idx])
		if !ok {
			warnings = append(warnings, fmt.Sprintf("reg %d (%s) invalid frame", idx, spec.name))
			continue
		}
		values = append(values, dustIQValue{name: spec.name, value: val})
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

func formatDustIQValue(v float64) string {
	s := fmt.Sprintf("%.3f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
