package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	wanfcodec "github.com/WJQSERVER/wanf"
)

type Config struct {
	Server   ServerConfig
	Log      LogConfig
	Database DatabaseConfig
	Cache    CacheConfig
}

type ServerConfig struct {
	Host     string `toml:"host" wanf:"host"`
	Port     int    `toml:"port" wanf:"port"`
	BaseDir  string `toml:"baseDir" wanf:"baseDir"`
	MemLimit int64  `toml:"memLimit" wanf:"memLimit"`
}

type LogConfig struct {
	LogFilePath string `toml:"logfilepath" wanf:"logfilepath"`
	MaxLogSize  int    `toml:"maxlogsize" wanf:"maxlogsize"`
	Level       string `toml:"level" wanf:"level"`
}

type DatabaseConfig struct {
	Path string `toml:"path" wanf:"path"` // bolt file path
}

/*
[cache]
expire = "30m"
expireEx = "10m"
*/
type CacheConfig struct {
	Expire   time.Duration `toml:"expire" wanf:"expire"`
	ExpireEx time.Duration `toml:"expireEx" wanf:"expireEx"`
}

// LoadConfig 从 WANF/TOML 配置文件加载配置，WANF 优先
func LoadConfig(filePath string) (*Config, error) {
	resolvedPath, err := resolveConfigPath(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	switch filepath.Ext(resolvedPath) {
	case ".wanf":
		file, err := os.Open(resolvedPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		decoder := wanfcodec.NewNeoDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return nil, err
		}
	default:
		if _, err := toml.DecodeFile(resolvedPath, &config); err != nil {
			return nil, err
		}
	}
	return &config, nil
}

func resolveConfigPath(filePath string) (string, error) {
	if FileExists(filePath) {
		return filePath, nil
	}

	if filepath.Ext(filePath) != "" {
		return "", fmt.Errorf("config file not found: %s", filePath)
	}

	wanfPath := filePath + ".wanf"
	if FileExists(wanfPath) {
		return wanfPath, nil
	}

	tomlPath := filePath + ".toml"
	if FileExists(tomlPath) {
		return tomlPath, nil
	}

	return "", fmt.Errorf("config file not found: %s (.wanf or .toml)", filePath)
}

// 写入配置文件
func (c *Config) WriteConfig(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(c)
}

// 检测文件是否存在
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

/*
[server]
host = "0.0.0.0"
port = 8080
baseDir = "/data/smart-git/repos"
memLimit = 0 #MB

[log]
logfilepath = "/data/smart-git/log/smart-git.log"
maxlogsize = 5 # MB
level = "info" # dump, debug, info, warn, error, none

[Database]
path = "/data/smart-git/db/smart-git.db"

[cache]
expire = "1h"
expireEx = "10m"
*/
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "0.0.0.0",
			Port:     8080,
			BaseDir:  "/data/smart-git/repos",
			MemLimit: 0,
		},
		Log: LogConfig{
			LogFilePath: "/data/smart-git/log/smart-git.log",
			MaxLogSize:  5,
			Level:       "info",
		},
		Database: DatabaseConfig{
			Path: "/data/smart-git/db/smart-git.db",
		},
		Cache: CacheConfig{
			Expire:   time.Hour,
			ExpireEx: 10 * time.Minute,
		},
	}
}
