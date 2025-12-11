package main

import (
	"fmt"
	"strings"
)

// RegisterValue represents a decoded register value for a slave.
type RegisterValue struct {
	Register int
	Name     string
	Type     string // int16 or uint16
	Value    float64
}

// decodeKnownRegisters returns human-readable lines and structured values for a slave's known registers.
func decodeKnownRegisters(np NPortConfig, slaveID uint8, data []byte) (slaveName string, lines []string, values []RegisterValue) {
	if len(data)%2 != 0 || len(np.Slaves) == 0 {
		return "", nil, nil
	}

	var slave *SlaveConfig
	for i := range np.Slaves {
		if np.Slaves[i].Address == slaveID {
			slave = &np.Slaves[i]
			break
		}
	}
	if slave == nil || len(slave.Registers) == 0 {
		return "", nil, nil
	}

	slaveName = slave.Name
	for _, reg := range slave.Registers {
		if reg.Register < 0 {
			continue
		}
		count := reg.RegisterCount
		if count == 0 {
			count = 1
		}
		if count != 1 {
			continue
		}

		byteIdx := reg.Register * 2
		if byteIdx+1 >= len(data) {
			continue
		}
		raw := data[byteIdx : byteIdx+2]
		valueU16 := uint16(raw[0])<<8 | uint16(raw[1])

		typ := strings.ToLower(reg.RegisterType)
		if typ == "" {
			typ = "int16"
		}

		switch typ {
		case "uint16":
			val := float64(valueU16)
			lines = append(lines, fmt.Sprintf("reg=%d name=%s uint16=%d", reg.Register, reg.RegisterName, int64(val)))
			values = append(values, RegisterValue{
				Register: reg.Register,
				Name:     reg.RegisterName,
				Type:     "uint16",
				Value:    val,
			})
		default: // int16 or unknown -> treat as int16
			val := float64(int16(valueU16))
			lines = append(lines, fmt.Sprintf("reg=%d name=%s int16=%d", reg.Register, reg.RegisterName, int64(val)))
			values = append(values, RegisterValue{
				Register: reg.Register,
				Name:     reg.RegisterName,
				Type:     "int16",
				Value:    val,
			})
		}
	}

	return slaveName, lines, values
}
