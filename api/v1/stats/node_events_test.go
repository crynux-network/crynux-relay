package stats

import (
	"crynux_relay/models"
	"database/sql"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestNodeReleaseTime(t *testing.T) {
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	validated := now.Add(-2 * time.Minute)
	uploaded := now.Add(-1 * time.Minute)
	updated := now

	tests := []struct {
		name string
		task models.InferenceTask
		want time.Time
	}{
		{
			name: "aborted without validated time uses updated at",
			task: models.InferenceTask{
				Model:  gorm.Model{UpdatedAt: updated},
				Status: models.TaskEndAborted,
			},
			want: updated,
		},
		{
			name: "aborted with validated time still uses updated at",
			task: models.InferenceTask{
				Model:         gorm.Model{UpdatedAt: updated},
				Status:        models.TaskEndAborted,
				ValidatedTime: sql.NullTime{Time: validated, Valid: true},
			},
			want: updated,
		},
		{
			name: "group refund uses validated time",
			task: models.InferenceTask{
				Model:         gorm.Model{UpdatedAt: updated},
				Status:        models.TaskEndGroupRefund,
				ValidatedTime: sql.NullTime{Time: validated, Valid: true},
			},
			want: validated,
		},
		{
			name: "invalidated without validated time uses updated at",
			task: models.InferenceTask{
				Model:  gorm.Model{UpdatedAt: updated},
				Status: models.TaskEndInvalidated,
			},
			want: updated,
		},
		{
			name: "success uses result uploaded time",
			task: models.InferenceTask{
				Model:              gorm.Model{UpdatedAt: updated},
				Status:             models.TaskEndSuccess,
				ResultUploadedTime: sql.NullTime{Time: uploaded, Valid: true},
			},
			want: uploaded,
		},
		{
			name: "group success uses result uploaded time",
			task: models.InferenceTask{
				Model:              gorm.Model{UpdatedAt: updated},
				Status:             models.TaskEndGroupSuccess,
				ResultUploadedTime: sql.NullTime{Time: uploaded, Valid: true},
			},
			want: uploaded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := nodeReleaseTime(test.task)
			if !got.Equal(test.want) {
				t.Fatalf("nodeReleaseTime() = %s, want %s", got, test.want)
			}
		})
	}
}
