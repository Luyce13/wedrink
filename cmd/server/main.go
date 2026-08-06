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

	// Seed default users only when AUTO_SEED=true (opt-in, not automatic)
	if cfg.AutoSeed {
		ctxSeed, cancelSeed := context.WithTimeout(context.Background(), 5*time.Second)
		if err := userRepo.SeedDefaultUsers(ctxSeed); err != nil {
			slog.Warn("Default user seeding encountered an error", "error", err)
		}
		cancelSeed()
	}

	notifRepo := repository.NewNotificationRepository(mongoDB.Database)
	auditRepo := repository.NewAuditRepository(mongoDB.Database)

	// Services
	auditService := services.NewAuditService(auditRepo)
	authService := services.NewAuthService(userRepo, auditService)
	reportService := services.NewReportService(reportRepo, auditService)
	userService := services.NewUserService(userRepo, auditService)
	notifService := services.NewNotificationService(notifRepo)

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
	reportHandler := handlers.NewReportHandler(reportService, authService, notifService, renderer)
	dashboardHandler := handlers.NewDashboardHandler(reportService, renderer)
	exportHandler := handlers.NewExportHandler(reportService)
	userHandler := handlers.NewUserHandler(userService, renderer)
	notifHandler := handlers.NewNotificationHandler(notifService, renderer)

	sessionMgr := middleware.NewSessionManager(cfg.SessionSecret)

	// Rate limiters for sensitive endpoints
	loginRateLimiter := middleware.Limit(5, 0.1)  // 5 max burst, 6 requests/min refill
	writeRateLimiter := middleware.Limit(10, 0.5) // 10 max burst, 30 requests/min refill

	// Router setup
	mux := http.NewServeMux()

	// Static file server — long-lived immutable cache (filenames versioned via ?v= params)
	staticDir := render.ResolveProjectPath("web", "static")
	staticFS := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		staticFS.ServeHTTP(w, r)
	})))

	// Auth routes (Public)
	mux.HandleFunc("GET /login", authHandler.RenderLogin)
	mux.Handle("POST /login", loginRateLimiter(http.HandlerFunc(authHandler.HandleLogin)))
	mux.HandleFunc("POST /logout", authHandler.HandleLogout)

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Protected routes (Staff & Super Admin)
	mux.HandleFunc("GET /{$}", middleware.RequireAuth(dashboardHandler.RenderDashboard))
	mux.HandleFunc("GET /submit", middleware.RequireAuth(reportHandler.RenderSubmitForm))
	mux.Handle("POST /reports", writeRateLimiter(http.HandlerFunc(middleware.RequireAuth(reportHandler.HandleSubmit))))
	mux.HandleFunc("GET /reports/submitted-dates", middleware.RequireAuth(reportHandler.GetSubmittedDates))
	mux.HandleFunc("GET /reports/check-date", middleware.RequireAuth(reportHandler.CheckReportDate))

	// Super Admin protected routes
	mux.HandleFunc("GET /reports", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderReportsList))
	mux.HandleFunc("GET /reports/detail", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderDetailModal))
	mux.HandleFunc("GET /reports/edit", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderEditForm))
	mux.HandleFunc("GET /reports/delete-confirm", middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.RenderDeleteConfirmModal))
	mux.Handle("DELETE /reports/delete", writeRateLimiter(http.HandlerFunc(middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.HandleDelete))))
	mux.Handle("POST /reports/delete", writeRateLimiter(http.HandlerFunc(middleware.RequireRole(models.RoleSuperAdmin)(reportHandler.HandleDelete))))
	mux.HandleFunc("GET /export/csv", middleware.RequireRole(models.RoleSuperAdmin)(exportHandler.ExportCSV))
	mux.HandleFunc("GET /export/excel", middleware.RequireRole(models.RoleSuperAdmin)(exportHandler.ExportExcel))

	// Notification routes (Super Admin / Manager)
	mux.HandleFunc("GET /notifications/unread", middleware.RequireRole(models.RoleSuperAdmin)(notifHandler.GetUnread))
	mux.HandleFunc("GET /notifications/list", middleware.RequireRole(models.RoleSuperAdmin)(notifHandler.GetList))
	mux.HandleFunc("GET /notifications/badge", middleware.RequireRole(models.RoleSuperAdmin)(notifHandler.GetBadge))
	mux.HandleFunc("POST /notifications/mark-read", middleware.RequireRole(models.RoleSuperAdmin)(notifHandler.MarkAsRead))

	// User Management (Super Admin)
	mux.HandleFunc("GET /admin/users", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.RenderUserList))
	mux.Handle("POST /admin/users/create", writeRateLimiter(http.HandlerFunc(middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleCreateUser))))
	mux.HandleFunc("GET /admin/users/edit", middleware.RequireRole(models.RoleSuperAdmin)(userHandler.RenderEditUserModal))
	mux.Handle("POST /admin/users/edit", writeRateLimiter(http.HandlerFunc(middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleEditUser))))
	mux.Handle("POST /admin/users/delete", writeRateLimiter(http.HandlerFunc(middleware.RequireRole(models.RoleSuperAdmin)(userHandler.HandleDeleteUser))))

	noCacheMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			next.ServeHTTP(w, r)
		})
	}

	// Global Middlewares (Logger + NoCache + Auth Context Injection)
	handler := middleware.Logger(noCacheMiddleware(sessionMgr.AuthMiddleware(mux)))

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
