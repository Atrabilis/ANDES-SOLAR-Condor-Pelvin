// internal/storage_config.go
package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type StorageFile struct {
	Storage StorageConfig `yaml:"storage"`
}

type StorageConfig struct {
	Local  LocalStorageConfig  `yaml:"local"`
	Remote RemoteStorageConfig `yaml:"remote"`
}

type LocalStorageConfig struct {
	Influxdb2 Influxdb2Config `yaml:"influxdb2"`
}
type RemoteStorageConfig struct {
	Influxdb2 Influxdb2Config `yaml:"influxdb2"`
}

type Influxdb2Config struct {
	Bucket      string `yaml:"bucket"`
	Measurement string `yaml:"measurement"`
}

func LoadStorageConfig(path string, out *StorageConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var wrapper StorageFile
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	// valida mínimos
	if wrapper.Storage.Local.Influxdb2.Measurement == "" {
		return fmt.Errorf("storage.local.influxdb2.measurement is empty")
	}
	if wrapper.Storage.Local.Influxdb2.Bucket == "" {
		return fmt.Errorf("storage.local.influxdb2.bucket is empty")
	}
	*out = wrapper.Storage
	return nil
}
