package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
)

type NodeQosTracingData struct {
	NodeAddress   string                      `json:"node_address"`
	MaxTaskEvents uint64                      `json:"max_task_events"`
	Events        []service.NodeQosTraceEvent `json:"events"`
}

type NodeQosTracingResponse struct {
	response.Response
	Data NodeQosTracingData `json:"data"`
}

func GetNodeQosTracing(c *gin.Context, input *GetNodeInputWithSignature) (*NodeQosTracingResponse, error) {
	if err := authorizeGetNode(c, input); err != nil {
		return nil, err
	}

	return &NodeQosTracingResponse{
		Data: NodeQosTracingData{
			NodeAddress:   input.Address,
			MaxTaskEvents: service.GetNodeQosTraceMaxTaskEvents(),
			Events:        service.ListNodeQosTraceEvents(input.Address),
		},
	}, nil
}
