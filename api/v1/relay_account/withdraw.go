package relayaccount

import (
	"crynux_relay/api/tools"
	"crynux_relay/api/v1/middleware"
	"crynux_relay/api/v1/response"
	"crynux_relay/blockchain"
	"crynux_relay/config"
	"crynux_relay/service"
	"crynux_relay/utils"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"

	log "github.com/sirupsen/logrus"
)

type CreateWithdrawInput struct {
	Address        string `path:"address" json:"address" form:"address" validate:"required" description:"The address of the user"`
	Amount         string `json:"amount" form:"amount" validate:"required" description:"The amount of the withdraw"`
	BenefitAddress string `json:"benefit_address" form:"benefit_address" validate:"required" description:"The address of the benefit"`
	Network        string `json:"network" form:"network" validate:"required" description:"The network of the withdraw"`
	Timestamp      int64  `json:"timestamp" form:"timestamp" validate:"required" description:"The timestamp of the withdraw"`
	Signature      string `json:"signature" form:"signature" validate:"required" description:"The signature of the withdraw"`
}

type CreateWithdrawData struct {
	RequestId uint `json:"request_id"`
}

type CreateWithdrawResponse struct {
	response.Response
	Data *CreateWithdrawData `json:"data" description:"The data of the withdraw"`
}

func CreateWithdrawRequest(c *gin.Context, in *CreateWithdrawInput) (*CreateWithdrawResponse, error) {
	userAddress := middleware.GetUserAddress(c)
	if userAddress != in.Address {
		validationErr := response.NewValidationErrorResponse("address", "Address mismatch")
		return nil, validationErr
	}
	amount, _ := big.NewInt(0).SetString(in.Amount, 10)
	message := tools.GenerateWithdrawMessage(in.Address, amount, in.BenefitAddress, in.Network, in.Timestamp)

	signerAddress, err := tools.ValidateAndRecover(message, in.Signature, in.Timestamp)
	if err != nil {
		log.Debugf("Error validating signature: %v", err)
		validationErr := response.NewValidationErrorResponse("signature", "Invalid signature")
		return nil, validationErr
	}

	if signerAddress != in.Address {
		validationErr := response.NewValidationErrorResponse("address", "Signature address mismatch")
		return nil, validationErr
	}

	appConfig := config.GetConfig()
	networkConfig, ok := appConfig.GetEffectiveFundingNetwork(in.Network)
	if !ok {
		validationErr := response.NewValidationErrorResponse("network", "Unsupported withdraw network")
		return nil, validationErr
	}
	minWithdrawalAmount := utils.EtherToWei(big.NewInt(0).SetUint64(networkConfig.WithdrawalMin))
	if amount.Cmp(minWithdrawalAmount) < 0 {
		validationErr := response.NewValidationErrorResponse("amount", "Amount is too small")
		return nil, validationErr
	}
	expectedBenefitAddress, err := blockchain.GetBenefitAddress(c.Request.Context(), common.HexToAddress(in.Address), in.Network)
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	if !common.IsHexAddress(in.BenefitAddress) || !strings.EqualFold(expectedBenefitAddress.Hex(), in.BenefitAddress) {
		validationErr := response.NewValidationErrorResponse("benefit_address", "Benefit address mismatch")
		return nil, validationErr
	}

	record, err := service.Withdraw(c.Request.Context(), config.GetDB(), in.Address, in.BenefitAddress, amount, in.Network)
	if err != nil {
		log.Errorf("Error creating withdraw record: %v", err)
		if errors.Is(err, service.ErrInsufficientRelayAccount) {
			validationErr := response.NewValidationErrorResponse("amount", "Insufficient relay account balance")
			return nil, validationErr
		}
		if errors.Is(err, service.ErrWithdrawDailyLimitExceeded) {
			validationErr := response.NewValidationErrorResponse("amount", "Daily withdrawal limit exceeded")
			return nil, validationErr
		}
		return nil, response.NewExceptionResponse(err)
	}

	return &CreateWithdrawResponse{
		Data: &CreateWithdrawData{
			RequestId: record.ID,
		},
	}, nil

}
