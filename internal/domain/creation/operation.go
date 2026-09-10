package creation

import "time"

// OperationStatus 是创作操作回执的持久化状态，只按核实结果转移。
type OperationStatus string

const (
	OperationReserved  OperationStatus = "reserved"
	OperationRunning   OperationStatus = "running"
	OperationSucceeded OperationStatus = "succeeded"
	OperationFailed    OperationStatus = "failed"
)

// Operation 记录一次付费或写入创作命令的持久化回执，供跨重启幂等去重与恢复对账。
type Operation struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	BranchID       string `json:"branchId"`
	Command        string `json:"command"`
	IdempotencyKey string `json:"idempotencyKey"`
	ArgumentsHash  string `json:"argumentsHash"`
	// FrozenInputDigest 是运行启动时已确认的上游输入摘要，恢复物化时不得用实时状态替代。
	FrozenInputDigest string          `json:"frozenInputDigest"`
	Status            OperationStatus `json:"status"`
	RunID             string          `json:"runId"`
	CandidateID       string          `json:"candidateId"`
	ErrorCode         string          `json:"errorCode"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

// Terminal 表示操作已核实到达终态，不再占用运行语义。
func (status OperationStatus) Terminal() bool {
	return status == OperationSucceeded || status == OperationFailed
}
