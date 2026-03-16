package api

import (
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// normalizeUploadURLForClient converts stored upload paths (e.g. "uploads/..." or "/uploads/...")
// into a URL path that can be used directly by browsers.
//
// - For local uploads stored as "uploads/...", it returns "/uploads/...".
// - For already-normalized "/uploads/...", it returns as-is.
// - For external URLs (http/https), it returns as-is.
func normalizeUploadURLForClient(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if strings.HasPrefix(p, "/uploads/") {
		return p
	}
	if strings.HasPrefix(p, "uploads/") {
		return "/" + p
	}
	return p
}

// normalizeImageURLForStorage 规范化图片URL用于存储。
// 它会将完整URL（带域名和签名）或带前导斜杠的路径转换为相对路径（如 "uploads/..."）。
func normalizeImageURLForStorage(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	// 1. 处理完整 URL 或带查询参数的情况
	// 寻找 /uploads/ 的位置
	if idx := strings.Index(p, "/uploads/"); idx != -1 {
		p = p[idx+1:] // 保留 "uploads/..."
	}

	// 2. 去除查询参数（如 ?expires=...&sig=...）
	if idx := strings.Index(p, "?"); idx != -1 {
		p = p[:idx]
	}

	// 3. 规范化路径前缀
	p = strings.TrimPrefix(p, "/")

	// 如果不以 uploads/ 开头，但属于已知目录，则补全
	if !strings.HasPrefix(p, "uploads/") {
		if strings.HasPrefix(p, "merchants/") || strings.HasPrefix(p, "riders/") ||
			strings.HasPrefix(p, "operators/") || strings.HasPrefix(p, "reviews/") ||
			strings.HasPrefix(p, "public/") {
			p = "uploads/" + p
		}
	}

	return p
}

// deleteStoredImageAsync 将旧图片文件加入有界删除队列，由 imageDeleteWorker 异步执行删除。
// 对空路径、外部 URL 或 uploads/ 路径以外的路径为空操作。
func (server *Server) deleteStoredImageAsync(storedURL string) {
	if storedURL == "" {
		return
	}
	if strings.HasPrefix(storedURL, "http://") || strings.HasPrefix(storedURL, "https://") {
		return
	}
	// 规范化为相对路径 "uploads/..."
	path := strings.TrimPrefix(storedURL, "/")
	if !strings.HasPrefix(path, "uploads/") {
		return
	}
	server.imageDeleter.submit(path)
}

// ==================== imageDeleteWorker ====================

const (
	// imageDeleteWorkerCount 是并发执行文件删除的 goroutine 数量。
	// 文件删除属于本地 I/O，2 个 worker 已可在高并发场景下平稳消费队列。
	imageDeleteWorkerCount = 2
	// imageDeleteQueueSize 是待删除任务的最大队列深度。
	// 超出此深度时新任务被丢弃并记录警告，防止在极端场景下内存无限增长。
	imageDeleteQueueSize = 256
)

// imageDeleteWorker 管理一组有界 goroutine，负责异步删除本地图片文件。
// 通过有界 channel 提供背压保护，通过 sync.WaitGroup 支持优雅关闭。
type imageDeleteWorker struct {
	jobs chan string
	wg   sync.WaitGroup
}

// newImageDeleteWorker 创建并启动 imageDeleteWorker。
// 调用方负责在程序退出时调用 shutdown()。
func newImageDeleteWorker() *imageDeleteWorker {
	w := &imageDeleteWorker{
		jobs: make(chan string, imageDeleteQueueSize),
	}
	for range imageDeleteWorkerCount {
		w.wg.Add(1)
		go w.run()
	}
	return w
}

func (w *imageDeleteWorker) run() {
	defer w.wg.Done()
	for path := range w.jobs {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Warn().Str("path", path).Err(err).Msg("delete old image file failed")
		}
	}
}

// submit 将文件路径加入删除队列。若队列已满则丢弃并记录警告（非阻塞）。
func (w *imageDeleteWorker) submit(path string) {
	select {
	case w.jobs <- path:
	default:
		log.Warn().Str("path", path).Msg("image delete queue full, dropping delete job")
	}
}

// shutdown 关闭任务通道并等待所有待处理删除任务完成后返回。
// 应在 Server.Shutdown() 中调用，确保优雅退出不丢失已入队的任务。
func (w *imageDeleteWorker) shutdown() {
	close(w.jobs)
	w.wg.Wait()
}
