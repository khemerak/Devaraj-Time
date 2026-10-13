package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devaraj-time/internal/auth"
	"devaraj-time/internal/database"
	"devaraj-time/internal/seed"
	"devaraj-time/internal/server"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func run() error {
	command := flag.String("command", "serve", "serve, migrate, seed, purge-location")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	if *command == "migrate" {
		migrationDSN := dsn
		if len(dsn) >= 11 && dsn[:11] == "postgres://" {
			migrationDSN = "pgx5://" + dsn[11:]
		} else if len(dsn) >= 13 && dsn[:13] == "postgresql://" {
			migrationDSN = "pgx5://" + dsn[13:]
		} else {
			return errors.New("DATABASE_URL must start with postgres:// or postgresql://")
		}
		m, e := migrate.New("file://"+env("MIGRATIONS_PATH", "migrations"), migrationDSN)
		if e != nil {
			return e
		}
		defer m.Close()
		e = m.Up()
		if errors.Is(e, migrate.ErrNoChange) {
			return nil
		}
		return e
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		return e
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		return e
	}
	if *command == "seed" {
		if env("APP_ENV", "development") != "development" {
			return errors.New("demo seeding is disabled outside development")
		}
		password := os.Getenv("DEMO_PASSWORD")
		if len(password) < 12 {
			return errors.New("set DEMO_PASSWORD to at least 12 characters")
		}
		return seed.Run(ctx, db, password, env("DEMO_TIMEZONE", "Asia/Phnom_Penh"))
	}
	if *command == "purge-location" {
		return database.PurgeLocation(ctx, db)
	}
	if *command == "bootstrap-admin" {
		email, password := os.Getenv("PLATFORM_ADMIN_EMAIL"), os.Getenv("PLATFORM_ADMIN_PASSWORD")
		if email == "" || len(password) < 12 {
			return errors.New("set PLATFORM_ADMIN_EMAIL and PLATFORM_ADMIN_PASSWORD (12+ characters)")
		}
		_, e = db.Exec(ctx, `INSERT INTO users(email,name,password_hash,platform_admin) VALUES(lower($1),'Platform Administrator',$2,true)`, email, auth.Hash(password))
		return e
	}
	if *command != "serve" {
		return errors.New("unknown command")
	}
	cfg := server.Config{Origin: env("PUBLIC_URL", "http://localhost:5173"), Secure: env("COOKIE_SECURE", "false") == "true", Production: env("APP_ENV", "development") == "production"}
	if duration, err := time.ParseDuration(env("PASSWORD_RESET_TTL", "15m")); err != nil || duration < time.Minute || duration > time.Hour {
		return errors.New("PASSWORD_RESET_TTL must be between 1m and 1h")
	} else {
		cfg.ResetTTL = duration
	}
	if e = server.ValidateConfig(cfg); e != nil {
		return e
	}
	srv := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: server.New(db, cfg).Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	slog.Info("API listening", "address", srv.Addr)
	e = srv.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func main() {
	if e := run(); e != nil {
		slog.Error("startup failed", "error", e)
		os.Exit(1)
	}
}
