package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"smart-git/config"

	"github.com/WJQSERVER-STUDIO/go-utils/logger"
)

var (
	cfgfile string = "./config/config.toml"
	cfg     *config.Config
)

var (
	logw       = logger.Logw
	logDump    = logger.LogDump
	logDebug   = logger.LogDebug
	logInfo    = logger.LogInfo
	logWarning = logger.LogWarning
	logError   = logger.LogError
)

func ReadFlag() {
	cfgfilePtr := flag.String("cfg", "./config/config.toml", "config file path")
	flag.Parse()
	cfgfile = *cfgfilePtr
}

func loadConfig() {
	var err error
	// 初始化配置
	cfg, err = config.LoadConfig(cfgfile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Printf("Loaded config: %v\n", cfg)
}

// init
func init() {
	ReadFlag()
	loadConfig()

	// 创建根目录 os
	err := os.MkdirAll(cfg.Server.BaseDir, 0755)
	if err != nil {
		fmt.Printf("Fail to create dir: %v\n", err)
		return
	}

	err = logger.Init(cfg.Log.LogFilePath, cfg.Log.MaxLogSize)
	if err != nil {
		fmt.Printf("Fail to init logger: %v\n", err)
		return
	}

}

func main() {

	// 运行HTTP Git Server
	err := RunHTTP(":8080", cfg.Server.BaseDir)
	if err != nil {
		fmt.Printf("Fail to run http: %v\n", err)
		return
	}

}
