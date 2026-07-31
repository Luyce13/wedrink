package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"wedrink/internal/config"
	"wedrink/internal/db"
	"wedrink/internal/handlers"
	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/repository"
	"wedrink/internal/services"
	"wedrink/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	utils.InitLogger(cfg.Env)

	slog.Info("Starting Wedrink EOD Go Server...", "port", cfg.Port, "env", cfg.Env)

	// Connect to MongoDB
	mongoDB, err := db.Connect(cfg)
	if err != nil {
		slog.Error("Failed to initialize MongoDB client. Please check your MONGO_URI in .env file.", "error", err)
		os.Exit(1)
	}
	defer func() {
		if mongoDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mongoDB.Close(ctx)
		}
	}()

	// Repositories
	userRepo := repository.NewUserRepository(mongoDB.Database)
	reportRepo := repository.NewReportRepository(mongoDB.Database)

	// Seed default users (staff / staffpassword, manager / managerpassword)
	ctxSeed, cancelSeed := context.WithTimeout(context.Background(), 5*time.Second)
	_ = userRepo.SeedDefaultUsers(ctxSeed)
	cancelSeed()

	// Services
	authService := services.NewAuthService(userRepo)
	reportService := services.NewReportService(reportRepo)
	userService := services.NewUserService(userRepo)

	// HTML Template setup with custom FuncMap
	funcMap := template.FuncMap{
		"mathAbs": func(val float64) float64 {
			return math.Abs(val)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"mod": func(a, b int) int {
			return a % b
		},
		// fmtNum formats a float64 as a comma-separated integer string (en-IN style).
		// e.g. 125000 -> "1,25,000"
		"fmtNum": func(val float64) string {
			n := int64(math.Round(math.Abs(val)))
			s := strconv.FormatInt(n, 10)
			var result strings.Builder
			l := len(s)
			if l <= 3 {
				result.WriteString(s)
			} else {
				last3 := s[l-3:]
				prefix := s[:l-3]
				var parts []string
				for len(prefix) > 2 {
					parts = append([]string{prefix[len(prefix)-2:]}, parts...)
					prefix = prefix[:len(prefix)-2]
				}
				if len(prefix) > 0 {
					parts = append([]string{prefix}, parts...)
				}
				parts = append(parts, last3)
				result.WriteString(strings.Join(parts, ","))
			}
			if val < 0 {
				return "-" + result.String()
			}
			return result.String()
		},
		"not": func(v bool) bool { return !v },
	}

	renderer, err := render.NewRenderer(funcMap)
	if err != nil {
		log.Fatalf("Failed to initialize template renderer: %v", err)
	}

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, renderer)
	reportHandler := handlers.NewReportHandler(reportService, renderer)
	dashboardHandler := handlers.NewDashboardHandler(reportService, renderer)
	exportHandler := handlers.NewExportHandler(reportService)
	userHandler := handlers.NewUserHandler(userService, renderer)

	sessionMgr := middleware.NewSessionManager(cfg.SessionSecret)

	// Router setup
	mux := http.NewServeMux()

	// Static file server — no-cache headers so Cloudflare always fetches fresh CSS/JS
	staticDir := render.ResolveProjectPath("web", "static")
	staticFS := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		staticFS.ServeHTTP(w, r)
	})))

	// Auth routes (Public)
	mux.HandleFunc("GET /login", authHandler.RenderLogin)
	mux.HandleFunc("POST /login", authHandler.HandleLogin)
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Protected routes (Staff & Super Admin)
	mux.HandleFunc("GET /{$}", middleware.RequireAuth(dashboardHandler.RenderDashboard))
	mux.HandleFunc("GET /submit", middleware.RequireAuth(reportHandler.RenderSubmitForm))
	mux.HandleFunc("POST /reports", middleware.RequireAuth(reportHandler.HandleSubmit))

	// Super Admin protected routes
	mux.HandleFunc("GET /reports", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderReportsList))
	mux.HandleFunc("GET /reports/detail", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderDetailModal))
	mux.HandleFunc("GET /reports/edit", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderEditForm))
	mux.HandleFunc("DELETE /reports/delete", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.HandleDelete))
	mux.HandleFunc("POST /reports/delete", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.HandleDelete))
	mux.HandleFunc("GET /export/csv", middleware.RequireRole(models.RoleSuperAdmin)(exportHandler.ExportCSV))
	mux.HandleFunc("GET /export/excel", middleware.RequireRole(models.RoleSuperAdmin)(exportHandler.ExportExcel))

	// User Management (Super Admin)
	mux.HandleFunc("GET /admin/users", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.RenderUserList))
	mux.HandleFunc("POST /admin/users/create", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleCreateUser))
	mux.HandleFunc("GET /admin/users/edit", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.RenderEditUserModal))
	mux.HandleFunc("POST /admin/users/edit", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleEditUser))
	mux.HandleFunc("POST /admin/users/delete", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleDeleteUser))

	// Global Middlewares (Logger + Auth Context Injection)
	handler := middleware.Logger(sessionMgr.AuthMiddleware(mux))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info(fmt.Sprintf("Wedrink Server listening on http://localhost:%s", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-stop
	slog.Info("Shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown forced", "error", err)
	}

	slog.Info("Server stopped cleanly.")
}
