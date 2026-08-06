package inference_tasks

import (
	"bytes"
	"crynux_relay/models"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func llmResultFileHeader(t *testing.T, content string) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "0.json")
	if err != nil {
		t.Fatalf("create result part: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write result part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	return request.MultipartForm.File["files"][0]
}

func TestReadLLMCompletionTokens(t *testing.T) {
	header := llmResultFileHeader(t, `{"usage":{"completion_tokens":42}}`)
	tokens, err := readLLMCompletionTokens(header)
	if err != nil {
		t.Fatalf("read completion tokens: %v", err)
	}
	if tokens != 42 {
		t.Fatalf("expected 42 completion tokens, got %d", tokens)
	}
}

func TestReadLLMCompletionTokensRejectsInvalidValues(t *testing.T) {
	for _, content := range []string{
		`{}`,
		`{"usage":{}}`,
		`{"usage":{"completion_tokens":-1}}`,
		`{"usage":{"completion_tokens":1.5}}`,
		`{"usage":{"completion_tokens":"42"}}`,
	} {
		header := llmResultFileHeader(t, content)
		if _, err := readLLMCompletionTokens(header); err == nil {
			t.Fatalf("expected invalid completion tokens for %s", content)
		}
	}
}

func TestSelectLLMGroupRefundCalibrationTasksRequiresGroupSuccessAndSameScore(t *testing.T) {
	uploaded := &models.InferenceTask{
		TaskIDCommitment: "0xupload",
		Status:           models.TaskEndGroupSuccess,
		Score:            "0xsame",
	}
	group := []models.InferenceTask{
		{TaskIDCommitment: "0xupload", Status: models.TaskEndGroupSuccess, Score: "0xsame"},
		{TaskIDCommitment: "0xrefund-same", Status: models.TaskEndGroupRefund, Score: "0xsame"},
		{TaskIDCommitment: "0xrefund-diff", Status: models.TaskEndGroupRefund, Score: "0xother"},
		{TaskIDCommitment: "0xinvalid", Status: models.TaskEndInvalidated, Score: "0xsame"},
	}

	selected := selectLLMGroupRefundCalibrationTasks(uploaded, group)
	if len(selected) != 1 {
		t.Fatalf("expected one same-score refund task, got %d", len(selected))
	}
	if selected[0].TaskIDCommitment != "0xrefund-same" {
		t.Fatalf("unexpected refund sample %s", selected[0].TaskIDCommitment)
	}

	uploaded.Status = models.TaskEndSuccess
	if selected := selectLLMGroupRefundCalibrationTasks(uploaded, group); len(selected) != 0 {
		t.Fatalf("non-group-success upload must not select refund samples, got %d", len(selected))
	}
}
