package router

import (
	"github.com/gin-gonic/gin"

	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/handler"
)

// RegisterRoleRoutes registra todas as rotas relacionadas a Role
func RegisterRoleRoutes(rg *gin.RouterGroup, h *handler.RoleHandler) {
	rg.POST("/roles", h.Create)
	rg.GET("/roles", h.List)
	rg.DELETE("/roles/:id", h.Delete)
}
