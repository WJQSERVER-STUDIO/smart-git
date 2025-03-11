package loggin

import (
	"smart-git/middleware/timing"
	"time"

	"github.com/WJQSERVER-STUDIO/go-utils/logger"
	"github.com/gofiber/fiber/v2"
)

var (
	logw       = logger.Logw
	LogDump    = logger.LogDump
	logDebug   = logger.LogDebug
	logInfo    = logger.LogInfo
	logWarning = logger.LogWarning
	logError   = logger.LogError
)

// 日志中间件
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 处理请求
		err := c.Next()

		var timingResults time.Duration

		// 获取计时结果
		timingResults, _ = timing.Get(c)

		// 记录日志 IP METHOD URL USERAGENT PROTOCOL STATUS TIMING
		logInfo("%s %s %s %s %d %s ", c.IP(), c.Method(), c.Path(), c.Get("User-Agent"), c.Response().StatusCode(), timingResults)

		return err
	}
}
