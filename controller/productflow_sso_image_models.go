package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetProductFlowSSOImageModels(c *gin.Context) {
	group := strings.TrimSpace(c.Query("group"))
	imageModels, err := model.GetGroupEnabledImageModels(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	textModels, err := model.GetGroupEnabledTextModels(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"group":        group,
		"models":       imageModels,
		"image_models": imageModels,
		"text_models":  textModels,
	})
}
