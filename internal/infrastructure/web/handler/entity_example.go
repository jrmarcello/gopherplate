package handler

import (
	"net/http"

	"bitbucket.org/appmax-space/go-boilerplate/internal/infrastructure/telemetry"
	entityuc "bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity_example"
	"bitbucket.org/appmax-space/go-boilerplate/internal/usecases/entity_example/dto"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/httputil/httpgin"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// EntityHandler agrupa todos os handlers relacionados a Entity.
// Segue o padrão de injeção de dependência (UseCases injetados via struct).
//
// Design choice: this handler depends on concrete use case types for simplicity.
// In a boilerplate/template context, adding handler-level interfaces for each use case
// adds ceremony without clear benefit — handlers are validated via E2E tests, not unit
// tests with mocked use cases. Teams needing handler-level unit tests should define
// interfaces (e.g., Creator, Getter) and accept them here instead.
type EntityHandler struct {
	CreateUC *entityuc.CreateUseCase
	GetUC    *entityuc.GetUseCase
	ListUC   *entityuc.ListUseCase
	UpdateUC *entityuc.UpdateUseCase
	DeleteUC *entityuc.DeleteUseCase
	Metrics  *telemetry.Metrics
}

// NewEntityHandler cria um novo EntityHandler com todos os use cases.
func NewEntityHandler(
	createUC *entityuc.CreateUseCase,
	getUC *entityuc.GetUseCase,
	listUC *entityuc.ListUseCase,
	updateUC *entityuc.UpdateUseCase,
	deleteUC *entityuc.DeleteUseCase,
	metrics *telemetry.Metrics,
) *EntityHandler {
	return &EntityHandler{
		CreateUC: createUC,
		GetUC:    getUC,
		ListUC:   listUC,
		UpdateUC: updateUC,
		DeleteUC: deleteUC,
		Metrics:  metrics,
	}
}

// Create godoc
// @Summary      Create a new entity
// @Description  Create a new entity with the input payload
// @Tags         entities
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateInput true "Entity info"
// @Success      201  {object}  dto.CreateOutput
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /entities [post]
func (h *EntityHandler) Create(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Create")
	defer span.End()

	var req dto.CreateInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.SetStatus(codes.Error, "invalid request body")
		httpgin.SendError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	span.SetAttributes(
		attribute.String("entity.name", req.Name),
	)

	res, execErr := h.CreateUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	span.SetAttributes(attribute.String("entity.id", res.ID))

	// Record metric
	if h.Metrics != nil {
		h.Metrics.RecordCreate(ctx)
	}

	httpgin.SendSuccess(c, http.StatusCreated, res)
}

// GetByID godoc
// @Summary      Get an entity by ID
// @Description  Get entity details by unique ID
// @Tags         entities
// @Produce      json
// @Param        id   path      string  true  "Entity ID"
// @Success      200  {object}  dto.GetOutput
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /entities/{id} [get]
func (h *EntityHandler) GetByID(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.GetByID")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	res, execErr := h.GetUC.Execute(ctx, dto.GetInput{ID: id})
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	httpgin.SendSuccess(c, http.StatusOK, res)
}

// List godoc
// @Summary      List entities
// @Description  Get a paginated list of entities
// @Tags         entities
// @Produce      json
// @Param        page    query     int     false  "Page number"
// @Param        limit   query     int     false  "Items per page"
// @Param        name    query     string  false  "Filter by name"
// @Param        email   query     string  false  "Filter by email"
// @Param        active  query     bool    false  "Filter by active status"
// @Success      200     {object}  dto.ListOutput
// @Failure      500     {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /entities [get]
func (h *EntityHandler) List(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.List")
	defer span.End()

	var req dto.ListInput
	if bindErr := c.ShouldBindQuery(&req); bindErr != nil {
		span.SetStatus(codes.Error, "invalid query parameters")
		httpgin.SendError(c, http.StatusBadRequest, "invalid query parameters")
		return
	}

	span.SetAttributes(
		attribute.Int("filter.page", req.Page),
		attribute.Int("filter.limit", req.Limit),
	)

	res, execErr := h.ListUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	span.SetAttributes(attribute.Int("result.total", res.Pagination.Total))
	httpgin.SendSuccessWithMeta(c, http.StatusOK, res.Data, res.Pagination, nil)
}

// Update godoc
// @Summary      Update an entity
// @Description  Update entity details by ID
// @Tags         entities
// @Accept       json
// @Produce      json
// @Param        id       path      string          true  "Entity ID"
// @Param        request  body      dto.UpdateInput true  "Update info"
// @Success      200      {object}  dto.UpdateOutput
// @Failure      400      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /entities/{id} [put]
func (h *EntityHandler) Update(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Update")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	var req dto.UpdateInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.SetStatus(codes.Error, "invalid request body")
		httpgin.SendError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id // ID vem da URL

	res, execErr := h.UpdateUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	// Record metric
	if h.Metrics != nil {
		h.Metrics.RecordUpdate(ctx)
	}

	httpgin.SendSuccess(c, http.StatusOK, res)
}

// Delete godoc
// @Summary      Delete an entity
// @Description  Soft delete an entity by ID
// @Tags         entities
// @Produce      json
// @Param        id   path      string  true  "Entity ID"
// @Success      200  {object}  dto.DeleteOutput
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /entities/{id} [delete]
func (h *EntityHandler) Delete(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Delete")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	res, execErr := h.DeleteUC.Execute(ctx, dto.DeleteInput{ID: id})
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	// Record metric
	if h.Metrics != nil {
		h.Metrics.RecordDelete(ctx)
	}

	httpgin.SendSuccess(c, http.StatusOK, res)
}
