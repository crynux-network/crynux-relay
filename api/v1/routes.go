package v1

import (
	"crynux_relay/api/v1/client"
	"crynux_relay/api/v1/credits"
	"crynux_relay/api/v1/delegator"
	"crynux_relay/api/v1/event"
	"crynux_relay/api/v1/incentive"
	"crynux_relay/api/v1/inference_tasks"
	"crynux_relay/api/v1/middleware"
	"crynux_relay/api/v1/network"
	"crynux_relay/api/v1/nodes"
	"crynux_relay/api/v1/response"
	"crynux_relay/api/v1/staking"
	"crynux_relay/api/v1/stats"
	taskfee "crynux_relay/api/v1/task_fee"
	taskquota "crynux_relay/api/v1/task_quota"
	"crynux_relay/api/v1/time"
	"crynux_relay/api/v1/withdraw"
	"crynux_relay/api/v1/worker"

	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
)

func InitRoutes(r *fizz.Fizz) {
	v1g := r.Group("v1", "ApiV1", "API version 1")

	v1g.GET("now", []fizz.OperationOption{
		fizz.Summary("Get current unix timestamp of server"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(time.GetNow, 200))

	tasksGroup := v1g.Group("inference_tasks", "Inference tasks", "Inference tasks related APIs")

	tasksGroup.POST("/:task_id_commitment", []fizz.OperationOption{
		fizz.Summary("Create an task"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.CreateTask, 200))

	tasksGroup.GET("/:task_id_commitment", []fizz.OperationOption{
		fizz.Summary("Get a task by task id"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.GetTaskById, 200))

	tasksGroup.POST("/:task_id_commitment/results", []fizz.OperationOption{
		fizz.Summary("Upload task result"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.UploadResult, 200))

	tasksGroup.GET("/:task_id_commitment/results/:index", []fizz.OperationOption{
		fizz.Summary("Get the result of the task by node address"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.GetResult, 200))
	tasksGroup.GET("/:task_id_commitment/results/checkpoint", []fizz.OperationOption{
		fizz.Summary("Get the result checkpoint of the task by node address"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.GetResultCheckpoint, 200))
	tasksGroup.GET("/:task_id_commitment/selected_node", []fizz.OperationOption{
		fizz.Summary("Get selected node info of this task"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.GetSelectedNodeInfo, 200))

	tasksGroup.GET("/:task_id_commitment/checkpoint", []fizz.OperationOption{
		fizz.Summary("Get the input checkpoint of the task"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.GetCheckpoint, 200))

	tasksGroup.POST("/:task_id_commitment/score", []fizz.OperationOption{
		fizz.Summary("Submit task score"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.SubmitScore, 200))
	tasksGroup.POST("/validate", []fizz.OperationOption{
		fizz.Summary("Validate single task or task group"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.ValidateTask, 200))
	tasksGroup.POST("/:task_id_commitment/abort_reason", []fizz.OperationOption{
		fizz.Summary("Abort task, report task abort resaon"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.AbortTask, 200))
	tasksGroup.POST("/:task_id_commitment/task_error", []fizz.OperationOption{
		fizz.Summary("Report task error"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(inference_tasks.ReportTaskError, 200))

	nodeGroup := v1g.Group("node", "node", "Node APIs")
	nodeGroup.GET("/:address", []fizz.OperationOption{
		fizz.Summary("Get node info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetNode, 200))
	nodeGroup.POST("/:address/join", []fizz.OperationOption{
		fizz.Summary("Node join"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodeJoin, 200))
	nodeGroup.POST("/:address/quit", []fizz.OperationOption{
		fizz.Summary("Node quit"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodeQuit, 200))
	nodeGroup.POST("/:address/pause", []fizz.OperationOption{
		fizz.Summary("Node pause"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodePause, 200))
	nodeGroup.POST("/:address/resume", []fizz.OperationOption{
		fizz.Summary("Node resume"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodeResume, 200))
	nodeGroup.POST("/:address/model", []fizz.OperationOption{
		fizz.Summary("Add node's local model id"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.AddModelID, 200))
	nodeGroup.POST("/:address/version", []fizz.OperationOption{
		fizz.Summary("Update node's version"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.UpdateNodeVersion, 200))
	nodeGroup.GET("/:address/task", []fizz.OperationOption{
		fizz.Summary("Get node current task"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetNodeTask, 200))

	balanceGroup := v1g.Group("balance", "balance", "balance related APIs")
	balanceGroup.GET("/:address", []fizz.OperationOption{
		fizz.Summary("Get balance of account"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(taskfee.GetTaskFee, 200))

	stakingGroup := v1g.Group("staking", "staking", "staking related APIs")
	stakingGroup.GET("/:address", []fizz.OperationOption{
		fizz.Summary("Get staking of node"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(staking.GetStaking, 200))

	userStakingGroup := v1g.Group("user_staking", "user_staking", "user staking related APIs")
	userStakingGroup.GET("/node/:address", []fizz.OperationOption{
		fizz.Summary("Get user staking of node"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(staking.GetUserStakingOfNode, 200))

	eventsGroup := v1g.Group("events", "events", "events related APIs")
	eventsGroup.GET("", []fizz.OperationOption{
		fizz.Summary("Get events"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(event.GetEvents, 200))
	eventsGroup.GET("/current_id", []fizz.OperationOption{
		fizz.Summary("Get current event id"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(event.GetCurrentEventID, 200))

	networkGroup := v1g.Group("network", "network", "Network stats related APIs")

	networkGroup.GET("/nodes/data", []fizz.OperationOption{
		fizz.Summary("Get the info of all the nodes in the network"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(network.GetAllNodeData, 200))

	networkGroup.GET("/nodes/number", []fizz.OperationOption{
		fizz.Summary("Get total nodes number in the network"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(network.GetAllNodeNumber, 200))

	networkGroup.GET("/tasks/number", []fizz.OperationOption{
		fizz.Summary("Get total task number in the network"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(network.GetAllTaskNumber, 200))

	networkGroup.GET("", []fizz.OperationOption{
		fizz.Summary("Get total TFLOPS of the network"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(network.GetNetworkTFLOPS, 200))

	workerGroup := v1g.Group("worker", "worker", "Worker count related APIs")

	workerGroup.POST("/:version", []fizz.OperationOption{
		fizz.Summary("Called when a worker is up"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(worker.WorkerJoin, 200))

	workerGroup.DELETE("/:version", []fizz.OperationOption{
		fizz.Summary("Called when a worker is down"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(worker.WorkerQuit, 200))

	workerGroup.GET("/:version/count", []fizz.OperationOption{
		fizz.Summary("Get worker count of specified version"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(worker.GetWorkerCount, 200))

	statsGroup := v1g.Group("stats", "stats", "task statistics related APIs")

	statsGroup.GET("/line_chart/task_count", []fizz.OperationOption{
		fizz.Summary("Get line chart data of task count"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskCountLineChart, 200))

	statsGroup.GET("/line_chart/task_success_rate", []fizz.OperationOption{
		fizz.Summary("Get line chart data of task success rate"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskSuccessRateLineChart, 200))

	statsGroup.GET("/line_chart/node/:address/earnings", []fizz.OperationOption{
		fizz.Summary("Get line chart data of node's earnings"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetNodeEarningsLineChart, 200))
	statsGroup.GET("/line_chart/node/:address/staking", []fizz.OperationOption{
		fizz.Summary("Get line chart data of node's staking"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetNodeStakingsLineChart, 200))
	statsGroup.GET("/line_chart/node/:address/scores", []fizz.OperationOption{
		fizz.Summary("Get line chart data of node's scores"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetNodeScoresLineChart, 200))
	statsGroup.GET("/line_chart/node/:address/delegator_num", []fizz.OperationOption{
		fizz.Summary("Get line chart data of node's delegator number"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetNodeDelegatorNumLineChart, 200))
	statsGroup.GET("/line_chart/delegator/:address/earnings", []fizz.OperationOption{
		fizz.Summary("Get line chart data of a delegator's earnings"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetDelegatorEarningsLineChart, 200))
	statsGroup.GET("/line_chart/delegation/:user_address/:node_address/earnings", []fizz.OperationOption{
		fizz.Summary("Get line chart data of a delegation's earnings"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetDelegationEarningsLineChart, 200))

	statsGroup.GET("/histogram/task_execution_time", []fizz.OperationOption{
		fizz.Summary("Get histogram data of task execution time"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskExecutionTimeHistogram, 200))
	statsGroup.GET("/histogram/task_upload_result_time", []fizz.OperationOption{
		fizz.Summary("Get histogram data of task upload result time"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskUploadResultTimeHistogram, 200))
	statsGroup.GET("/histogram/task_waiting_time", []fizz.OperationOption{
		fizz.Summary("Get histogram data of task waiting time"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskWaitingTimeHistogram, 200))

	statsGroup.GET("/line_chart/incentive", []fizz.OperationOption{
		fizz.Summary("Get line chart data of incentives"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetIncentiveLineChart, 200))
	statsGroup.GET("/histogram/task_fee", []fizz.OperationOption{
		fizz.Summary("Get histogram data of task fee in the path hour"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetTaskFeeHistogram, 200))

	statsGroup.GET("/node_events", []fizz.OperationOption{
		fizz.Summary("Get node event logs in the recent hour"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetNodeEventLogs, 200))
	statsGroup.GET("/queue/count", []fizz.OperationOption{
		fizz.Summary("Get queued tasks count"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(stats.GetQueuedTasksCount, 200))

	incentiveGroup := v1g.Group("incentive", "incentive", "incentive statistics related APIs")

	incentiveGroup.GET("/total", []fizz.OperationOption{
		fizz.Summary("Get today's total incentive"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(incentive.GetTotalIncentive, 200))

	incentiveGroup.GET("/nodes", []fizz.OperationOption{
		fizz.Summary("Get nodes with top K incentive"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(incentive.GetNodeIncentiveRank, 200))
	incentiveGroup.GET("/node/:address", []fizz.OperationOption{
		fizz.Summary("Get node daily incentive"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(incentive.GetNodeDailyIncentive, 200))

	taskFeeGroup := v1g.Group("task_fee", "task_fee", "task fee related APIs")
	taskFeeGroup.GET("/logs", []fizz.OperationOption{
		fizz.Summary("Get task fee logs"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(taskfee.GetTaskFeeLogs, 200))

	taskQuotaGroup := v1g.Group("task_quota", "task_quota", "task quota related APIs")
	taskQuotaGroup.GET("/:address", []fizz.OperationOption{
		fizz.Summary("Get task quota"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(taskquota.GetTaskQuota, 200))

	withdrawGroup := v1g.Group("withdraw", "withdraw", "withdraw related APIs")
	withdrawGroup.GET("/list", []fizz.OperationOption{
		fizz.Summary("Get withdraw requests"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(withdraw.GetWithdrawRequests, 200))
	withdrawGroup.POST("/:id/fulfill", []fizz.OperationOption{
		fizz.Summary("Fulfill withdraw request"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(withdraw.FulfillWithdrawRequest, 200))
	withdrawGroup.POST("/:id/reject", []fizz.OperationOption{
		fizz.Summary("Reject withdraw request"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(withdraw.RejectWithdrawRequest, 200))

	creditsGroup := v1g.Group("credits", "credits", "credits related APIs")
	creditsGroup.POST("/:address", []fizz.OperationOption{
		fizz.Summary("Create credits"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(credits.CreateCredits, 200))
	creditsGroup.GET("/:id", []fizz.OperationOption{
		fizz.Summary("Get credits record"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(credits.GetCreditsRecord, 200))

	clientGroup := v1g.Group("client", "client", "client related APIs")
	clientGroup.POST("/connect_wallet", []fizz.OperationOption{
		fizz.Summary("Connect wallet"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(client.ConnectWallet, 200))
	clientGroup.POST("/:address/withdraw", []fizz.OperationOption{
		fizz.Summary("Create withdraw request"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(client.CreateWithdrawRequest, 200))
	clientGroup.GET("/:address/withdraw/list", []fizz.OperationOption{
		fizz.Summary("Get withdraw records"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(client.GetWithdrawRecords, 200))
	clientGroup.GET("/:address/deposit/list", []fizz.OperationOption{
		fizz.Summary("Get deposit records"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(client.GetDepositRecords, 200))

	delegatorGroup := v1g.Group("delegator", "delegator", "delegator related APIs")
	delegatorGroup.GET("/:user_address/delegation", []fizz.OperationOption{
		fizz.Summary("Get delegation info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(delegator.GetDelegation, 200))
	delegatorGroup.GET("/:user_address", []fizz.OperationOption{
		fizz.Summary("Get delegator info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(delegator.GetDelegatorInfo, 200))
	delegatorGroup.GET("/:user_address/delegations", []fizz.OperationOption{
		fizz.ID("get_user_delegations"),
		fizz.Summary("Get all delegations of the user"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(delegator.GetDelegations, 200))
}
