package cnx

import (
	"net/http"
	"time"

	"crynux_relay/config"
	"crynux_relay/service"

	"github.com/gin-gonic/gin"
	"github.com/wI2L/fizz"
)

func InitRoutes(r *fizz.Fizz) {
	cnxGroup := r.Group("cnx", "cnx", "CNX token APIs")

	cnxGroup.GET("/total-supply", []fizz.OperationOption{
		fizz.ID("cnx_total_supply"),
		fizz.Summary("Get total CNX supply"),
	}, GetTotalSupply)

	cnxGroup.GET("/circulating-supply", []fizz.OperationOption{
		fizz.ID("cnx_circulating_supply"),
		fizz.Summary("Get circulating CNX supply"),
	}, GetCirculatingSupply)
}

func GetTotalSupply(c *gin.Context) {
	c.String(http.StatusOK, service.GetCNXTotalSupply().String())
}

func GetCirculatingSupply(c *gin.Context) {
	supply, err := service.GetCNXCirculatingSupply(time.Now().UTC(), config.GetConfig().Dao.MainnetStartTime)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.String(http.StatusOK, supply.String())
}
