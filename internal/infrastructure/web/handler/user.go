package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmarcello/go-boilerplate/internal/infrastructure/telemetry"
	useruc "github.com/jrmarcello/go-boilerplate/internal/usecases/user"
	"github.com/jrmarcello/go-boilerplate/internal/usecases/user/dto"
	"github.com/jrmarcello/go-boilerplate/pkg/httputil/httpgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// UserHandler agrupa todos os handlers relacionados a User.
// Segue o padrão de injeção de dependência (UseCases injetados via struct).
//
// Design choice: this handler depends on concrete use case types for simplicity.
// In a boilerplate/template context, adding handler-level interfaces for each use case
// adds ceremony without clear benefit — handlers are validated via E2E tests, not unit
// tests with mocked use cases. Teams needing handler-level unit tests should define
// interfaces (e.g., Creator, Getter) and accept them here instead.
type UserHandler struct {
	CreateUC *useruc.CreateUseCase
	GetUC    *useruc.GetUseCase
	ListUC   *useruc.ListUseCase
	UpdateUC *useruc.UpdateUseCase
	DeleteUC *useruc.DeleteUseCase
	Metrics  *telemetry.Metrics
}

// NewUserHandler cria um novo UserHandler com todos os use cases.
func NewUserHandler(
	createUC *useruc.CreateUseCase,
	getUC *useruc.GetUseCase,
	listUC *useruc.ListUseCase,
	updateUC *useruc.UpdateUseCase,
	deleteUC *useruc.DeleteUseCase,
	metrics *telemetry.Metrics,
) *UserHandler {
	return &UserHandler{
		CreateUC: createUC,
		GetUC:    getUC,
		ListUC:   listUC,
		UpdateUC: updateUC,
		DeleteUC: deleteUC,
		Metrics:  metrics,
	}
}

// Create godoc
// @Summary      Create a new user
// @Description  Create a new user with the input payload
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateInput true "User info"
// @Success      201  {object}  dto.CreateOutput
// @Failure      400  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "UserHandler.Create")
	defer span.End()

	var req dto.CreateInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.SetStatus(codes.Error, "invalid request body")
		httpgin.SendError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	span.SetAttributes(
		attribute.String("user.name", req.Name),
	)

	res, execErr := h.CreateUC.Execute(ctx, req)
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	span.SetAttributes(attribute.String("user.id", res.ID))

	// Record metric
	if h.Metrics != nil {
		h.Metrics.RecordCreate(ctx)
	}

	httpgin.SendSuccess(c, http.StatusCreated, res)
}

// GetByID godoc
// @Summary      Get a user by ID
// @Description  Get user details by unique ID
// @Tags         users
// @Produce      json
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  dto.GetOutput
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /users/{id} [get]
func (h *UserHandler) GetByID(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "UserHandler.GetByID")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("user.id", id))

	res, execErr := h.GetUC.Execute(ctx, dto.GetInput{ID: id})
	if execErr != nil {
		HandleError(c, span, execErr)
		return
	}

	httpgin.SendSuccess(c, http.StatusOK, res)
}

// List godoc
// @Summary      List users
// @Description  Get a paginated list of users
// @Tags         users
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
// @Router       /users [get]
func (h *UserHandler) List(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "UserHandler.List")
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
// @Summary      Update a user
// @Description  Update user details by ID
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id       path      string          true  "User ID"
// @Param        request  body      dto.UpdateInput true  "Update info"
// @Success      200      {object}  dto.UpdateOutput
// @Failure      400      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "UserHandler.Update")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("user.id", id))

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
// @Summary      Delete a user
// @Description  Soft delete a user by ID
// @Tags         users
// @Produce      json
// @Param        id   path      string  true  "User ID"
// @Success      200  {object}  dto.DeleteOutput
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ServiceName
// @Security     ServiceKey
// @Router       /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "UserHandler.Delete")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("user.id", id))

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
