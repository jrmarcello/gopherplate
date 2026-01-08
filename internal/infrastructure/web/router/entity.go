package router

import (
	"github.com/gin-gonic/gin"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/web/handler"
)

// RegisterEntityRoutes registra todas as rotas relacionadas a Entity
func RegisterEntityRoutes(r *gin.Engine, h *handler.EntityHandler) {
	r.POST("/entities", h.Create)
	r.GET("/entities", h.List)
	r.GET("/entities/:id", h.GetByID)
	r.PUT("/entities/:id", h.Update)
	r.DELETE("/entities/:id", h.Delete)
}
