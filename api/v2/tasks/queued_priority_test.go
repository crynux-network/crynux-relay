package tasks

import (
	"context"
	"crynux_relay/models"
	"crynux_relay/service"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resetQueuedPriorityAPISnapshot(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := service.RefreshQueuedTaskPrioritySnapshot(context.Background(), db, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("reset snapshot: %v", err)
	}
}

func TestGetQueuedTaskPriorityEmptyJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetQueuedPriorityAPISnapshot(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v2/tasks/queued/priority", nil)

	resp, err := GetQueuedTaskPriority(c)
	if err != nil {
		t.Fatalf("GetQueuedTaskPriority: %v", err)
	}
	resp.SetMessage("success")
	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := parsed["data"].(map[string]any)
	if data["queued_task_count"].(float64) != 0 {
		t.Fatalf("expected count 0, got %v", data["queued_task_count"])
	}
	if data["highest_priority_gwei"] != nil || data["median_priority_gwei"] != nil || data["lowest_priority_gwei"] != nil {
		t.Fatalf("expected null priorities, got %v", data)
	}
}

func TestGetQueuedTaskPriorityGweiStringsAndNoDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:queued_priority_api?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.InferenceTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	priority := new(big.Int)
	priority.SetString("57447899546", 10)
	task := models.InferenceTask{
		TaskIDCommitment: "api-task",
		Status:           models.TaskQueued,
		TaskType:         models.TaskTypeLLM,
		Priority:         models.BigInt{Int: *priority},
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	asOf := time.Unix(1_700_000_600, 0).UTC()
	if err := service.RefreshQueuedTaskPrioritySnapshot(context.Background(), db, asOf); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v2/tasks/queued/priority", nil)
	resp, err := GetQueuedTaskPriority(c)
	if err != nil {
		t.Fatalf("GetQueuedTaskPriority after db close: %v", err)
	}
	resp.SetMessage("success")
	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	expected := `{"message":"success","data":{"as_of":1700000600,"queued_task_count":1,"highest_priority_gwei":"57","median_priority_gwei":"57","lowest_priority_gwei":"57"}}`
	if string(body) != expected {
		t.Fatalf("unexpected body\nwant %s\ngot  %s", expected, string(body))
	}
}

func TestGetQueuedTaskPriorityPublicRouteHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetQueuedPriorityAPISnapshot(t)

	router := gin.New()
	router.GET("/v2/tasks/queued/priority", tonic.Handler(GetQueuedTaskPriority, 200))
	req := httptest.NewRequest(http.MethodGet, "/v2/tasks/queued/priority", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
}
