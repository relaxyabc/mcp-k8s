package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// PendingOperation 待确认的操作
type PendingOperation struct {
	ID        string      // 操作唯一标识
	Type      string      // 操作类型：list_resources, get_resource, read_pod_logs
	Params    interface{} // 原始请求参数
	Message   string      // 用户友好的描述信息
	CreatedAt time.Time   // 创建时间
	ExpiresAt time.Time   // 过期时间
}

var (
	pendingOps = make(map[string]*PendingOperation)
	opsMutex   sync.RWMutex
)

// CreateConfirmation 创建待确认操作
func CreateConfirmation(opType string, params interface{}, message string) *PendingOperation {
	opsMutex.Lock()
	defer opsMutex.Unlock()

	op := &PendingOperation{
		ID:        generateOpID(opType, params),
		Type:      opType,
		Params:    params,
		Message:   message,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	pendingOps[op.ID] = op
	return op
}

// ValidateConfirmation 验证并返回待确认操作（验证后删除）
func ValidateConfirmation(opID string) (*PendingOperation, bool) {
	opsMutex.Lock()
	defer opsMutex.Unlock()

	op, exists := pendingOps[opID]
	if !exists || time.Now().After(op.ExpiresAt) {
		return nil, false
	}
	delete(pendingOps, opID)
	return op, true
}

// GetOperation 获取待确认操作（不删除）
func GetOperation(opID string) (*PendingOperation, bool) {
	opsMutex.RLock()
	defer opsMutex.RUnlock()

	op, exists := pendingOps[opID]
	if !exists || time.Now().After(op.ExpiresAt) {
		return nil, false
	}
	return op, true
}

// generateOpID 生成操作唯一标识
func generateOpID(opType string, params interface{}) string {
	data := fmt.Sprintf("%s:%v:%d", opType, params, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// CleanExpired 清理过期操作
func CleanExpired() {
	opsMutex.Lock()
	defer opsMutex.Unlock()

	now := time.Now()
	for id, op := range pendingOps {
		if now.After(op.ExpiresAt) {
			delete(pendingOps, id)
		}
	}
}