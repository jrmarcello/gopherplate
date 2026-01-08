package handler

import (
	"net/http"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/telemetry"
	entityuc "bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/entity/dto"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// EntityHandler agrupa todos os handlers relacionados a Entity.
// Segue o padrão de injeção de dependência (UseCases injetados via struct).
type EntityHandler struct {
	CreateUC *entityuc.CreateUseCase
	GetUC    *entityuc.GetUseCase
	ListUC   *entityuc.ListUseCase
	UpdateUC *entityuc.UpdateUseCase
	DeleteUC *entityuc.DeleteUseCase
}

// NewEntityHandler cria um novo EntityHandler com todos os use cases.
func NewEntityHandler(
	createUC *entityuc.CreateUseCase,
	getUC *entityuc.GetUseCase,
	listUC *entityuc.ListUseCase,
	updateUC *entityuc.UpdateUseCase,
	deleteUC *entityuc.DeleteUseCase,
) *EntityHandler {
	return &EntityHandler{
		CreateUC: createUC,
		GetUC:    getUC,
		ListUC:   listUC,
		UpdateUC: updateUC,
		DeleteUC: deleteUC,
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
// @Failure      400  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /entities [post]
func (h *EntityHandler) Create(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Create")
	defer span.End()

	var req dto.CreateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetStatus(codes.Error, "invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	span.SetAttributes(
		attribute.String("entity.name", req.Name),
		attribute.String("entity.email", req.Email),
	)

	res, err := h.CreateUC.Execute(ctx, req)
	if err != nil {
		HandleError(c, span, err)
		return
	}

	span.SetAttributes(attribute.String("entity.id", res.ID))

	// Record metric
	if m := telemetry.GetMetrics(); m != nil {
		m.RecordCreate(ctx)
	}

	c.JSON(http.StatusCreated, res)
}

// GetByID godoc
// @Summary      Get an entity by ID
// @Description  Get entity details by unique ID
// @Tags         entities
// @Produce      json
// @Param        id   path      string  true  "Entity ID"
// @Success      200  {object}  dto.GetOutput
// @Failure      404  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /entities/{id} [get]
func (h *EntityHandler) GetByID(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.GetByID")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	res, err := h.GetUC.Execute(ctx, dto.GetInput{ID: id})
	if err != nil {
		HandleError(c, span, err)
		return
	}

	c.JSON(http.StatusOK, res)
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
// @Failure      500     {object}  middleware.ErrorResponse
// @Router       /entities [get]
func (h *EntityHandler) List(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.List")
	defer span.End()

	var req dto.ListInput
	if err := c.ShouldBindQuery(&req); err != nil {
		HandleError(c, span, err)
		return
	}

	span.SetAttributes(
		attribute.Int("filter.page", req.Page),
		attribute.Int("filter.limit", req.Limit),
	)

	res, err := h.ListUC.Execute(ctx, req)
	if err != nil {
		HandleError(c, span, err)
		return
	}

	span.SetAttributes(attribute.Int("result.total", res.Pagination.Total))
	c.JSON(http.StatusOK, res)
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
// @Failure      400      {object}  middleware.ErrorResponse
// @Failure      404      {object}  middleware.ErrorResponse
// @Failure      500      {object}  middleware.ErrorResponse
// @Router       /entities/{id} [put]
func (h *EntityHandler) Update(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Update")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	var req dto.UpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetStatus(codes.Error, "invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}
	req.ID = id // ID vem da URL

	res, err := h.UpdateUC.Execute(ctx, req)
	if err != nil {
		HandleError(c, span, err)
		return
	}

	// Record metric
	if m := telemetry.GetMetrics(); m != nil {
		m.RecordUpdate(ctx)
	}

	c.JSON(http.StatusOK, res)
}

// Delete godoc
// @Summary      Delete an entity
// @Description  Soft delete an entity by ID
// @Tags         entities
// @Produce      json
// @Param        id   path      string  true  "Entity ID"
// @Success      200  {object}  dto.DeleteOutput
// @Failure      404  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /entities/{id} [delete]
func (h *EntityHandler) Delete(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "EntityHandler.Delete")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("entity.id", id))

	res, err := h.DeleteUC.Execute(ctx, dto.DeleteInput{ID: id})
	if err != nil {
		HandleError(c, span, err)
		return
	}

	// Record metric
	if m := telemetry.GetMetrics(); m != nil {
		m.RecordDelete(ctx)
	}

	c.JSON(http.StatusOK, res)
}
