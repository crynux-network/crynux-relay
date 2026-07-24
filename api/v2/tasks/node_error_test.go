package tasks

import (
	"bytes"
	"context"
	"crynux_relay/api/v2/response"
	"crynux_relay/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const nodeTaskErrorTestPrivateKey = "420fcabfd5dbb55215490693062e6e530840c64de837d071f0d9da21aaac861e"

func newNodeTaskErrorAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}, &models.NodeTaskError{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newSignedNodeTaskErrorInput(t *testing.T, privateKeyHex string, taskIDCommitment string) *NodeTaskErrorInputWithSignature {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	timestamp := time.Now().Unix()
	input := NodeTaskErrorInput{
		NodeTaskErrorSigningInput: NodeTaskErrorSigningInput{
			NodeAddress:      crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
			TaskIDCommitment: taskIDCommitment,
			TaskArgs:         `{"prompt":"full task args","steps":20}`,
			ErrorType:        "TaskExecutionError",
			Message:          "worker execution failed",
			StackTrace:       "Traceback (most recent call last):\n  File \"worker.py\", line 7\nRuntimeError: boom",
		},
		CapturedAt: timestamp - 10,
	}
	return signNodeTaskErrorInput(t, privateKeyHex, input, timestamp)
}

func signNodeTaskErrorInput(t *testing.T, privateKeyHex string, input NodeTaskErrorInput, timestamp int64) *NodeTaskErrorInputWithSignature {
	t.Helper()
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	dataBytes, err := json.Marshal(input.NodeTaskErrorSigningInput)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	var canonical interface{}
	if err := json.Unmarshal(dataBytes, &canonical); err != nil {
		t.Fatalf("failed to canonicalize input: %v", err)
	}
	dataBytes, err = json.Marshal(canonical)
	if err != nil {
		t.Fatalf("failed to marshal canonical input: %v", err)
	}
	hash := crypto.Keccak256Hash(append(dataBytes, []byte(strconv.FormatInt(timestamp, 10))...))
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("failed to sign input: %v", err)
	}
	return &NodeTaskErrorInputWithSignature{
		NodeTaskErrorInput: input,
		Timestamp:          timestamp,
		Signature:          hexutil.Encode(signature),
	}
}

func TestReportNodeTaskErrorAcceptsTerminalTaskAndIsIdempotent(t *testing.T) {
	db := newNodeTaskErrorAPITestDB(t)
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "task-terminal")
	task := models.InferenceTask{
		TaskIDCommitment: input.TaskIDCommitment,
		SelectedNode:     input.NodeAddress,
		Status:           models.TaskEndSuccess,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if _, err := reportNodeTaskError(context.Background(), db, input.TaskIDCommitment, input); err != nil {
			t.Fatalf("report attempt %d failed: %v", attempt+1, err)
		}
	}

	var records []models.NodeTaskError
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("failed to query reports: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one idempotent record, got %d", len(records))
	}
	if records[0].TaskArgs != input.TaskArgs || records[0].StackTrace != input.StackTrace {
		t.Fatalf("expected complete diagnostic content, got %+v", records[0])
	}
	var storedTask models.InferenceTask
	if err := db.First(&storedTask, task.ID).Error; err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if storedTask.Status != models.TaskEndSuccess {
		t.Fatalf("expected diagnostic report not to change task status, got %d", storedTask.Status)
	}
}

func TestReportNodeTaskErrorRejectsSignerThatIsNotSelectedNode(t *testing.T) {
	db := newNodeTaskErrorAPITestDB(t)
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "task-wrong-node")
	task := models.InferenceTask{
		TaskIDCommitment: input.TaskIDCommitment,
		SelectedNode:     "0x1111111111111111111111111111111111111111",
		Status:           models.TaskStarted,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	_, err := reportNodeTaskError(context.Background(), db, input.TaskIDCommitment, input)
	validationErr, ok := err.(*response.ValidationErrorResponse)
	if !ok || validationErr.GetFieldName() != "signature" {
		t.Fatalf("expected signer validation error, got %v", err)
	}
}

func TestReportNodeTaskErrorRejectsMissingTask(t *testing.T) {
	db := newNodeTaskErrorAPITestDB(t)
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "missing-task")

	_, err := reportNodeTaskError(context.Background(), db, input.TaskIDCommitment, input)
	if _, ok := err.(*response.NotFoundErrorResponse); !ok {
		t.Fatalf("expected task not found error, got %v", err)
	}
}

func TestReportNodeTaskErrorRejectsMismatchedSubmittedAddress(t *testing.T) {
	db := newNodeTaskErrorAPITestDB(t)
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "task-address-mismatch")
	unsignedInput := input.NodeTaskErrorInput
	unsignedInput.NodeAddress = "0x2222222222222222222222222222222222222222"
	input = signNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, unsignedInput, input.Timestamp)

	_, err := reportNodeTaskError(context.Background(), db, input.TaskIDCommitment, input)
	validationErr, ok := err.(*response.ValidationErrorResponse)
	if !ok || validationErr.GetFieldName() != "signature" {
		t.Fatalf("expected signature validation error, got %v", err)
	}
}

func TestReportNodeTaskErrorAcceptsCapturedAtOutsideSignature(t *testing.T) {
	db := newNodeTaskErrorAPITestDB(t)
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "task-unsigned-capture-time")
	input.CapturedAt = 1_700_000_000
	task := models.InferenceTask{
		TaskIDCommitment: input.TaskIDCommitment,
		SelectedNode:     input.NodeAddress,
		Status:           models.TaskStarted,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if _, err := reportNodeTaskError(context.Background(), db, input.TaskIDCommitment, input); err != nil {
		t.Fatalf("expected captured_at not to affect signature validation: %v", err)
	}
	var record models.NodeTaskError
	if err := db.First(&record).Error; err != nil {
		t.Fatalf("failed to load stored report: %v", err)
	}
	if record.CapturedAt != input.CapturedAt {
		t.Fatalf("expected captured_at %d to be stored, got %d", input.CapturedAt, record.CapturedAt)
	}
}

func TestReportNodeTaskErrorRejectsPathBodyCommitmentMismatch(t *testing.T) {
	input := newSignedNodeTaskErrorInput(t, nodeTaskErrorTestPrivateKey, "signed-task")
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	tonic.SetErrorHook(response.TonicErrorResponse)
	engine := gin.New()
	engine.POST("/v2/tasks/:task_id_commitment/node_error", tonic.Handler(ReportNodeTaskError, http.StatusOK))
	req := httptest.NewRequest(http.MethodPost, "/v2/tasks/path-task/node_error", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var validationErr response.ValidationErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &validationErr); err != nil {
		t.Fatalf("failed to decode validation response: %v", err)
	}
	if validationErr.GetFieldName() != "task_id_commitment" {
		t.Fatalf("expected task commitment validation error, got %s", recorder.Body.String())
	}
}
