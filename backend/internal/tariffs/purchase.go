package tariffs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/httpx"
	"github.com/khfcre-afk/pterodactyl-billing/backend/internal/pterodactyl"
)

// PurchaseService wires balance deduction, DB server row creation, and
// Pterodactyl provisioning. It implements promo.ServerProvisioner.
type PurchaseService struct {
	db       *pgxpool.Pool
	repo     *Repo
	ptero    *pterodactyl.Client
	nodeID   int
}

func NewPurchaseService(db *pgxpool.Pool, repo *Repo, ptero *pterodactyl.Client, defaultNodeID int) *PurchaseService {
	return &PurchaseService{db: db, repo: repo, ptero: ptero, nodeID: defaultNodeID}
}

// Purchase deducts the tariff price from the user's balance and provisions a server.
// The DB transaction covers balance + server row + transaction log; external
// Pterodactyl calls happen before commit and their result is persisted so that
// the user sees accurate state. If Pterodactyl fails, the whole transaction is
// rolled back and the user is not charged.
func (s *PurchaseService) Purchase(ctx context.Context, userID, tariffID uuid.UUID) (uuid.UUID, error) {
	var serverID uuid.UUID
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tariff, err := s.repo.GetTx(ctx, tx, tariffID)
		if err != nil {
			return err
		}
		if !tariff.IsActive {
			return httpx.WithDetail(httpx.ErrBadRequest, "tariff is inactive")
		}

		var (
			email       string
			pteroUserID *int
			balance     int64
		)
		if err := tx.QueryRow(ctx,
			`SELECT email, ptero_user_id, balance_cents FROM users WHERE id = $1 FOR UPDATE`, userID,
		).Scan(&email, &pteroUserID, &balance); err != nil {
			return err
		}
		if balance < tariff.PriceCents {
			return httpx.WithDetail(httpx.ErrBadRequest, "insufficient balance")
		}

		pteroUID, err := s.ensurePteroUser(ctx, tx, userID, email, pteroUserID)
		if err != nil {
			return fmt.Errorf("ensure ptero user: %w", err)
		}

		alloc, err := s.pickAllocation(ctx)
		if err != nil {
			return err
		}

		srvName := "srv-" + shortID()
		created, err := s.ptero.CreateServer(ctx, pterodactyl.CreateServerInput{
			Name:           srvName,
			UserID:         pteroUID,
			EggID:          tariff.EggID,
			NestID:         tariff.NestID,
			DockerImage:    tariff.DockerImage,
			StartupCommand: tariff.StartupCommand,
			Environment:    tariff.Environment,
			Limits: pterodactyl.Limits{
				Memory: tariff.MemoryMB, Swap: tariff.SwapMB, Disk: tariff.DiskMB,
				IO: orDefault(tariff.IOWeight, 500), CPU: tariff.CPULimit,
			},
			FeatureLimits: pterodactyl.FeatureLimits{Databases: 1, Backups: 1, Allocations: 1},
			Allocation:    pterodactyl.Allocation{Default: alloc.ID},
		})
		if err != nil {
			return fmt.Errorf("create server: %w", err)
		}

		paidUntil := time.Now().Add(30 * 24 * time.Hour)
		if err := tx.QueryRow(ctx, `
			INSERT INTO servers (user_id, tariff_id, ptero_server_id, ptero_identifier, name,
			                     status, paid_until, allocation_ip, allocation_port, last_charged_at)
			VALUES ($1,$2,$3,$4,$5,'active',$6,$7,$8,now()) RETURNING id`,
			userID, tariff.ID, created.ID, created.Identifier, srvName,
			paidUntil, alloc.IP, alloc.Port,
		).Scan(&serverID); err != nil {
			return err
		}

		var newBal int64
		if err := tx.QueryRow(ctx, `
			UPDATE users SET balance_cents = balance_cents - $2, updated_at = now()
			WHERE id = $1 RETURNING balance_cents`,
			userID, tariff.PriceCents,
		).Scan(&newBal); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO transactions (user_id, kind, amount_cents, balance_after, description, tariff_id, server_id)
			VALUES ($1,'tariff_purchase',$2,$3,$4,$5,$6)`,
			userID, -tariff.PriceCents, newBal,
			"Purchase: "+tariff.Name, tariff.ID, serverID,
		); err != nil {
			return err
		}
		return nil
	})
	return serverID, err
}

// ProvisionFromGrant implements promo.ServerProvisioner: creates a paid-up
// server without debiting balance. Called inside the promo redemption tx.
func (s *PurchaseService) ProvisionFromGrant(ctx context.Context, tx pgx.Tx, userID, tariffID uuid.UUID, grantDays int, promoID uuid.UUID) (uuid.UUID, error) {
	tariff, err := s.repo.GetTx(ctx, tx, tariffID)
	if err != nil {
		return uuid.Nil, err
	}
	var (
		email       string
		pteroUserID *int
	)
	if err := tx.QueryRow(ctx,
		`SELECT email, ptero_user_id FROM users WHERE id = $1 FOR UPDATE`, userID,
	).Scan(&email, &pteroUserID); err != nil {
		return uuid.Nil, err
	}

	pteroUID, err := s.ensurePteroUser(ctx, tx, userID, email, pteroUserID)
	if err != nil {
		return uuid.Nil, err
	}

	alloc, err := s.pickAllocation(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	srvName := "srv-" + shortID()
	created, err := s.ptero.CreateServer(ctx, pterodactyl.CreateServerInput{
		Name:           srvName,
		UserID:         pteroUID,
		EggID:          tariff.EggID,
		NestID:         tariff.NestID,
		DockerImage:    tariff.DockerImage,
		StartupCommand: tariff.StartupCommand,
		Environment:    tariff.Environment,
		Limits: pterodactyl.Limits{
			Memory: tariff.MemoryMB, Swap: tariff.SwapMB, Disk: tariff.DiskMB,
			IO: orDefault(tariff.IOWeight, 500), CPU: tariff.CPULimit,
		},
		FeatureLimits: pterodactyl.FeatureLimits{Databases: 1, Backups: 1, Allocations: 1},
		Allocation:    pterodactyl.Allocation{Default: alloc.ID},
	})
	if err != nil {
		return uuid.Nil, err
	}

	paidUntil := time.Now().Add(time.Duration(grantDays) * 24 * time.Hour)
	var serverID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO servers (user_id, tariff_id, ptero_server_id, ptero_identifier, name,
		                     status, paid_until, allocation_ip, allocation_port, last_charged_at)
		VALUES ($1,$2,$3,$4,$5,'active',$6,$7,$8,now()) RETURNING id`,
		userID, tariff.ID, created.ID, created.Identifier, srvName,
		paidUntil, alloc.IP, alloc.Port,
	).Scan(&serverID); err != nil {
		return uuid.Nil, err
	}

	var newBal int64
	if err := tx.QueryRow(ctx, `SELECT balance_cents FROM users WHERE id = $1`, userID).Scan(&newBal); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transactions (user_id, kind, amount_cents, balance_after, description, tariff_id, server_id, promo_id)
		VALUES ($1,'promo_tariff_grant',0,$2,$3,$4,$5,$6)`,
		userID, newBal, "Promo grant: "+tariff.Name, tariff.ID, serverID, promoID,
	); err != nil {
		return uuid.Nil, err
	}
	return serverID, nil
}

func (s *PurchaseService) ensurePteroUser(ctx context.Context, tx pgx.Tx, userID uuid.UUID, email string, existing *int) (int, error) {
	if existing != nil && *existing > 0 {
		return *existing, nil
	}
	password, err := randomPassword()
	if err != nil {
		return 0, err
	}
	created, err := s.ptero.CreateUser(ctx, pterodactyl.CreateUserInput{
		Email:     email,
		Username:  usernameFromEmail(email, userID),
		FirstName: "Billing",
		LastName:  "User",
		Password:  password,
	})
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET ptero_user_id = $2 WHERE id = $1`, userID, created.ID); err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (s *PurchaseService) pickAllocation(ctx context.Context) (*pterodactyl.AllocationInfo, error) {
	list, err := s.ptero.ListFreeAllocations(ctx, s.nodeID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errors.New("no free allocations on configured node")
	}
	a := list[0]
	return &a, nil
}

func shortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func usernameFromEmail(email string, id uuid.UUID) string {
	u := ""
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			u += string(r)
		}
		if len(u) >= 8 {
			break
		}
	}
	if len(u) < 3 {
		u = "usr"
	}
	return u + id.String()[:8]
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
