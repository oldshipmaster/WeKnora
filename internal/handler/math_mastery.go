package handler

import (
	"net/http"

	appErrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

type MathMasteryHandler struct {
	service interfaces.MathMasteryService
}

func NewMathMasteryHandler(service interfaces.MathMasteryService) *MathMasteryHandler {
	return &MathMasteryHandler{service: service}
}

func (h *MathMasteryHandler) GetTree(c *gin.Context) {
	tree, err := h.service.GetTree(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tree})
}

type seedMathCurriculumRequest struct {
	Nodes []types.MathCurriculumNode `json:"nodes" binding:"required"`
	Edges []types.MathCurriculumEdge `json:"edges"`
}

func (h *MathMasteryHandler) SeedCurriculum(c *gin.Context) {
	var request seedMathCurriculumRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewBadRequestError("课程树数据不合法").WithDetails(err.Error()))
		return
	}
	if err := h.service.SeedCurriculum(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), request.Nodes, request.Edges); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type upsertMathSourcesRequest struct {
	Sources []types.MathSourceBinding `json:"sources" binding:"required"`
}

func (h *MathMasteryHandler) UpsertSources(c *gin.Context) {
	var request upsertMathSourcesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewBadRequestError("素材清单数据不合法").WithDetails(err.Error()))
		return
	}
	if err := h.service.UpsertSources(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), request.Sources); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type startMathAttemptRequest struct {
	ProfileID string     `json:"profile_id" binding:"required"`
	Scope     types.JSON `json:"scope"`
}

func (h *MathMasteryHandler) StartAttempt(c *gin.Context) {
	var request startMathAttemptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewBadRequestError("诊断范围不合法").WithDetails(err.Error()))
		return
	}
	attempt, err := h.service.StartAttempt(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), request.ProfileID, request.Scope)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": attempt})
}

func (h *MathMasteryHandler) SubmitResponse(c *gin.Context) {
	var response types.MathDiagnosticResponse
	if err := c.ShouldBindJSON(&response); err != nil {
		c.Error(appErrors.NewBadRequestError("诊断作答不合法").WithDetails(err.Error()))
		return
	}
	assessment, err := h.service.SubmitResponse(c.Request.Context(), secutils.SanitizeForLog(c.Param("id")), response)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": assessment})
}
