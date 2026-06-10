package v2

import (
	"crynux_relay/api/v2/admin"
	"crynux_relay/api/v2/incentive"
	"crynux_relay/api/v2/middleware"
	"crynux_relay/api/v2/network"
	"crynux_relay/api/v2/nodes"
	relayaccount "crynux_relay/api/v2/relay_account"
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

	delegatedStakingGroup := v2g.Group("delegated_staking", "delegated_staking", "Delegated staking APIs")
	delegatedStakingGroup.GET("/nodes", []fizz.OperationOption{
		fizz.ID("get_delegated_nodes_v2"),
		fizz.Summary("Get delegated nodes"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetDelegatedNodes, 200))
	delegatedStakingGroup.GET("/nodes/:address", []fizz.OperationOption{
		fizz.ID("get_delegated_node_v2"),
		fizz.Summary("Get delegated node info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("404", "Not found", response.NotFoundErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetDelegatedNode, 200))
	delegatedStakingGroup.GET("/nodes/:address/delegations", []fizz.OperationOption{
		fizz.ID("get_delegated_node_delegations_v2"),
		fizz.Summary("Get delegations of the node"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("404", "Not found", response.NotFoundErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetDelegations, 200))
	delegatedStakingGroup.GET("/nodes/:address/emission/chart", []fizz.OperationOption{
		fizz.ID("get_delegated_node_emission_chart_v2"),
		fizz.Summary("Get weekly emission chart for delegated node"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("403", "access denied", response.ErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetNodeEmissionChart, 200))

	relayAccountGroup := v2g.Group("relay_account", "relay_account", "relay account related APIs")
	relayAccountGroup.GET("/:address/vesting/locked", []fizz.OperationOption{
		fizz.ID("relay_account_vesting_locked_v2"),
		fizz.Summary("Get locked vesting amount"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(relayaccount.GetLockedVesting, 200))
	relayAccountGroup.GET("/:address/vesting/list", []fizz.OperationOption{
		fizz.ID("relay_account_vesting_list_v2"),
		fizz.Summary("Get vesting records"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(relayaccount.GetVestingRecords, 200))
	relayAccountGroup.GET("/:address/emission/chart", []fizz.OperationOption{
		fizz.ID("relay_account_emission_chart_v2"),
		fizz.Summary("Get weekly emission chart for relay account"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
	}, middleware.JWTAuthMiddleware(), tonic.Handler(relayaccount.GetEmissionChart, 200))

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
	adminGroup.GET("/emission/task_fee_csv", []fizz.OperationOption{
		fizz.ID("admin_emission_task_fee_csv_v2"),
		fizz.Summary("Export previous emission week task fee and emission in CSV"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), admin.ExportEmissionTaskFeeCSV)
	adminGroup.POST("/vesting", []fizz.OperationOption{
		fizz.ID("admin_vesting_create_v2"),
		fizz.Summary("Create vesting records"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.CreateVestingRecords, 200))
	adminGroup.POST("/vesting/restore", []fizz.OperationOption{
		fizz.ID("admin_vesting_restore_v2"),
		fizz.Summary("Restore slashed node vesting records"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.RestoreNodeVestings, 200))
	adminGroup.GET("/delegated_slash/audits", []fizz.OperationOption{
		fizz.ID("admin_delegated_slash_audits_v2"),
		fizz.Summary("List delegated slash audit records"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.ListDelegatedSlashAudits, 200))
	adminGroup.POST("/nodes/slash", []fizz.OperationOption{
		fizz.ID("admin_node_slash_v2"),
		fizz.Summary("Queue a node slash transaction"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.TriggerNodeSlash, 200))
	adminGroup.GET("/task_whitelist", []fizz.OperationOption{
		fizz.ID("admin_task_whitelist_list_v2"),
		fizz.Summary("List task creator whitelist"),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.ListTaskWhitelist, 200))
	adminGroup.POST("/task_whitelist", []fizz.OperationOption{
		fizz.ID("admin_task_whitelist_add_v2"),
		fizz.Summary("Add an address to task creator whitelist"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.AddTaskWhitelist, 200))
	adminGroup.DELETE("/task_whitelist/:address", []fizz.OperationOption{
		fizz.ID("admin_task_whitelist_delete_v2"),
		fizz.Summary("Delete an address from task creator whitelist"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.DeleteTaskWhitelist, 200))
	adminGroup.GET("/node_name_whitelist", []fizz.OperationOption{
		fizz.ID("admin_node_name_whitelist_list_v2"),
		fizz.Summary("List node name whitelist"),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.ListNodeNameWhitelist, 200))
	adminGroup.POST("/node_name_whitelist", []fizz.OperationOption{
		fizz.ID("admin_node_name_whitelist_add_v2"),
		fizz.Summary("Add an entry to node name whitelist"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.AddNodeNameWhitelist, 200))
	adminGroup.DELETE("/node_name_whitelist/:gpu_name/:gpu_vram/:node_version", []fizz.OperationOption{
		fizz.ID("admin_node_name_whitelist_delete_v2"),
		fizz.Summary("Delete an entry from node name whitelist"),
		fizz.Response("400", "validation errors", response.ErrorResponse{}, nil, nil),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), tonic.Handler(admin.DeleteNodeNameWhitelist, 200))
	adminGroup.GET("/node_names_csv", []fizz.OperationOption{
		fizz.ID("admin_node_names_csv_v2"),
		fizz.Summary("Export node names in CSV"),
		fizz.Response("401", "unauthorized", response.ErrorResponse{}, nil, nil),
		fizz.Response("500", "exception", response.ExceptionResponse{}, nil, nil),
	}, middleware.AdminAuthMiddleware(), admin.ExportNodeNamesCSV)

}
