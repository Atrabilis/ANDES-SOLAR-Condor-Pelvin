package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config describes the expected YAML structure for the agent.
type Config struct {
	Mode    string `yaml:"mode"`
	SubMode string `yaml:"sub_mode"`
	// TestDurationSeconds applies to test sub-mode; if >0 the program stops after this many seconds.
	TestDurationSeconds int           `yaml:"test_duration_seconds"`
	TestOnlyValidCRC    bool          `yaml:"test_only_valid_crc"`
	NPorts              []NPortConfig `yaml:"nports"`
	Storage             StorageConfig `yaml:"storage"`
}

// NPortConfig defines connection parameters for a single NPort device.
type NPortConfig struct {
	Name              string         `yaml:"name"`
	Host              string         `yaml:"host"`
	Port              int            `yaml:"port"`
	DeviceType        string         `yaml:"device_type"` // optional hint for parsing payloads
	IdleGapMS         int            `yaml:"idle_gap_ms"` // gap of silence that delimits a frame
	Serial            SerialSettings `yaml:"serial"`
	DialTimeoutMS     int            `yaml:"dial_timeout_ms"`     // timeout for establishing the TCP connection
	ReconnectDelayMS  int            `yaml:"reconnect_delay_ms"`  // wait time before retrying after a disconnect
	ReadBufferBytes   int            `yaml:"read_buffer_bytes"`   // optional override for read buffer size
	LogFrameHex       bool           `yaml:"log_frame_hex"`       // whether to print frames in hex
	MaxFrameBytes     int            `yaml:"max_frame_bytes"`     // optional guardrail for frame size
	ConnectionKeepLog bool           `yaml:"connection_keep_log"` // log heartbeat while connected
	SkipInvalidCRC    bool           `yaml:"skip_invalid_crc"`    // if true, ignore frames with bad CRC
	Slaves            []SlaveConfig  `yaml:"slaves"`              // optional per-slave register maps
	DetectedSlaves    []uint8        `yaml:"detected_slaves"`     // expected slaves for store sub-mode
}

// StorageConfig defines local and remote storage destinations.
type StorageConfig struct {
	Local   []StorageTarget `yaml:"local"`
	Remotes []StorageTarget `yaml:"remotes"`
}

// StorageTarget represents a single database destination.
type StorageTarget struct {
	Name          string `yaml:"name"`
	DBType        string `yaml:"db-type"`
	DBURL         string `yaml:"db-url"`
	DBToken       string `yaml:"db-token"`
	DBOrg         string `yaml:"db-org"`
	DBBucket      string `yaml:"db-bucket"`
	DBMeasurement string `yaml:"db-measurement"` // optional; defaults to "registers"
}

// SerialSettings holds basic RS485/RTU timing parameters.
type SerialSettings struct {
	Baud     int     `yaml:"baud"`
	DataBits int     `yaml:"data_bits"`
	Parity   string  `yaml:"parity"`    // none/even/odd
	StopBits float64 `yaml:"stop_bits"` // 1, 1.5, 2
}

// SlaveConfig holds a Modbus slave address and its known registers.
type SlaveConfig struct {
	Address   uint8            `yaml:"address"`
	Name      string           `yaml:"name"`
	Registers []RegisterConfig `yaml:"registers"`
}

// RegisterConfig describes a known register mapping for easier decoding.
type RegisterConfig struct {
	Register      int    `yaml:"register"`
	RegisterName  string `yaml:"register_name"`
	RegisterType  string `yaml:"register_type"`  // supported: int16, uint16 (default int16)
	RegisterCount int    `yaml:"register_count"` // currently only 1 is supported
}

func loadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
