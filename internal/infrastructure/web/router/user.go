package router

import (
	"github.com/gin-gonic/gin"

	"github.com/jrmarcello/go-boilerplate/internal/infrastructure/web/handler"
)

// RegisterUserRoutes registra todas as rotas relacionadas a User
func RegisterUserRoutes(rg *gin.RouterGroup, h *handler.UserHandler) {
	rg.POST("/users", h.Create)
	rg.GET("/users", h.List)
	rg.GET("/users/:id", h.GetByID)
	rg.PUT("/users/:id", h.Update)
	rg.DELETE("/users/:id", h.Delete)
}
