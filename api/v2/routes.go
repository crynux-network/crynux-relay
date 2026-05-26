package v2

import (
	"crynux_relay/api/v2/admin"
	"crynux_relay/api/v2/incentive"
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/network"
	"crynux_relay/api/v2/nodes"
	"crynux_relay/api/v2/response"

	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
)

func InitRoutes(r *fizz.Fizz) {

	v2g := r.Group("v2", "ApiV2", "API version 2")

	incentiveGroup := v2g.Group("incentive", "incentive", "incentive statistics related APIs")

	incentiveGroup.GET("/nodes", []fizz.OperationOption{
		fizz.ID("incentive_nodes_v2"),
		fizz.Summary("Get nodes with top K incentive"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(incentive.GetNodeIncentive, 200))
	incentiveGroup.GET("/nodes/all", []fizz.OperationOption{
		fizz.ID("incentive_nodes_all_v2"),
		fizz.Summary("Get all nodes with incentive"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(incentive.GetAllNodeIncentive, 200))

	networkGroup := v2g.Group("network", "network", "Network stats related APIs")

	networkGroup.GET("/nodes/data", []fizz.OperationOption{
		fizz.ID("network_nodes_data_v2"),
		fizz.Summary("Get the info of all the nodes in the network"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(network.GetAllNodeData, 200))

	nodeGroup := v2g.Group("node", "node", "Node APIs")

	nodeGroup.GET("/:address", []fizz.OperationOption{
		fizz.ID("node_get_v2"),
		fizz.Summary("Get node info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetNode, 200))
	nodeGroup.POST("/:address/join", []fizz.OperationOption{
		fizz.ID("node_join_v2"),
		fizz.Summary("Node join"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodeJoin, 200))

	adminGroup := v2g.Group("admin", "admin", "Admin APIs")
	adminNodesGroup := adminGroup.Group("nodes", "admin nodes", "Admin node management APIs")
	adminNodesGroup.GET("/qos", []fizz.OperationOption{
		fizz.ID("admin_nodes_qos_v2"),
		fizz.Summary("Export active node QoS statistics in CSV"),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), admin.ExportNodeQosCSV)
	adminNodesGroup.GET("/tasks/history", []fizz.OperationOption{
		fizz.ID("admin_nodes_task_history_v2"),
		fizz.Summary("Render node task history in HTML"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), admin.ExportNodeTaskHistoryHTML)
	adminGroup.GET("/nodes_token_csv", []fizz.OperationOption{
		fizz.ID("admin_nodes_token_csv_v2"),
		fizz.Summary("Start exporting node token balances in CSV"),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), admin.ExportNodesTokenCSV)
	adminGroup.POST("/vesting", []fizz.OperationOption{
		fizz.ID("admin_vesting_create_v2"),
		fizz.Summary("Create vesting records"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.CreateVestingRecords, 200))

}
