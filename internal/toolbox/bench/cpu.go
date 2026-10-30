// Package bench 实现 CPU 与磁盘吞吐量的基准测试。
//
// 迁移自 legacy/python 中的 Python 跑分脚本：
//   - CPU 单核基准测试脚本.py：质数查找 + 三角函数混合负载
//   - 磁盘 IO 速度测试 Python 脚本.py：顺序写入 + fsync + 顺序读取
package bench

import (
	"fmt"
	"math"
	"time"
)

// CPUOptions 控制 CPU 基准测试的规模。
type CPUOptions struct {
	// IntLimit 是整数测试（质数查找）的上限。
	IntLimit int
	// FloatIters 是浮点测试的迭代次数。
	FloatIters int
	// BaselineInt / BaselineFloat 是参考机器的耗时基准（秒），用于换算分数。
	BaselineInt   float64
	BaselineFloat float64
	// Warmup 为 true 时先跑一小段预热，唤醒 CPU 到高频。
	Warmup bool
}

// DefaultCPUOptions 返回与原 Python 脚本一致的默认参数。
func DefaultCPUOptions() CPUOptions {
	return CPUOptions{
		IntLimit:      50000,
		FloatIters:    3000000,
		BaselineInt:   1.25,
		BaselineFloat: 1.15,
		Warmup:        true,
	}
}

// CPUResult 是 CPU 基准测试的结果。
type CPUResult struct {
	IntDuration   time.Duration
	IntScore      int
	FloatDuration time.Duration
	FloatScore    int
	OverallScore  int
}

// RunCPU 执行 CPU 基准测试。
func RunCPU(opts CPUOptions) CPUResult {
	if opts.IntLimit <= 0 {
		opts.IntLimit = DefaultCPUOptions().IntLimit
	}
	if opts.FloatIters <= 0 {
		opts.FloatIters = DefaultCPUOptions().FloatIters
	}
	if opts.BaselineInt <= 0 {
		opts.BaselineInt = DefaultCPUOptions().BaselineInt
	}
	if opts.BaselineFloat <= 0 {
		opts.BaselineFloat = DefaultCPUOptions().BaselineFloat
	}

	if opts.Warmup {
		// 预热：让 CPU 从低频醒来，避免第一项测试被降频拖累。
		benchmarkInt(10000)
	}

	intDur := benchmarkInt(opts.IntLimit)
	floatDur := benchmarkFloat(opts.FloatIters)

	intScore := score(intDur.Seconds(), opts.BaselineInt)
	floatScore := score(floatDur.Seconds(), opts.BaselineFloat)

	return CPUResult{
		IntDuration:   intDur,
		IntScore:      intScore,
		FloatDuration: floatDur,
		FloatScore:    floatScore,
		OverallScore:  (intScore + floatScore) / 2,
	}
}

// benchmarkInt 用试除法统计 limit 以内的质数个数。
//
// 保留 Python 原版的朴素实现：不做任何优化，只为消耗 CPU 周期，
// 这样不同语言的分数才可比。
func benchmarkInt(limit int) time.Duration {
	start := time.Now()
	count := 0
	for num := 2; num < limit; num++ {
		isPrime := true
		for i := 2; i*i <= num; i++ {
			if num%i == 0 {
				isPrime = false
				break
			}
		}
		if isPrime {
			count++
		}
	}
	elapsed := time.Since(start)
	// 把结果算进去，避免编译器把整个循环优化掉。
	primeCountSink = count
	// 极短循环在某些平台会量出 0 耗时，此时兜底为 1ns，
	// 避免后续除以 0 得出无意义的分数。
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	return elapsed
}

// benchmarkFloat 执行高强度浮点混合负载（sin/cos/sqrt）。
func benchmarkFloat(iterations int) time.Duration {
	start := time.Now()
	val := 0.0
	for i := 1; i < iterations; i++ {
		val += math.Sin(float64(i))*math.Cos(float64(i)) + math.Sqrt(float64(i))
	}
	elapsed := time.Since(start)
	floatSink = val
	// 极短循环在某些平台（Windows 的低精度计时器）会量出 0，
	// 此时用外层墙钟时间兜底，避免分数除以 0。
	if elapsed <= 0 {
		elapsed = time.Since(start)
	}
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	return elapsed
}

// primeCountSink / floatSink 是防止编译器优化掉基准循环的全局变量。
var (
	primeCountSink int
	floatSink      float64
)

// score 按 (基准耗时 / 实际耗时) * 1000 换算相对分数。
func score(duration, baseline float64) int {
	if duration <= 0 {
		return 0
	}
	return int((baseline / duration) * 1000)
}

// FormatCPUResult 把 CPU 测试结果渲染成人类可读的报告。
func FormatCPUResult(systemInfo string, cores int, r CPUResult) string {
	out := ""
	out += "=== CPU 单核性能基准测试 (Go) ===\n"
	out += fmt.Sprintf("系统信息: %s\n", systemInfo)
	out += fmt.Sprintf("逻辑核心: %d 核\n", cores)
	out += "---------------------------------------\n"
	out += "1. 整数 (Integer) 测试...\n"
	out += fmt.Sprintf("    耗时: %.4f 秒\n", r.IntDuration.Seconds())
	out += fmt.Sprintf("    整数得分: %d\n", r.IntScore)
	out += "---------------------------------------\n"
	out += "2. 浮点 (Float) 测试...\n"
	out += fmt.Sprintf("    耗时: %.4f 秒\n", r.FloatDuration.Seconds())
	out += fmt.Sprintf("    浮点得分: %d\n", r.FloatScore)
	out += "=======================================\n"
	out += fmt.Sprintf("综合单核分数: %d\n", r.OverallScore)
	out += "=======================================\n"
	return out
}
