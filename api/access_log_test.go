package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type logEntryHook struct {
	entries []*logrus.Entry
}

func (h *logEntryHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *logEntryHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, entry)
	return nil
}

func TestAccessLoggerLevels(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantLevel  logrus.Level
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			wantLevel:  logrus.DebugLevel,
		},
		{
			name:       "client error",
			statusCode: http.StatusNotFound,
			wantLevel:  logrus.DebugLevel,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantLevel:  logrus.DebugLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			hook := &logEntryHook{}
			logger.AddHook(hook)

			engine := gin.New()
			engine.Use(AccessLogger(logger))
			engine.GET("/test", func(c *gin.Context) {
				c.Status(tc.statusCode)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			resp := httptest.NewRecorder()
			engine.ServeHTTP(resp, req)

			if len(hook.entries) != 1 {
				t.Fatalf("expected one log entry, got %d", len(hook.entries))
			}
			if hook.entries[0].Level != tc.wantLevel {
				t.Fatalf("expected log level %s, got %s", tc.wantLevel, hook.entries[0].Level)
			}
		})
	}
}
