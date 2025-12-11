package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"ion-7400/internal"

	"github.com/goburrow/modbus"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	dotenv "github.com/joho/godotenv"
)

var (
	configPath      = flag.String("configPath", "", "Path to the config file")
	localAvailable  = false
	remoteAvailable = false
)

const (
	envPath = "/home/admin/workspace/.env"
	test    = false
)

func main() {
	fmt.Println("Time of execution:", time.Now().UTC().Format("2006-01-02 15:04:05"))
	flag.Parse()
	if *configPath == "" {
		log.Fatalf("Registers file path is required")
	}

	if err := dotenv.Load(envPath); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	if test {
		fmt.Println(os.Getenv("INFLUX_HOST_LOCAL"))
		fmt.Println(os.Getenv("INFLUX_TOKEN_LOCAL"))
		fmt.Println(os.Getenv("INFLUX_ORG_LOCAL"))
		fmt.Println(os.Getenv("INFLUX_HOST_REMOTE"))
		fmt.Println(os.Getenv("INFLUX_TOKEN_REMOTE"))
		fmt.Println(os.Getenv("INFLUX_ORG_REMOTE"))
	}
	ts := time.Now().Truncate(time.Minute).UTC()
	begin := time.Now()

	// Load YAML
	var devices internal.Devices
	if err := internal.LoadRegisters(*configPath, &devices); err != nil {
		log.Fatalf("Error loading registers file: %v", err)
	}

	// Load storage config
	var storageConfig internal.StorageConfig
	if err := internal.LoadStorageConfig(*configPath, &storageConfig); err != nil {
		log.Fatalf("Error loading storage config file: %v", err)
	}

	// Local InfluxDB
	localInfluxBucket := storageConfig.Local.Influxdb2.Bucket
	localInfluxMeasurement := storageConfig.Local.Influxdb2.Measurement
	localInfluxClient := influxdb2.NewClient(os.Getenv("INFLUX_HOST_LOCAL"), os.Getenv("INFLUX_TOKEN_LOCAL"))
	defer localInfluxClient.Close()
	localPing, err := localInfluxClient.Ping(context.Background())
	if err != nil || !localPing {
		fmt.Println("WARNING: Local InfluxDB not reachable")
		localAvailable = false
	} else {
		localAvailable = true
		fmt.Println("Local InfluxDB is reachable")
	}
	localInfluxWriteAPI := localInfluxClient.WriteAPI(os.Getenv("INFLUX_ORG_LOCAL"), localInfluxBucket)
	errChLocal := localInfluxWriteAPI.Errors()
	go func() {
		for err := range errChLocal {
			fmt.Println("Error writing to local InfluxDB", err)
		}
	}()

	// Remote InfluxDB
	remoteInfluxBucket := storageConfig.Remote.Influxdb2.Bucket
	remoteInfluxMeasurement := storageConfig.Remote.Influxdb2.Measurement
	remoteInfluxClient := influxdb2.NewClient(os.Getenv("INFLUX_HOST_REMOTE"), os.Getenv("INFLUX_TOKEN_REMOTE"))
	defer remoteInfluxClient.Close()
	remotePing, err := remoteInfluxClient.Ping(context.Background())
	if err != nil || !remotePing {
		fmt.Println("WARNING: Remote InfluxDB not reachable")
		remoteAvailable = false
	} else {
		remoteAvailable = true
		fmt.Println("Remote InfluxDB is reachable")
	}
	remoteInfluxWriteAPI := remoteInfluxClient.WriteAPI(os.Getenv("INFLUX_ORG_REMOTE"), remoteInfluxBucket)
	errChRemote := remoteInfluxWriteAPI.Errors()
	go func() {
		for err := range errChRemote {
			fmt.Println("Error writing to remote InfluxDB", err)
		}
	}()

	for _, devItem := range devices.Devices {
		dev := devItem.Device
		fmt.Println("Device:", dev.Name)

		addr := dev.IP + ":" + strconv.Itoa(dev.Port)
		handler := modbus.NewTCPClientHandler(addr)
		handler.Timeout = 5 * time.Second

		if err := handler.Connect(); err != nil {
			log.Fatalf("Error connecting to Modbus (%s): %v", addr, err)
		}
		client := modbus.NewClient(handler)

		for _, slave := range dev.Slaves {
			fmt.Println("  Slave:", slave.Name)
			handler.SlaveId = byte(slave.SlaveID)

			for _, reg := range slave.Registers {
				var resp []byte
				var err error
				switch reg.FunctionCode {
				case 3:
					resp, err = client.ReadHoldingRegisters(uint16(reg.Register-slave.Offset), uint16(reg.Words))
				case 4:
					resp, err = client.ReadInputRegisters(uint16(reg.Register-slave.Offset), uint16(reg.Words))
				default:
					log.Printf("    unknown function code=%d at addr=%d", reg.FunctionCode, reg.Register)
					continue
				}
				if err != nil {
					log.Printf("    read err addr=%d words=%d: %v", reg.Register, reg.Words, err)
					continue
				}
				want := reg.Words * 2
				if len(resp) != want {
					log.Printf("    unexpected length at addr=%d: got=%d want=%d", reg.Register, len(resp), want)
					continue
				}
				if reg.Name == "" {
					log.Printf("    register at addr=%d has empty name; skipping", reg.Register)
					continue
				}
				v := 0.0
				switch reg.Datatype {
				case "UTF-8", "STRING":
					s := internal.UTF8(resp)
					fmt.Printf("    [%s] %-28s -> %q (raw=% x)\n", ts, reg.Name, s, resp)
				case "U8":
					v = float64(internal.U8(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)

				case "U16":
					v = float64(internal.U16(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)

				case "S16":
					v = float64(internal.S16(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)

				case "U32":
					// If you detect swapped words later, replace by internal.U32_CDAB
					v = float64(internal.U32(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)

				case "S32":
					v = float64(internal.S32(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				case "U32LE":
					v = float64(internal.U32LE(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				case "S32LE":
					v = float64(internal.S32LE(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				case "F32BE":
					v = float64(internal.F32BE(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				case "U64BE":
					v = float64(internal.U64BE(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				case "S64BE":
					v = float64(internal.S64BE(resp)) * reg.Gain
					fmt.Printf("    [%s] %-28s -> %.6f %s (raw=% x)\n", ts, reg.Name, v, reg.Unit, resp)
				default:
					log.Printf("    unknown datatype=%q at addr=%d (raw=% x)", reg.Datatype, reg.Register, resp)
					continue
				}
				if test {
					fmt.Println(reg.Name, v)
				} else {
					flags := map[string]string{}
					flags["device"] = dev.Name
					flags["slave"] = slave.Name
					localPoint := influxdb2.NewPoint(localInfluxMeasurement,
						flags,
						map[string]any{reg.Name: v}, ts)
					remotePoint := influxdb2.NewPoint(remoteInfluxMeasurement,
						flags,
						map[string]any{reg.Name: v}, ts)
					if localAvailable {
						localInfluxWriteAPI.WritePoint(localPoint)
					}
					if remoteAvailable {
						remoteInfluxWriteAPI.WritePoint(remotePoint)
					}
				}
			}

			// Close TCP for this device before moving to the next
			if err := handler.Close(); err != nil {
				log.Printf("close error for device %s: %v", dev.Name, err)
			}
		}
		if !test {
			if localAvailable {
				fmt.Println("Flushing local InfluxDB")
				localInfluxWriteAPI.Flush()
			}
			if remoteAvailable {
				fmt.Println("Flushing remote InfluxDB")
				remoteInfluxWriteAPI.Flush()
			}
		}
		fmt.Println("Time taken:", time.Since(begin))
	}
}