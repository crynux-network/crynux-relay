package v2

import (
	"crynux_relay/api/v2/incentive"
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
	nodeGroup.GET("/public/:address", []fizz.OperationOption{
		fizz.ID("node_public_get_v2"),
		fizz.Summary("Get public node info"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
		fizz.Response("404", "Not found", response.NotFoundErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetPublicNode, 200))
	nodeGroup.POST("/:address/join", []fizz.OperationOption{
		fizz.ID("node_join_v2"),
		fizz.Summary("Node join"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.NodeJoin, 200))
	nodeGroup.GET("/:address/delegations", []fizz.OperationOption{
		fizz.ID("get_node_delegations"),
		fizz.Summary("Get delegations of the node"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetDelegations, 200))

	nodesGroup := v2g.Group("nodes", "nodes", "Nodes APIs")
	nodesGroup.GET("/delegated", []fizz.OperationOption{
		fizz.ID("delegated_nodes_v2"),
		fizz.Summary("Get delegated nodes"),
		fizz.Response("400", "validation errors", response.ValidationErrorResponse{}, nil, nil),
	}, tonic.Handler(nodes.GetDelegatedNodes, 200))
}
