package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	roleuc "github.com/jrmarcello/gopherplate/internal/usecases/role"
	"github.com/jrmarcello/gopherplate/internal/usecases/role/dto"
	"github.com/jrmarcello/gopherplate/pkg/httputil/httpgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// RoleHandler agrupa todos os handlers relacionados a Role.
// Segue o padrão de injeção de dependência (UseCases injetados via struct).
type RoleHandler struct {
	CreateUC *roleuc.CreateUseCase
	ListUC   *roleuc.ListUseCase
	DeleteUC *roleuc.DeleteUseCase
}

// NewRoleHandler cria um novo RoleHandler com todos os use cases.
func NewRoleHandler(
	createUC *roleuc.CreateUseCase,
	listUC *roleuc.ListUseCase,
	deleteUC *roleuc.DeleteUseCase,
) *RoleHandler {
	return &RoleHandler{
		CreateUC: createUC,
		ListUC:   listUC,
		DeleteUC: deleteUC,
	}
}

// Create godoc
// @Summary      Create a new role
// @Description  Create a new role with the input payload
// @Tags         roles
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateInput true "Role info"
// @Success      201  {object}  dto.CreateOutput
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /roles [post]
func (h *RoleHandler) Create(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "RoleHandler.Create")
	defer span.End()

	var req dto.CreateInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		httpgin.SendError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	span.SetAttributes(
		attribute.String("role.name", req.Name),
	)

	res, execErr := h.CreateUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, execErr)
		return
	}

	span.SetAttributes(attribute.String("role.id", res.ID))

	httpgin.SendSuccess(c, http.StatusCreated, res)
}

// List godoc
// @Summary      List roles
// @Description  Get a paginated list of roles
// @Tags         roles
// @Produce      json
// @Param        page   query     int     false  "Page number"
// @Param        limit  query     int     false  "Items per page"
// @Param        name   query     string  false  "Filter by name"
// @Success      200    {object}  dto.ListOutput
// @Failure      400   {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /roles [get]
func (h *RoleHandler) List(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "RoleHandler.List")
	defer span.End()

	var req dto.ListInput
	if bindErr := c.ShouldBindQuery(&req); bindErr != nil {
		httpgin.SendError(c, http.StatusBadRequest, "invalid query parameters")
		return
	}

	span.SetAttributes(
		attribute.Int("filter.page", req.Page),
		attribute.Int("filter.limit", req.Limit),
	)

	res, execErr := h.ListUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, execErr)
		return
	}

	span.SetAttributes(attribute.Int("result.total", res.Pagination.Total))
	httpgin.SendSuccessWithMeta(c, http.StatusOK, res.Data, res.Pagination, nil)
}

// Delete godoc
// @Summary      Delete a role
// @Description  Delete a role by ID
// @Tags         roles
// @Produce      json
// @Param        id   path      string  true  "Role ID"
// @Success      200  {object}  dto.DeleteOutput
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /roles/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "RoleHandler.Delete")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("role.id", id))

	res, execErr := h.DeleteUC.Execute(ctx, dto.DeleteInput{ID: id})
	if execErr != nil {
		HandleError(c, execErr)
		return
	}

	httpgin.SendSuccess(c, http.StatusOK, res)
}
