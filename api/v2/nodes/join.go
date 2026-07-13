package nodes

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/api/v2/validate"
	"crynux_relay/config"
	"crynux_relay/models"
	"crynux_relay/service"
	"crynux_relay/utils"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NodeJoinInput struct {
	Network  string        `json:"network" description:"network" validate:"required"`
	Address  string        `json:"address" path:"address" description:"address" validate:"required"`
	GPUName  string        `json:"gpu_name" description:"gpu_name" validate:"required"`
	GPUVram  uint64        `json:"gpu_vram" description:"gpu_vram" validate:"required"`
	Version  string        `json:"version" description:"version" validate:"required"`
	ModelIDs []string      `json:"model_ids" description:"node local model ids" validate:"required"`
	Staking  models.BigInt `json:"staking" description:"staking amount" validate:"required"`
}

type NodeJoinInputWithSignature struct {
	NodeJoinInput
	Timestamp int64  `json:"timestamp" description:"Signature timestamp" validate:"required"`
	Signature string `json:"signature" description:"Signature" validate:"required"`
}

func NodeJoin(c *gin.Context, in *NodeJoinInputWithSignature) (*response.Response, error) {
	match, address, err := validate.ValidateSignature(in.NodeJoinInput, in.Timestamp, in.Signature)

	if err != nil || !match {

		if err != nil {
			log.Debugln("error in sig validate: " + err.Error())
		}

		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if address != in.Address {
		validationErr := response.NewValidationErrorResponse("address", "Signer not allowed")
		return nil, validationErr
	}

	in.ModelIDs = models.NormalizeModelIDs(in.ModelIDs)

	unlockJoin := service.LockNodeIndexByAddress(in.Address)
	defer unlockJoin()

	versions := strings.Split(in.Version, ".")
	if len(versions) != 3 {
		return nil, response.NewValidationErrorResponse("version", "Invalid node version")
	}
	nodeVersions := make([]uint64, 3)
	for i := 0; i < 3; i++ {
		if v, err := strconv.ParseUint(versions[i], 10, 64); err != nil {
			return nil, response.NewValidationErrorResponse("task_version", "Invalid task version")
		} else {
			nodeVersions[i] = v
		}
	}

	isNewNode := false
	node, err := models.GetNodeByAddress(c.Request.Context(), config.GetDB(), in.Address)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		isNewNode = true
		node = &models.Node{
			Network:      in.Network,
			Address:      in.Address,
			GPUName:      in.GPUName,
			GPUVram:      in.GPUVram,
			QOSScore:     0,
			MajorVersion: nodeVersions[0],
			MinorVersion: nodeVersions[1],
			PatchVersion: nodeVersions[2],
			Status:       models.NodeStatusQuit,
		}
	} else if err != nil {
		return nil, response.NewExceptionResponse(err)
	} else {
		node.Network = in.Network
		node.GPUName = in.GPUName
		node.GPUVram = in.GPUVram
		node.MajorVersion = nodeVersions[0]
		node.MinorVersion = nodeVersions[1]
		node.PatchVersion = nodeVersions[2]
	}
	if node.Status != models.NodeStatusQuit {
		return nil, response.NewValidationErrorResponse("address", "Node already joined")
	}
	rejoinQosFloorBefore := service.CaptureNodeQosTraceValues(node)
	service.AdjustNodeQosForJoin(node, isNewNode)
	rejoinQosFloorAfter := service.CaptureNodeQosTraceValues(node)

	stakeAmount := &in.Staking.Int
	if stakeAmount.Sign() == 0 {
		appConfig := config.GetConfig()
		stakeAmount = utils.EtherToWei(big.NewInt(int64(appConfig.Task.StakeAmount)))
	}
	node.StakeAmount = models.BigInt{Int: *stakeAmount}

	err = service.SetNodeStatusJoin(c.Request.Context(), config.GetDB(), node, in.ModelIDs)
	if refreshErr := service.RefreshNodeIndexLocked(c.Request.Context(), config.GetDB(), in.Address); refreshErr != nil {
		log.Errorf("NodeJoin: refresh node index for %s error: %v", in.Address, refreshErr)
	}
	if err != nil {
		if errors.Is(err, service.ErrDelegatedSlashJobInProgress) {
			return nil, response.NewValidationErrorResponse("address", "Delegated slash job in progress")
		}
		return nil, response.NewExceptionResponse(err)
	}
	service.RecordNodeQosTrace(service.NodeQosTraceInput{
		NodeAddress: in.Address,
		EventType:   service.QosTraceEventNodeRejoinQosFloor,
		Before:      rejoinQosFloorBefore,
		After:       rejoinQosFloorAfter,
	})
	return &response.Response{}, nil
}
