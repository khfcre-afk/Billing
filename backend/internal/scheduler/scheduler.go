// Package scheduler runs periodic billing jobs: the daily charge debits active
// servers from their owners' balances and suspends servers whose balance runs
// out. All changes are written inside a single transaction per server so that
// balance and server status stay consistent.
package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/pterodactyl"
)

type Scheduler struct {
	db    *pgxpool.Pool
	ptero *pterodactyl.Client
	cron  *cron.Cron
}

func New(db *pgxpool.Pool, ptero *pterodactyl.Client) *Scheduler {
	return &Scheduler{db: db, ptero: ptero, cron: cron.New()}
}

func (s *Scheduler) Start(ctx context.Context, spec string) error {
	_, err := s.cron.AddFunc(spec, func() {
		if err := s.DailyCharge(ctx); err != nil {
			log.Error().Err(err).Msg("daily charge failed")
		}
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	return nil
}

func (s *Scheduler) Stop() { <-s.cron.Stop().Done() }

type dueServer struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	PteroID     *int
	DailyCost   int64
}

// DailyCharge iterates over all active servers whose paid_until < now() and
// either debits the next day of rent or suspends the server.
func (s *Scheduler) DailyCharge(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT s.id, s.user_id, s.ptero_server_id, t.daily_cost_cents
		FROM servers s JOIN tariffs t ON t.id = s.tariff_id
		WHERE s.status = 'active' AND s.paid_until < now()`)
	if err != nil {
		return err
	}
	var list []dueServer
	for rows.Next() {
		var d dueServer
		if err := rows.Scan(&d.ID, &d.UserID, &d.PteroID, &d.DailyCost); err != nil {
			rows.Close()
			return err
		}
		list = append(list, d)
	}
	rows.Close()

	for _, d := range list {
		if err := s.chargeOne(ctx, d); err != nil {
			log.Error().Err(err).Str("server", d.ID.String()).Msg("charge failed")
		}
	}
	return nil
}

func (s *Scheduler) chargeOne(ctx context.Context, d dueServer) error {
	suspended := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var balance int64
		if err := tx.QueryRow(ctx,
			`SELECT balance_cents FROM users WHERE id = $1 FOR UPDATE`, d.UserID,
		).Scan(&balance); err != nil {
			return err
		}
		if balance < d.DailyCost {
			if _, err := tx.Exec(ctx,
				`UPDATE servers SET status = 'suspended', updated_at = now() WHERE id = $1`, d.ID,
			); err != nil {
				return err
			}
			suspended = true
			return nil
		}
		var newBal int64
		if err := tx.QueryRow(ctx, `
			UPDATE users SET balance_cents = balance_cents - $2, updated_at = now()
			WHERE id = $1 RETURNING balance_cents`, d.UserID, d.DailyCost,
		).Scan(&newBal); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE servers
			SET paid_until = paid_until + INTERVAL '1 day',
			    last_charged_at = now(),
			    updated_at = now()
			WHERE id = $1`, d.ID,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO transactions (user_id, kind, amount_cents, balance_after, description, server_id)
			VALUES ($1,'daily_charge',$2,$3,$4,$5)`,
			d.UserID, -d.DailyCost, newBal, "Daily rent charge", d.ID,
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if suspended && d.PteroID != nil {
		ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := s.ptero.Suspend(ctx2, *d.PteroID); err != nil {
			log.Error().Err(err).Int("ptero_id", *d.PteroID).Msg("suspend on ptero failed")
		}
	}
	return nil
}
