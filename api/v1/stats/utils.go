package stats

type TaskTypeString string

const (
	ImageTaskType TaskTypeString = "Image"
	TextTaskType  TaskTypeString = "Text"
	VideoTaskType TaskTypeString = "Video"
	AllTaskType   TaskTypeString = "All"
)

type TimeUnit string

const (
	UnitHour  TimeUnit = "Hour"
	UnitDay   TimeUnit = "Day"
	UnitWeek  TimeUnit = "Week"
	UnitMonth TimeUnit = "Month"
)
