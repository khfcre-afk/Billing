package app

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/auth"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/billing"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/config"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/db"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/promo"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/pterodactyl"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/scheduler"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/servers"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/tariffs"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/users"
)

//go:embed docs/*
var docsFS embed.FS

type App struct {
	cfg       config.Config
	pool      *pgxpool.Pool
	rdb       *redis.Client
	router    http.Handler
	scheduler *scheduler.Scheduler
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := db.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		log.Warn().Err(err).Msg("redis ping failed; rate limiting will be best-effort")
	}

	ptero := pterodactyl.New(cfg.Ptero.BaseURL, cfg.Ptero.AppAPIKey, cfg.Ptero.ClientAPIKey)

	issuer := auth.NewIssuer(cfg.Security.JWTSecret, cfg.Security.JWTAccessTTL, cfg.Security.JWTRefreshTTL)
	authSvc := auth.NewService(pool, issuer)
	authH := auth.NewHandler(authSvc)

	usersRepo := users.NewRepo(pool)
	usersH := users.NewHandler(usersRepo)

	tariffRepo := tariffs.NewRepo(pool)
	purchase := tariffs.NewPurchaseService(pool, tariffRepo, ptero, cfg.Ptero.DefaultNodeID)
	tariffH := tariffs.NewHandler(tariffRepo, purchase)

	promoSvc := promo.NewService(pool, purchase)
	promoH := promo.NewHandler(promoSvc)
	promoAdminH := promo.NewAdminHandler(promo.NewAdminRepo(pool))

	serverRepo := servers.NewRepo(pool)
	serverH := servers.NewHandler(serverRepo, ptero)

	txRepo := billing.NewRepo(pool)
	txH := billing.NewHandler(txRepo)

	sched := scheduler.New(pool, ptero)

	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.Recoverer, httpx.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"payments_disabled": cfg.Security.PaymentsDisabled,
			"time":              time.Now(),
		})
	})

	sub, _ := fs.Sub(docsFS, "docs")
	r.Handle("/docs", http.RedirectHandler("/docs/", http.StatusMovedPermanently))
	r.Handle("/docs/*", http.StripPrefix("/docs/", http.FileServer(http.FS(sub))))

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Use(httprate.LimitByIP(30, time.Minute))
			r.Mount("/", authH.Routes())
		})

		r.Group(func(r chi.Router) {
			r.Get("/tariffs", tariffH.List)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(issuer))
			r.Mount("/users", usersH.Routes())
			r.Mount("/promo", promoH.Routes())
			r.Post("/tariffs/{id}/purchase", tariffH.Purchase)
			r.Mount("/servers", serverH.Routes())
			r.Mount("/transactions", txH.Routes())

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Mount("/admin/promo", promoAdminH.Routes())
				r.Mount("/admin/tariffs", tariffH.AdminRoutes())
			})
		})
	})

	return &App{cfg: cfg, pool: pool, rdb: rdb, router: r, scheduler: sched}, nil
}

func (a *App) Router() http.Handler { return a.router }

func (a *App) StartScheduler(ctx context.Context) {
	if err := a.scheduler.Start(ctx, a.cfg.Scheduler.DailyChargeCron); err != nil {
		log.Error().Err(err).Msg("scheduler start failed")
	}
}

func (a *App) Close() {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	if a.rdb != nil {
		_ = a.rdb.Close()
	}
	if a.pool != nil {
		a.pool.Close()
	}
}
