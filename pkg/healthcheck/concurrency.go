package healthcheck

import "github.com/One-Piecs/proxypool/config"

// healthcheckConcurrency 健康检查/中转检测的并发度（来自配置 healthcheck-concurrency，默认 200）。
// 原硬编码 500 并发对目标源压力较大，改为可配置。
func healthcheckConcurrency() int {
	c := config.Config().HealthcheckConcurrency
	if c <= 0 {
		return 200
	}
	return c
}
