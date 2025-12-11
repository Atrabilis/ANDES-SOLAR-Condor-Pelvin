package main

import (
	"fmt"
	"strings"
)

// FrameSummary holds a minimal parse of a Modbus RTU-like frame (address, function, length, CRC).
type FrameSummary struct {
	Length       int
	SlaveID      uint8
	FunctionCode uint8
	DataLength   int
	ByteCount    *int
	CRC          uint16
	CRCValid     *bool
}

// summarizeFrame attempts a light parse to help fingerprint traffic.
// It assumes a Modbus RTU-like structure: Addr (1) + Function (1) + Data (n) + CRC (2).
func summarizeFrame(frame []byte) FrameSummary {
	s := FrameSummary{Length: len(frame)}
	if len(frame) < 4 {
		return s
	}

	s.SlaveID = frame[0]
	s.FunctionCode = frame[1]
	s.DataLength = len(frame) - 4 // subtract addr, func, CRC16

	if len(frame) >= 4 {
		payload := frame[:len(frame)-2]
		crcExpected := modbusCRC16(payload)
		crcSeen := uint16(frame[len(frame)-2]) | uint16(frame[len(frame)-1])<<8
		valid := crcExpected == crcSeen
		s.CRC = crcSeen
		s.CRCValid = &valid
	}

	// Try to read byte count as first byte of data if any
	if s.DataLength > 0 {
		count := int(frame[2])
		s.ByteCount = &count
	}

	return s
}

func formatSummary(s FrameSummary) string {
	if s.Length == 0 {
		return "empty frame"
	}

	crcPart := "crc: n/a"
	if s.CRCValid != nil {
		crcStatus := "bad"
		if *s.CRCValid {
			crcStatus = "ok"
		}
		crcPart = fmt.Sprintf("crc: 0x%04X (%s)", s.CRC, crcStatus)
	}

	byteCountPart := ""
	if s.ByteCount != nil {
		byteCountPart = fmt.Sprintf(" byte_count=%d", *s.ByteCount)
	}

	return fmt.Sprintf("len=%d addr=%d func=0x%02X data_len=%d%s %s", s.Length, s.SlaveID, s.FunctionCode, s.DataLength, byteCountPart, crcPart)
}

// modbusCRC16 computes the Modbus RTU CRC16 over the given bytes.
func modbusCRC16(b []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, v := range b {
		crc ^= uint16(v)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func toHex(b []byte) string {
	const hextable = "0123456789ABCDEF"
	out := make([]byte, 3*len(b))
	for i, v := range b {
		out[i*3] = hextable[v>>4]
		out[i*3+1] = hextable[v&0x0F]
		out[i*3+2] = ' '
	}
	if len(out) > 0 {
		out = out[:len(out)-1]
	}
	return string(out)
}

// bytesToDecimal returns slice of ints representing byte values.
func bytesToDecimal(b []byte) []int {
	out := make([]int, len(b))
	for i, v := range b {
		out[i] = int(v)
	}
	return out
}

func formatRegisterParsers(data []byte) string {
	// data is expected to be two bytes representing a single register
	if len(data) != 2 {
		return ""
	}
	u16be := uint16(data[0])<<8 | uint16(data[1])
	i16be := int16(u16be)

	var parts []string
	parts = append(parts, fmt.Sprintf("u16be=%d", u16be))
	parts = append(parts, fmt.Sprintf("i16be=%d", i16be))

	return strings.Join(parts, ", ")
}

func registerParserLines(data []byte) []string {
	// Interpret every two bytes as a Modbus register and parse them individually.
	if len(data)%2 != 0 {
		return nil
	}
	if len(data) == 2 {
		return []string{formatRegisterParsers(data)}
	}

	lines := make([]string, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		lines = append(lines, fmt.Sprintf("[%d] %s", i/2, formatRegisterParsers(data[i:i+2])))
	}
	return lines
}

func parseBCD(data []byte) (string, bool) {
	var sb strings.Builder
	for _, b := range data {
		hi := (b >> 4) & 0x0F
		lo := b & 0x0F
		if hi > 9 || lo > 9 {
			return "", false
		}
		sb.WriteByte('0' + hi)
		sb.WriteByte('0' + lo)
	}
	return sb.String(), true
}

func printableASCII(data []byte) string {
	for _, b := range data {
		if b < 32 || b > 126 {
			return ""
		}
	}
	return string(data)
}
