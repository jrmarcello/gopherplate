// Package bootstrap is the composition root for the application.
// It wires all dependencies (repos, use cases, handlers) into a typed Container.
// This is the only package allowed to import from all architecture layers.
package bootstrap

import (
	"github.com/jmoiron/sqlx"

	"github.com/jrmarcello/gopherplate/internal/infrastructure/db/postgres/repository"
	infratelemetry "github.com/jrmarcello/gopherplate/internal/infrastructure/telemetry"
	"github.com/jrmarcello/gopherplate/internal/infrastructure/web/handler"
	roleuc "github.com/jrmarcello/gopherplate/internal/usecases/role"
	useruc "github.com/jrmarcello/gopherplate/internal/usecases/user"
	"github.com/jrmarcello/gopherplate/pkg/cache"
)

// Container holds all application dependencies grouped by layer.
type Container struct {
	Repos        Repos
	UserUseCases UserUseCases
	RoleUseCases RoleUseCases
	Handlers     Handlers
}

// Repos groups all repository implementations.
type Repos struct {
	User *repository.UserRepository
	Role *repository.RoleRepository
}

// UserUseCases groups all user domain use cases.
type UserUseCases struct {
	Create *useruc.CreateUseCase
	Get    *useruc.GetUseCase
	List   *useruc.ListUseCase
	Update *useruc.UpdateUseCase
	Delete *useruc.DeleteUseCase
}

// RoleUseCases groups all role domain use cases.
type RoleUseCases struct {
	Create *roleuc.CreateUseCase
	List   *roleuc.ListUseCase
	Delete *roleuc.DeleteUseCase
}

// Handlers groups all HTTP handlers.
type Handlers struct {
	User *handler.UserHandler
	Role *handler.RoleHandler
}

// New creates a fully wired Container. The construction follows a strict phase order:
// repos -> use cases -> handlers, preventing circular dependencies.
// metrics may be nil (for tests or contexts without OTel).
func New(writer, reader *sqlx.DB, cacheClient cache.Cache, metrics *infratelemetry.Metrics) *Container {
	c := &Container{}
	c.buildRepos(writer, reader)
	c.buildUseCases(cacheClient)
	c.buildHandlers(metrics)
	return c
}

func (c *Container) buildRepos(writer, reader *sqlx.DB) {
	c.Repos = Repos{
		User: repository.NewUserRepository(writer, reader),
		Role: repository.NewRoleRepository(writer, reader),
	}
}

func (c *Container) buildUseCases(cacheClient cache.Cache) {
	flightGroup := cache.NewFlightGroup()

	c.UserUseCases = UserUseCases{
		Create: useruc.NewCreateUseCase(c.Repos.User),
		Get:    useruc.NewGetUseCase(c.Repos.User).WithCache(cacheClient).WithFlight(flightGroup),
		List:   useruc.NewListUseCase(c.Repos.User),
		Update: useruc.NewUpdateUseCase(c.Repos.User).WithCache(cacheClient),
		Delete: useruc.NewDeleteUseCase(c.Repos.User).WithCache(cacheClient),
	}

	c.RoleUseCases = RoleUseCases{
		Create: roleuc.NewCreateUseCase(c.Repos.Role),
		List:   roleuc.NewListUseCase(c.Repos.Role),
		Delete: roleuc.NewDeleteUseCase(c.Repos.Role),
	}
}

func (c *Container) buildHandlers(metrics *infratelemetry.Metrics) {
	c.Handlers = Handlers{
		User: handler.NewUserHandler(
			c.UserUseCases.Create,
			c.UserUseCases.Get,
			c.UserUseCases.List,
			c.UserUseCases.Update,
			c.UserUseCases.Delete,
			metrics,
		),
		Role: handler.NewRoleHandler(
			c.RoleUseCases.Create,
			c.RoleUseCases.List,
			c.RoleUseCases.Delete,
		),
	}
}
