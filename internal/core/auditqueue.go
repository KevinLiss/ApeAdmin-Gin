package core

import (
	"context"
	"sync"
	"time"

	"gin-apeadmin/internal/model"

	"gorm.io/gorm"
)

// AuditQueue 操作日志缓冲队列（批量落库 + 关闭 drain）
type AuditQueue struct {
	ch   chan model.SysLog
	db   *gorm.DB
	done chan struct{}
	wg   sync.WaitGroup
}

// NewAuditQueue 创建日志队列
func NewAuditQueue(db *gorm.DB, queueSize int) *AuditQueue {
	if queueSize <= 0 {
		queueSize = 1024
	}
	return &AuditQueue{
		ch:   make(chan model.SysLog, queueSize),
		db:   db,
		done: make(chan struct{}),
	}
}

// Start 启动消费者 goroutine
func (q *AuditQueue) Start() {
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		ticker := time.NewTicker(time.Second)
		batch := make([]model.SysLog, 0, 50)
		for {
			select {
			case item := <-q.ch:
				batch = append(batch, item)
				if len(batch) >= 50 {
					q.flush(&batch)
				}
			case <-ticker.C:
				q.flush(&batch)
			case <-q.done:
				q.drainAndFlush(&batch)
				return
			}
		}
	}()
}

// Enqueue 投递日志（队列满则丢弃，不阻塞）
func (q *AuditQueue) Enqueue(entry model.SysLog) {
	select {
	case q.ch <- entry:
	default:
		// 队列满，丢弃并计数（生产环境可接入 metrics）
	}
}

// SyncWrite 同步写入（敏感操作使用）
func (q *AuditQueue) SyncWrite(entry model.SysLog) error {
	return q.db.Create(&entry).Error
}

// Shutdown 优雅关闭：停止接收 → 抽干 → flush
func (q *AuditQueue) Shutdown(ctx context.Context) {
	close(q.done)
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		// 超预算，放弃剩余并记录
	}
}

func (q *AuditQueue) flush(batch *[]model.SysLog) {
	if len(*batch) == 0 {
		return
	}
	q.db.Create(batch)
	*batch = (*batch)[:0]
}

func (q *AuditQueue) drainAndFlush(batch *[]model.SysLog) {
	for {
		select {
		case item := <-q.ch:
			*batch = append(*batch, item)
		default:
			q.flush(batch)
			return
		}
	}
}
