package internal

import (
	"bytes"
	"os"

	"gopkg.in/yaml.v3"
)

type Devices struct {
	Devices []DeviceItem `yaml:"devices"`
}

type DeviceItem struct {
	Device Device `yaml:"device"`
}

type Device struct {
	Name   string   `yaml:"name"`
	IP     string   `yaml:"ip"`
	Port   int      `yaml:"port"`
	Flags  []string `yaml:"flags,omitempty"` // optional at device level
	Slaves []Slave  `yaml:"slaves"`
}

type Slave struct {
	Name    string `yaml:"name"`
	SlaveID int    `yaml:"slave_id"`
	Offset  int    `yaml:"offset"`
	// No 'flags' at slave level in the new structure
	Registers []Register `yaml:"modbus_registers"`
}

type Register struct {
	Register     int          `yaml:"register"`
	FunctionCode int          `yaml:"function_code"`
	Name         string       `yaml:"name"`
	Description  string       `yaml:"description"`
	Words        int          `yaml:"words"`
	Datatype     string       `yaml:"datatype"`
	Unit         string       `yaml:"unit"`
	Gain         float64      `yaml:"gain"`
	Flags        RegisterFlag `yaml:"flags,omitempty"` // list of objects per register
}

// RegisterFlag supports heterogeneous keys used in YAML:
//   - module_number: 16
//   - module_label: "amp/freq/unbal"
type RegisterFlag struct {
	ModuleNumber int    `yaml:"module_number"`
	ModuleLabel  string `yaml:"module_label"`
}

func LoadRegisters(path string, devices *Devices) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, devices)
}

// ---------- Byte-order helpers (big-endian by byte) ----------

func U8(b []byte) uint8 {
	if len(b) == 0 {
		return 0
	}
	if len(b) == 1 {
		return uint8(b[0])
	}
	return uint8(b[len(b)-1]) // low byte
}

func U16(b []byte) uint16 {
	if len(b) < 2 {
		return 0
	}
	return uint16(b[0])<<8 | uint16(b[1])
}

func U32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func S16(b []byte) int16 {
	return int16(U16(b)) // two's complement
}

func S32(b []byte) int32 {
	return int32(U32(b)) // two's complement
}
func UTF8(b []byte) string {
	// cortar en primer NUL si existe
	if i := bytes.IndexByte(b, 0x00); i >= 0 {
		b = b[:i]
	}
	// quitar padding NUL del final si quedara
	b = bytes.TrimRight(b, "\x00")
	return string(b)
}

func U32LE(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	low := U16(b[0:2])  // first word = low
	high := U16(b[2:4]) // second word = high
	return uint32(low) | (uint32(high) << 16)
}

func S32LE(b []byte) int32 { return int32(U32LE(b)) }
