package handler

import (
	"github.com/MicahParks/keyfunc/v3"
	"github.com/appraisal-crm/inspect-service/internal/middleware"
	"github.com/appraisal-crm/inspect-service/internal/service"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(svc service.InspectionService, jwks keyfunc.Keyfunc, allowedOrigins []string) *chi.Mux {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}))
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Public.
	r.Get("/health", Health)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Authenticated.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwks))

		ih := newInspectionHandler(svc)

		r.With(middleware.RequireRoles("inspector", "appraiser", "admin")).Get("/inspections", ih.List)
		r.With(middleware.RequireRoles("inspector", "appraiser", "admin")).Get("/inspections/{id}", ih.GetByID)
		r.With(middleware.RequireRoles("inspector", "appraiser", "admin")).Patch("/inspections/{id}", ih.Update)
		r.With(middleware.RequireRoles("inspector")).Post("/inspections/{id}/photos", ih.AddPhoto)
		r.With(middleware.RequireRoles("inspector")).Post("/inspections/{id}/complete", ih.Complete)
	})

	return r
}
