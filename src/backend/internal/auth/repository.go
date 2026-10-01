package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	database *gorm.DB
}

func NewRepository(database *gorm.DB) *Repository {
	return &Repository{database: database}
}

func (repository *Repository) FindAdminByUsername(ctx context.Context, username string) (Admin, error) {
	var admin Admin
	err := repository.database.WithContext(ctx).
		Table("admins").
		Select("id", "username", "password_hash").
		Where("username = ?", username).
		Take(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Admin{}, ErrAdminNotFound
	}
	if err != nil {
		return Admin{}, fmt.Errorf("query administrator: %w", err)
	}
	return admin, nil
}

func (repository *Repository) CreateRefreshToken(ctx context.Context, token RefreshToken) error {
	err := repository.database.WithContext(ctx).Table("refresh_tokens").Create(map[string]any{
		"admin_id":           token.AdminID,
		"token_hash":         token.TokenHash[:],
		"family_id":          token.FamilyID,
		"parent_id":          nil,
		"expires_at":         token.ExpiresAt,
		"session_expires_at": token.SessionExpiresAt,
		"used_at":            nil,
		"revoked_at":         nil,
		"created_at":         token.CreatedAt,
	}).Error
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

type refreshTokenRow struct {
	ID               uint64
	AdminID          uint64
	FamilyID         []byte
	ExpiresAt        time.Time
	SessionExpiresAt time.Time
	UsedAt           *time.Time
	RevokedAt        *time.Time
}

func (repository *Repository) RotateRefreshToken(
	ctx context.Context,
	currentHash [sha256.Size]byte,
	nextHash [sha256.Size]byte,
	now time.Time,
) (RotationResult, error) {
	var result RotationResult
	var replay bool
	err := repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var current refreshTokenRow
		err := transaction.Table("refresh_tokens").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", currentHash[:]).
			Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidSession
		}
		if err != nil {
			return fmt.Errorf("lock refresh token: %w", err)
		}
		if current.RevokedAt != nil {
			return ErrInvalidSession
		}
		if current.UsedAt != nil {
			// 已使用令牌即使自身刚过期，也可能仍有尚未过期的后继令牌；必须撤销整个 family。
			if err := revokeFamily(transaction, current.FamilyID, now); err != nil {
				return err
			}
			replay = true
			return nil
		}
		if !now.Before(current.ExpiresAt) || !now.Before(current.SessionExpiresAt) {
			return ErrInvalidSession
		}

		if err := transaction.Table("refresh_tokens").
			Where("id = ?", current.ID).
			Update("used_at", now).Error; err != nil {
			return fmt.Errorf("mark refresh token used: %w", err)
		}
		expiresAt := now.Add(RefreshTokenLifetime)
		if expiresAt.After(current.SessionExpiresAt) {
			expiresAt = current.SessionExpiresAt
		}
		if err := transaction.Table("refresh_tokens").Create(map[string]any{
			"admin_id":           current.AdminID,
			"token_hash":         nextHash[:],
			"family_id":          current.FamilyID,
			"parent_id":          current.ID,
			"expires_at":         expiresAt,
			"session_expires_at": current.SessionExpiresAt,
			"used_at":            nil,
			"revoked_at":         nil,
			"created_at":         now,
		}).Error; err != nil {
			return fmt.Errorf("insert rotated refresh token: %w", err)
		}
		var admin Admin
		if err := transaction.Table("admins").
			Select("id", "username").
			Where("id = ?", current.AdminID).
			Take(&admin).Error; err != nil {
			return fmt.Errorf("load refresh administrator: %w", err)
		}
		result = RotationResult{Admin: admin, ExpiresAt: expiresAt}
		return nil
	})
	if err != nil {
		return RotationResult{}, err
	}
	if replay {
		return RotationResult{}, ErrRefreshReplay
	}
	return result, nil
}

func (repository *Repository) RevokeRefreshFamily(
	ctx context.Context,
	tokenHash [sha256.Size]byte,
	now time.Time,
) error {
	return repository.database.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		var token refreshTokenRow
		err := transaction.Table("refresh_tokens").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", tokenHash[:]).
			Take(&token).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock refresh token for revocation: %w", err)
		}
		return revokeFamily(transaction, token.FamilyID, now)
	})
}

func revokeFamily(transaction *gorm.DB, familyID []byte, now time.Time) error {
	if err := transaction.Table("refresh_tokens").
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", now).Error; err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	return nil
}
