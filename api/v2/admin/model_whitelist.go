package admin

import (
	"crynux_relay/api/v2/response"
	"crynux_relay/config"
	"crynux_relay/service"
	"errors"

	"github.com/gin-gonic/gin"
)

type AddModelWhitelistInput struct {
	ModelName string `json:"model_name" validate:"required"`
}

type DeleteModelWhitelistInput struct {
	ModelName string `query:"model_name" validate:"required"`
}

type ListModelWhitelistData struct {
	ModelNames []string `json:"model_names"`
}

type ListModelWhitelistResponse struct {
	response.Response
	Data ListModelWhitelistData `json:"data"`
}

func ListModelWhitelist(c *gin.Context) (*ListModelWhitelistResponse, error) {
	names, err := service.ListModelWhitelistNames(c.Request.Context(), config.GetDB())
	if err != nil {
		return nil, response.NewExceptionResponse(err)
	}
	return &ListModelWhitelistResponse{
		Data: ListModelWhitelistData{
			ModelNames: names,
		},
	}, nil
}

func AddModelWhitelist(c *gin.Context, in *AddModelWhitelistInput) (*response.Response, error) {
	err := service.AddModelWhitelistName(c.Request.Context(), config.GetDB(), in.ModelName)
	if err != nil {
		if errors.Is(err, service.ErrInvalidModelWhitelistName) ||
			errors.Is(err, service.ErrModelWhitelistNameExists) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}

func DeleteModelWhitelist(c *gin.Context, in *DeleteModelWhitelistInput) (*response.Response, error) {
	err := service.DeleteModelWhitelistName(c.Request.Context(), config.GetDB(), in.ModelName)
	if err != nil {
		if errors.Is(err, service.ErrInvalidModelWhitelistName) ||
			errors.Is(err, service.ErrModelWhitelistNameMissing) {
			return nil, &response.ErrorResponse{
				Response: response.Response{Message: err.Error()},
			}
		}
		return nil, response.NewExceptionResponse(err)
	}
	return &response.Response{}, nil
}
