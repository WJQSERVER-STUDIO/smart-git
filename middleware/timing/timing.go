package timing

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PhaseTiming 存储单个阶段的计时信息
type PhaseTiming struct {
	Name string
	Dur  time.Duration
}

// timingData 存储请求的计时数据，使用固定大小的数组优化性能
type timingData struct {
	phaseRecords [8]PhaseTiming // 预分配 8 个阶段存储，大多数场景下足够
	phaseCount   int            // 已记录的阶段数量
	startTime    time.Time      // 请求开始时间
}

// 对象池，用于重用 timingData 对象，减少内存分配和 GC 压力
var pool = sync.Pool{
	New: func() interface{} {
		return new(timingData)
	},
}

// Middleware 返回 Fiber 中间件函数，用于初始化请求计时
//
// 该中间件会在每个请求开始时从对象池获取 timingData 对象，
// 并存储到 Fiber Context 的 Local Storage 中，键名为 "timing"。
// 请求处理完成后，会将 timingData 对象放回对象池。
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 从池中获取计时器
		td := pool.Get().(*timingData)
		td.startTime = time.Now()
		td.phaseCount = 0

		// 存储到 Fiber Context 的 Local Storage
		c.Locals("timing", td)

		// 请求完成后回收对象
		defer func() {
			pool.Put(td)
		}()

		return c.Next()
	}
}

// Record 记录请求处理过程中的阶段耗时
//
// 参数:
//   - c: Fiber Context
//   - name: 阶段名称
//
// 该函数会将指定阶段的名称和从请求开始到现在的耗时记录下来。
// 注意: 最多记录 8 个阶段，超出部分将被忽略。
func Record(c *fiber.Ctx, name string) {
	val := c.Locals("timing")
	if val != nil {
		td, ok := val.(*timingData)
		if !ok {
			log.Println("[Timing Middleware] Error: timingData not found in context locals, incorrect middleware setup?") // 记录警告日志
			return
		}
		if td.phaseCount < len(td.phaseRecords) {
			td.phaseRecords[td.phaseCount].Name = name
			td.phaseRecords[td.phaseCount].Dur = time.Since(td.startTime)
			td.phaseCount++
		}
	}
}

// Get 获取请求的计时结果
//
// 参数:
//   - c: Fiber Context
//
// 返回值:
//   - total: 请求总耗时
//   - phases: 阶段计时信息切片
func Get(c *fiber.Ctx) (total time.Duration, phases []PhaseTiming) {
	val := c.Locals("timing")
	if val != nil {
		td, ok := val.(*timingData)
		if !ok {
			log.Println("[Timing Middleware] Error: timingData not found in context locals, incorrect middleware setup?") // 记录警告日志
			return
		}
		phases = make([]PhaseTiming, 0, td.phaseCount) // 预分配切片容量
		for i := 0; i < td.phaseCount; i++ {
			phases = append(phases, PhaseTiming{
				Name: td.phaseRecords[i].Name,
				Dur:  td.phaseRecords[i].Dur,
			})
		}
		total = time.Since(td.startTime)
	}
	return
}

// FiberMiddleware 是 Middleware 的别名，为了更符合 Fiber 中间件的命名习惯
// Deprecated: Use Middleware instead.
func FiberMiddleware() fiber.Handler {
	return Middleware()
}
