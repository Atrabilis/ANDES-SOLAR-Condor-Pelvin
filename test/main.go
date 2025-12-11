package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/goburrow/modbus"
)

// decodeUTF8 decodifica bytes como UTF-8, similar a internal.UTF8
func decodeUTF8(b []byte) string {
	// cortar en primer NUL si existe
	if i := bytes.IndexByte(b, 0x00); i >= 0 {
		b = b[:i]
	}
	// quitar padding NUL del final si quedara
	b = bytes.TrimRight(b, "\x00")
	return string(b)
}

func main() {
	target := "192.168.1.90:502" // CHANGE THIS IF NEEDED
	fmt.Println("Testing Modbus device at", target)
	fmt.Println("Scanning Slave IDs 0-255 for serial number (register 4990, Input Register, 10 words)")
	fmt.Println()

	// Check if port is open before using Modbus
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		log.Fatalf("Port 502 not reachable: %v", err)
	}
	conn.Close()
	fmt.Println("Port 502 reachable.")
	fmt.Println()

	// Configuración del registro "sn"
	// register: 4990, offset: 1, entonces dirección base 0 = 4990 - 1 = 4989
	snRegister := uint16(4989) // base 0 (4990 - offset 1)
	snWords := uint16(10)

	// Estructura para guardar los resultados
	type Inverter struct {
		SlaveID      int
		SerialNumber string
	}
	var foundInverters []Inverter

	totalSlaves := 256
	progressInterval := 10 // Mostrar progreso cada 10 Slave IDs

	fmt.Println("Starting scan...")
	startTime := time.Now()

	for slaveID := 0; slaveID <= 255; slaveID++ {
		// Mostrar progreso cada N Slave IDs
		if slaveID%progressInterval == 0 {
			progress := float64(slaveID) / float64(totalSlaves) * 100
			fmt.Printf("\rProgress: %3d/256 (%.1f%%) | Found: %d", slaveID, progress, len(foundInverters))
		}
		handler := modbus.NewTCPClientHandler(target)
		handler.Timeout = 500 * time.Millisecond
		handler.SlaveId = byte(slaveID)

		if err := handler.Connect(); err != nil {
			// Silenciosamente continuar si no puede conectar
			continue
		}

		client := modbus.NewClient(handler)

		// Intentar leer el registro "sn" (Input Register, function code 4)
		resp, err := client.ReadInputRegisters(snRegister, snWords)
		if err != nil {
			handler.Close()
			continue
		}

		// Verificar que la respuesta tenga el tamaño correcto (10 words = 20 bytes)
		if len(resp) != int(snWords*2) {
			handler.Close()
			continue
		}

		// Decodificar como UTF-8
		sn := decodeUTF8(resp)
		sn = strings.TrimSpace(sn)

		// Si el serial number es válido (no vacío y tiene al menos 3 caracteres)
		if len(sn) >= 3 {
			foundInverters = append(foundInverters, Inverter{
				SlaveID:      slaveID,
				SerialNumber: sn,
			})
			// Mostrar inmediatamente cuando se encuentra uno
			fmt.Printf("\r✓ Found at Slave ID %3d: %s\n", slaveID, sn)
		}

		handler.Close()

		// Pequeña pausa para no saturar el dispositivo
		time.Sleep(50 * time.Millisecond)
	}

	// Mostrar progreso final
	fmt.Printf("\rProgress: 256/256 (100.0%%) | Found: %d\n\n", len(foundInverters))

	// Resumen final
	elapsed := time.Since(startTime)
	fmt.Println("=" + strings.Repeat("=", 60) + "=")
	fmt.Println("SCAN COMPLETE")
	fmt.Println("=" + strings.Repeat("=", 60) + "=")
	fmt.Printf("Total time: %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Slave IDs scanned: 256\n")
	fmt.Printf("Inverters found: %d\n", len(foundInverters))
	fmt.Println()

	if len(foundInverters) > 0 {
		fmt.Println("FOUND INVERTERS:")
		fmt.Println(strings.Repeat("-", 62))
		fmt.Printf("%-12s | %s\n", "Slave ID", "Serial Number")
		fmt.Println(strings.Repeat("-", 62))
		for _, inv := range foundInverters {
			fmt.Printf("%-12d | %s\n", inv.SlaveID, inv.SerialNumber)
		}
		fmt.Println(strings.Repeat("-", 62))
	} else {
		fmt.Println("No inverters found with valid serial numbers.")
		fmt.Println("Please check:")
		fmt.Println("  - IP address is correct")
		fmt.Println("  - Device is powered on and connected")
		fmt.Println("  - Register address (4990) is correct for your device model")
	}
}
