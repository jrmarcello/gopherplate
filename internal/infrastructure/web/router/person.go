package router

import (
	"github.com/gin-gonic/gin"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/web/handler"
)

// RegisterPersonRoutes registra todas as rotas relacionadas a Person
func RegisterPersonRoutes(r *gin.Engine, h *handler.PersonHandler) {
	r.POST("/people", h.Create)
	r.GET("/people/:id", h.GetByID)
	r.PUT("/people/:id", h.Update)
	r.DELETE("/people/:id", h.Delete)
}
