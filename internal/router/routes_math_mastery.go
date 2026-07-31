package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterMathMasteryRoutes(r *gin.RouterGroup, mathHandler *handler.MathMasteryHandler, g *rbacGuards) {
	if mathHandler == nil {
		return
	}
	mastery := g.apiKeyGroup(r.Group("/knowledge-bases/:id/math-mastery"), apiKeyIngest(apiKeyFullAccess()))
	masteryRead := mastery.With(apiKeyRetrieve(apiKeyFullAccess()))

	masteryRead.GET("/tree", g.Viewer(), g.KBAccessRead("id"), mathHandler.GetTree)
	masteryRead.GET("/questions", g.Viewer(), g.KBAccessRead("id"), mathHandler.ListQuestions)
	mastery.POST("/seed", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), mathHandler.SeedCurriculum)
	mastery.PUT("/sources", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), mathHandler.UpsertSources)
	mastery.PUT("/questions", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), mathHandler.UpsertQuestions)
	mastery.POST("/attempts", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), mathHandler.StartAttempt)
	mastery.POST("/responses", g.OwnedKBOrAdmin(), g.KBAccessWrite("id"), mathHandler.SubmitResponse)
}
