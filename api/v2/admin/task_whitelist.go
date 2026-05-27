package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
)

type AddTaskWhitelistInput struct {
	Address string `json:"address" validate:"required"`
}

type DeleteTaskWhitelistInput struct {
	Address string `path:"address" validate:"required"`
}

type ListTaskWhitelistData struct {
	Addresses []string `json:"addresses"`
}

type ListTaskWhitelistResponse struct {
	response.Response
	Data ListTaskWhitelistData `json:"data"`
}

func ListTaskWhitelist(c *gin.Context) (*ListTaskWhitelistResponse, error) {
	addresses, err := service.ListTaskWhitelistAddresses(c.Request.Context(), config.GetDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListTaskWhitelistResponse{
		Data: ListTaskWhitelistData{
			Addresses: addresses,
		},
	}, nil
}

func AddTaskWhitelist(c *gin.Context, in *AddTaskWhitelistInput) (*response.Response, error) {
	err := service.AddTaskWhitelistAddress(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaskWhitelistAddress) ||
			errors.Is(err, service.ErrTaskWhitelistAddressExists) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}

func DeleteTaskWhitelist(c *gin.Context, in *DeleteTaskWhitelistInput) (*response.Response, error) {
	err := service.DeleteTaskWhitelistAddress(c.Request.Context(), config.GetDB(), in.Address)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTaskWhitelistAddress) ||
			errors.Is(err, service.ErrTaskWhitelistAddressMissing) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
