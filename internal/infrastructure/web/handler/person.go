package handler

import (
	"net/http"

	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/infrastructure/telemetry"
	personuc "bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/person"
	"bitbucket.org/appmax-space/ms-boilerplate-go/internal/usecases/person/dto"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// PersonHandler agrupa todos os handlers relacionados a Person.
// Segue o padrão de injeção de dependência (UseCases injetados via struct).
type PersonHandler struct {
	CreateUC *personuc.CreateUseCase
	GetUC    *personuc.GetUseCase
	ListUC   *personuc.ListUseCase
	UpdateUC *personuc.UpdateUseCase
	DeleteUC *personuc.DeleteUseCase
}

// NewPersonHandler cria um novo PersonHandler com todos os use cases.
func NewPersonHandler(
	createUC *personuc.CreateUseCase,
	getUC *personuc.GetUseCase,
	listUC *personuc.ListUseCase,
	updateUC *personuc.UpdateUseCase,
	deleteUC *personuc.DeleteUseCase,
) *PersonHandler {
	return &PersonHandler{
		CreateUC: createUC,
		GetUC:    getUC,
		ListUC:   listUC,
		UpdateUC: updateUC,
		DeleteUC: deleteUC,
	}
}

// Create godoc
// @Summary      Create a new person
// @Description  Create a new person with the input payload
// @Tags         persons
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateInput true "Person info"
// @Success      201  {object}  dto.CreateOutput
// @Failure      400  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /person [post]
func (h *PersonHandler) Create(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "PersonHandler.Create")
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
		attribute.String("person.name", req.Name),
		attribute.String("person.email", req.Email),
	)

	res, err := h.CreateUC.Execute(ctx, req)
	if err != nil {
		HandleError(c, span, err)
		return
	}

	span.SetAttributes(attribute.String("person.id", res.ID))

	// Record metric
	if m := telemetry.GetMetrics(); m != nil {
		m.RecordCreate(ctx)
	}

	c.JSON(http.StatusCreated, res)
}

// GetByID godoc
// @Summary      Get a person by ID
// @Description  Get person details by unique ID
// @Tags         persons
// @Produce      json
// @Param        id   path      string  true  "Person ID"
// @Success      200  {object}  dto.GetOutput
// @Failure      404  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /person/{id} [get]
func (h *PersonHandler) GetByID(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "PersonHandler.GetByID")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("person.id", id))

	res, err := h.GetUC.Execute(ctx, dto.GetInput{ID: id})
	if err != nil {
		HandleError(c, span, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// List godoc
// @Summary      List persons
// @Description  Get a paginated list of persons
// @Tags         persons
// @Produce      json
// @Param        page    query     int     false  "Page number"
// @Param        limit   query     int     false  "Items per page"
// @Param        name    query     string  false  "Filter by name"
// @Param        email   query     string  false  "Filter by email"
// @Param        active  query     bool    false  "Filter by active status"
// @Success      200     {object}  dto.ListOutput
// @Failure      500     {object}  middleware.ErrorResponse
// @Router       /person [get]
func (h *PersonHandler) List(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "PersonHandler.List")
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
// @Summary      Update a person
// @Description  Update person details by ID
// @Tags         persons
// @Accept       json
// @Produce      json
// @Param        id       path      string          true  "Person ID"
// @Param        request  body      dto.UpdateInput true  "Update info"
// @Success      200      {object}  dto.UpdateOutput
// @Failure      400      {object}  middleware.ErrorResponse
// @Failure      404      {object}  middleware.ErrorResponse
// @Failure      500      {object}  middleware.ErrorResponse
// @Router       /person/{id} [put]
func (h *PersonHandler) Update(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "PersonHandler.Update")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("person.id", id))

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
// @Summary      Delete a person
// @Description  Soft delete a person by ID
// @Tags         persons
// @Produce      json
// @Param        id   path      string  true  "Person ID"
// @Success      200  {object}  dto.DeleteOutput
// @Failure      404  {object}  middleware.ErrorResponse
// @Failure      500  {object}  middleware.ErrorResponse
// @Router       /person/{id} [delete]
func (h *PersonHandler) Delete(c *gin.Context) {
	ctx, span := otel.Tracer("http-handler").Start(c.Request.Context(), "PersonHandler.Delete")
	defer span.End()

	id := c.Param("id")
	span.SetAttributes(attribute.String("person.id", id))

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
