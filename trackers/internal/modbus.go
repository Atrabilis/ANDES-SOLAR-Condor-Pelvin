package internal

import (
	"bytes"
	"math"
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

func F32BE(b []byte) float32 {
	if len(b) < 4 {
		return 0
	}
	u := U32(b) // ya es big-endian por byte (ABCD)
	return math.Float32frombits(u)
}

func F32LE(b []byte) float32 {
	if len(b) < 4 {
		return 0
	}
	u := U32LE(b) // DCBA por tu helper LE
	return math.Float32frombits(u)
}

func F32CDAB(b []byte) float32 {
	// word-swap (16-bit): [A B][C D] -> [C D][A B]
	if len(b) < 4 {
		return 0
	}
	tmp := []byte{b[2], b[3], b[0], b[1]}
	return F32BE(tmp)
}

func F32BADC(b []byte) float32 {
	// byte-swap dentro de cada palabra: [A B][C D] -> [B A][D C]
	if len(b) < 4 {
		return 0
	}
	tmp := []byte{b[1], b[0], b[3], b[2]}
	return F32BE(tmp)
}

// ---------- (opcional) Float64 si mañana lo necesitas ----------
func F64BE(b []byte) float64 {
	if len(b) < 8 {
		return 0
	}
	u := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	return math.Float64frombits(u)
}

func U64BE(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}
func S64BE(b []byte) int64 { return int64(U64BE(b)) }