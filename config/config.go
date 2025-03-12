package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server   ServerConfig
	Log      LogConfig
	Database DatabaseConfig
	Cache    CacheConfig
}

type ServerConfig struct {
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	BaseDir string `toml:"baseDir"`
}

type LogConfig struct {
	LogFilePath string `toml:"logfilepath"`
	MaxLogSize  int    `toml:"maxlogsize"`
}

type DatabaseConfig struct {
	Path string `toml:"path"` // bolt file path
}

/*
[cache]
expire = "30m"
expireEx = "10m"
*/
type CacheConfig struct {
	Expire   time.Duration `toml:"expire"`
	ExpireEx time.Duration `toml:"expireEx"`
}

// LoadConfig 从 TOML 配置文件加载配置
func LoadConfig(filePath string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(filePath, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveConfig 保存配置到 TOML 配置文件
func SaveConfig(filePath string, config *Config) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return err
	}
	return nil
}
