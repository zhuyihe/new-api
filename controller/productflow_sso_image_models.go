package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetProductFlowSSOImageModels(c *gin.Context) {
	group := strings.TrimSpace(c.Query("group"))
	models, err := model.GetGroupEnabledImageModels(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"group":  group,
		"models": models,
	})
}
