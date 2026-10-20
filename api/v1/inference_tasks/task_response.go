package inference_tasks

import (
	"crynux_relay/api/v1/response"
	"crynux_relay/models"
	"encoding/json"
	"time"
)

// requestedModelCacheHashHex returns the hex requested model cache hash of the
// task, or an empty string when the task has none.
func requestedModelCacheHashHex(task *models.InferenceTask) (string, error) {
	if len(task.RequestedModelCacheHash) == 0 {
		return "", nil
	}
	hash, err := task.RequestedModelCacheHash32()
	if err != nil {
		return "", err
	}
	return hash.Hex(), nil
}

type InferenceTask struct {
	Sequence         uint64            `json:"sequence"`
	TaskArgs         string            `json:"task_args"`
	TaskIDCommitment string            `json:"task_id_commitment"`
	Creator          string            `json:"creator"`
	SamplingSeed     string            `json:"sampling_seed"`
	Nonce            string            `json:"nonce"`
	Status           models.TaskStatus `json:"status"`
	TaskType         models.TaskType   `json:"task_type"`
	TaskVersion      string            `json:"task_version"`
	Timeout          uint64            `json:"timeout"`
	MinVRAM          uint64            `json:"min_vram"`
	RequiredGPU      string            `json:"required_gpu"`
	RequiredGPUVRAM  uint64            `json:"required_gpu_vram"`
	TaskFee          models.BigInt     `json:"task_fee"`
	TaskSize         uint64            `json:"task_size"`
	ModelIDs         []string          `json:"model_ids"`
	// ExecutionModelFiles lists the exact model files frozen for the execution once the task has started.
	ExecutionModelFiles []models.ModelFileEntry `json:"execution_model_files,omitempty"`
	// RequestedModelCacheHash is the hex identity of the in-memory model the task requests.
	RequestedModelCacheHash string `json:"requested_model_cache_hash,omitempty"`
	// RequestedModelCacheConfig is the canonical JSON the hash is computed from.
	RequestedModelCacheConfig json.RawMessage `json:"requested_model_cache_config,omitempty"`
	AbortReason         models.TaskAbortReason  `json:"abort_reason"`
	TaskError           models.TaskError        `json:"task_error"`
	Score               string                  `json:"score"`
	QOSScore            uint64                  `json:"qos_score"`
	SelectedNode        string                  `json:"selected_node"`
	CreateTime          *time.Time              `json:"create_time,omitempty"`
	StartTime           *time.Time              `json:"start_time,omitempty"`
	ScoreReadyTime      *time.Time              `json:"score_ready_time,omitempty"`
	ValidatedTime       *time.Time              `json:"validated_time,omitempty"`
	ResultUploadedTime  *time.Time              `json:"result_uploaded_time,omitempty"`
}

type TaskResponse struct {
	response.Response
	Data *InferenceTask `json:"data"`
}

type TasksResponse struct {
	response.Response
	Data []InferenceTask `json:"data"`
}
