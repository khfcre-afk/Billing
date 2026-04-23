package tariffs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
)

type Tariff struct {
	ID              uuid.UUID         `json:"id"`
	Slug            string            `json:"slug"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	PriceCents      int64             `json:"price_cents"`
	DailyCostCents  int64             `json:"daily_cost_cents"`
	CPULimit        int               `json:"cpu_limit"`
	MemoryMB        int               `json:"memory_mb"`
	DiskMB          int               `json:"disk_mb"`
	SwapMB          int               `json:"swap_mb"`
	IOWeight        int               `json:"io_weight"`
	EggID           int               `json:"egg_id"`
	NestID          int               `json:"nest_id"`
	DockerImage     string            `json:"docker_image"`
	StartupCommand  string            `json:"startup_command"`
	Environment     map[string]string `json:"environment"`
	IsActive        bool              `json:"is_active"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type Repo struct{ db *pgxpool.Pool }

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) List(ctx context.Context, onlyActive bool) ([]Tariff, error) {
	q := `SELECT id, slug, name, description, price_cents, daily_cost_cents, cpu_limit,
	             memory_mb, disk_mb, swap_mb, io_weight, egg_id, nest_id, docker_image,
	             startup_command, environment, is_active, created_at, updated_at
	      FROM tariffs`
	if onlyActive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY price_cents ASC`

	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tariff{}
	for rows.Next() {
		t, err := scanTariff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (*Tariff, error) {
	return r.getBy(ctx, r.db, `id = $1`, id)
}

func (r *Repo) GetTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Tariff, error) {
	return r.getByTx(ctx, tx, `id = $1`, id)
}

func (r *Repo) getBy(ctx context.Context, q querier, where string, args ...any) (*Tariff, error) {
	row := q.QueryRow(ctx, `
		SELECT id, slug, name, description, price_cents, daily_cost_cents, cpu_limit,
		       memory_mb, disk_mb, swap_mb, io_weight, egg_id, nest_id, docker_image,
		       startup_command, environment, is_active, created_at, updated_at
		FROM tariffs WHERE `+where, args...)
	t, err := scanTariffRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return t, err
}

func (r *Repo) getByTx(ctx context.Context, tx pgx.Tx, where string, args ...any) (*Tariff, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, slug, name, description, price_cents, daily_cost_cents, cpu_limit,
		       memory_mb, disk_mb, swap_mb, io_weight, egg_id, nest_id, docker_image,
		       startup_command, environment, is_active, created_at, updated_at
		FROM tariffs WHERE `+where, args...)
	t, err := scanTariffRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	return t, err
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type CreateInput struct {
	Slug            string            `json:"slug" validate:"required,min=2,max=64"`
	Name            string            `json:"name" validate:"required,min=2,max=120"`
	Description     string            `json:"description"`
	PriceCents      int64             `json:"price_cents" validate:"gte=0"`
	DailyCostCents  int64             `json:"daily_cost_cents" validate:"gte=0"`
	CPULimit        int               `json:"cpu_limit" validate:"gt=0"`
	MemoryMB        int               `json:"memory_mb" validate:"gt=0"`
	DiskMB          int               `json:"disk_mb" validate:"gt=0"`
	SwapMB          int               `json:"swap_mb"`
	IOWeight        int               `json:"io_weight"`
	EggID           int               `json:"egg_id" validate:"gt=0"`
	NestID          int               `json:"nest_id" validate:"gt=0"`
	DockerImage     string            `json:"docker_image" validate:"required"`
	StartupCommand  string            `json:"startup_command" validate:"required"`
	Environment     map[string]string `json:"environment"`
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (*Tariff, error) {
	env, _ := json.Marshal(in.Environment)
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO tariffs (slug, name, description, price_cents, daily_cost_cents,
		                     cpu_limit, memory_mb, disk_mb, swap_mb, io_weight,
		                     egg_id, nest_id, docker_image, startup_command, environment)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		in.Slug, in.Name, in.Description, in.PriceCents, in.DailyCostCents,
		in.CPULimit, in.MemoryMB, in.DiskMB, in.SwapMB, in.IOWeight,
		in.EggID, in.NestID, in.DockerImage, in.StartupCommand, env,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *Repo) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := r.db.Exec(ctx, `UPDATE tariffs SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	return err
}

func scanTariff(rows pgx.Rows) (*Tariff, error) {
	var (
		t      Tariff
		envRaw []byte
	)
	err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.PriceCents, &t.DailyCostCents,
		&t.CPULimit, &t.MemoryMB, &t.DiskMB, &t.SwapMB, &t.IOWeight, &t.EggID, &t.NestID,
		&t.DockerImage, &t.StartupCommand, &envRaw, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(envRaw, &t.Environment)
	if t.Environment == nil {
		t.Environment = map[string]string{}
	}
	return &t, nil
}

func scanTariffRow(row pgx.Row) (*Tariff, error) {
	var (
		t      Tariff
		envRaw []byte
	)
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.PriceCents, &t.DailyCostCents,
		&t.CPULimit, &t.MemoryMB, &t.DiskMB, &t.SwapMB, &t.IOWeight, &t.EggID, &t.NestID,
		&t.DockerImage, &t.StartupCommand, &envRaw, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(envRaw, &t.Environment)
	if t.Environment == nil {
		t.Environment = map[string]string{}
	}
	return &t, nil
}
