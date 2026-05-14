package browser

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Task 抓取任务
type Task struct {
	// URL 目标地址
	URL string

	// Chain 降级链 (如 [TypeSurf, TypeRodHeadless])
	Chain []Type
}

// TaskResult 任务执行结果
type TaskResult struct {
	// URL 目标地址
	URL string `json:"url"`

	// FetchResult 抓取结果 (nil if error)
	FetchResult *FetchResult `json:"fetch_result"`

	// Err 错误 (如果有)
	Err error `json:"-"`

	// Duration 执行耗时
	Duration time.Duration `json:"duration"`
}

// WorkerConfig Worker 配置
type WorkerConfig struct {
	// Concurrency 并发 worker 数量
	Concurrency int

	// RateLimit 每秒最大请求数 (0 表示不限制)
	RateLimit int

	// OnResult 每完成一个任务的回调 (可选，在 goroutine 中调用)
	// idx 为任务在原始列表中的索引
	OnResult func(idx int, result TaskResult)
}

// DefaultWorkerConfig 默认 Worker 配置
var DefaultWorkerConfig = WorkerConfig{
	Concurrency: 4,
}

// Worker 并发任务调度器
//
// 批量执行抓取任务，通过信号量控制并发数，支持速率限制和结果回调。
// 结果按原始任务顺序返回。
type Worker struct {
	pool     *Pool
	cfg      WorkerConfig
	detector BlockDetector
}

// NewWorker 创建 Worker
func NewWorker(pool *Pool, detector BlockDetector, cfg WorkerConfig) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultWorkerConfig.Concurrency
	}
	return &Worker{
		pool:     pool,
		cfg:      cfg,
		detector: detector,
	}
}

// Run 并发执行任务列表，返回所有结果 (保持原始顺序)
//
// ctx 取消时，尚未开始的任务不会执行，已开始的任务会通过 ctx 传播取消信号。
func (w *Worker) Run(ctx context.Context, tasks []Task) []TaskResult {
	results := make([]TaskResult, len(tasks))
	sem := make(chan struct{}, w.cfg.Concurrency)
	var wg sync.WaitGroup

	// 速率限制器
	var limiter <-chan time.Time
	if w.cfg.RateLimit > 0 {
		interval := time.Second / time.Duration(w.cfg.RateLimit)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		limiter = ticker.C
	}

	for i, task := range tasks {
		if ctx.Err() != nil {
			// 填充剩余任务为取消错误
			for j := i; j < len(tasks); j++ {
				results[j] = TaskResult{URL: tasks[j].URL, Err: ctx.Err()}
			}
			break
		}

		// 速率限制
		if limiter != nil {
			select {
			case <-limiter:
			case <-ctx.Done():
				results[i] = TaskResult{URL: task.URL, Err: ctx.Err()}
				continue
			}
		}

		wg.Add(1)
		go w.executeTask(ctx, task, i, sem, results, &wg)
	}

	wg.Wait()
	return results
}

// executeTask 执行单个任务
func (w *Worker) executeTask(ctx context.Context, t Task, idx int, sem chan struct{}, results []TaskResult, wg *sync.WaitGroup) {
	defer wg.Done()

	// 获取信号量
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		results[idx] = TaskResult{URL: t.URL, Err: ctx.Err()}
		return
	}
	defer func() { <-sem }()

	slog.Debug("browser: worker task start", "index", idx, "url", t.URL)
	start := time.Now()

	fr, err := w.pool.FetchWithFallback(ctx, t.URL, t.Chain, w.detector)
	dur := time.Since(start)

	results[idx] = TaskResult{
		URL:         t.URL,
		FetchResult: fr,
		Err:         err,
		Duration:    dur,
	}

	if err != nil {
		slog.Warn("browser: worker task failed",
			"index", idx, "url", t.URL, "error", err, "duration", dur)
	} else {
		slog.Info("browser: worker task done",
			"index", idx, "url", t.URL,
			"type", fr.FinalType.String(), "duration", dur)
	}

	// 结果回调
	if w.cfg.OnResult != nil {
		w.cfg.OnResult(idx, results[idx])
	}
}
