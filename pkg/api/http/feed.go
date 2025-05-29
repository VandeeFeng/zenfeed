package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/storage/kv"
)

// FeedStatusRequest 表示更新 feed 阅读状态的请求
type FeedStatusRequest struct {
	ReadStatus string `json:"read_status"` // "read", "unread", "deleted"
}

// isValidReadStatus 检查阅读状态值是否有效
func isValidReadStatus(status string) bool {
	validStatuses := map[string]bool{
		"read":   true,
		"unread": true,
	}
	return validStatuses[status]
}

// handleUpdateFeedStatus 处理更新 feed 阅读状态的请求
func (s *server) handleUpdateFeedStatus(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径中提取 feed ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	feedIDStr := parts[2] // 获取 feed ID
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var req FeedStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证状态值
	if !isValidReadStatus(req.ReadStatus) {
		http.Error(w, "Invalid read status value", http.StatusBadRequest)
		return
	}

	// 构造 KV 存储的键
	key := fmt.Sprintf("feed:%d:read_status", feedID)

	// 存储状态
	if err := s.Dependencies().API.KVStorage().Set(r.Context(), key, req.ReadStatus, 0); err != nil {
		http.Error(w, "Failed to update read status", http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"read_status": req.ReadStatus,
	})
}

// handleGetFeedStatus 处理获取 feed 阅读状态的请求
func (s *server) handleGetFeedStatus(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径中提取 feed ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	feedIDStr := parts[2] // 获取 feed ID
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	// 构造 KV 存储的键
	key := fmt.Sprintf("feed:%d:read_status", feedID)

	// 获取状态
	status, err := s.Dependencies().API.KVStorage().Get(r.Context(), key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			// 如果没有找到状态，默认为 "unread"
			status = "unread"
		} else {
			http.Error(w, "Failed to get read status", http.StatusInternalServerError)
			return
		}
	}

	// 返回状态
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"read_status": status,
	})
}

// handleDeleteFeed 处理删除feed的请求
func (s *server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取feed ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL path", http.StatusBadRequest)
		return
	}
	feedIDStr := parts[2]
	feedID, err := strconv.ParseUint(feedIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", http.StatusBadRequest)
		return
	}

	// 检查feed是否存在
	exists, err := s.Dependencies().API.FeedStorage().Exists(r.Context(), feedID, time.Now())
	if err != nil {
		http.Error(w, "Failed to check feed existence", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Feed not found", http.StatusNotFound)
		return
	}

	// 删除feed
	if err := s.Dependencies().API.FeedStorage().Delete(r.Context(), feedID); err != nil {
		http.Error(w, "Failed to delete feed", http.StatusInternalServerError)
		return
	}

	// 删除feed的状态
	statusKey := fmt.Sprintf("feed:%d:read_status", feedID)
	if err := s.Dependencies().API.KVStorage().Delete(r.Context(), statusKey); err != nil && !errors.Is(err, kv.ErrNotFound) {
		// 如果状态不存在就忽略错误
		http.Error(w, "Failed to delete feed status", http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Feed deleted successfully",
	})
}
