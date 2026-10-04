package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const FixtureVersion = "housing_demo_v1"

func (s *Store) SeedDemo(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var fixture string
	err = tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'fixture_version'").Scan(&fixture)
	if err == nil {
		var flats int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM flats").Scan(&flats); err != nil {
			return err
		}
		if fixture != FixtureVersion || flats != 118 {
			return errors.New("existing fixture differs; create a separate demo database")
		}
		var kind string
		if err := tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key = 'data_kind'").Scan(&kind); err != nil || kind != "synthetic" {
			return errors.New("existing data is not marked synthetic")
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var occupied int
	if err := tx.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM app_metadata) + (SELECT COUNT(*) FROM buildings) +
		(SELECT COUNT(*) FROM flats) + (SELECT COUNT(*) FROM residents) +
		(SELECT COUNT(*) FROM flat_memberships)`).Scan(&occupied); err != nil {
		return err
	}
	if occupied != 0 {
		return errors.New("refusing to mix synthetic fixtures with existing data")
	}
	for _, code := range []string{"A", "B", "C"} {
		buildingID := "demo-building-" + code
		if _, err := tx.ExecContext(ctx, "INSERT INTO buildings VALUES (?, ?, ?)", buildingID, code, "Wing "+code); err != nil {
			return err
		}
		count := 40
		if code == "C" {
			count = 38
		}
		for i := 1; i <= count; i++ {
			floor, number := (i-1)/4+1, fmt.Sprintf("%d%02d", (i-1)/4+1, (i-1)%4+1)
			flatID := fmt.Sprintf("demo-flat-%s-%s", code, number)
			status := "OWNER_OCCUPIED"
			if i%11 == 0 {
				status = "VACANT"
			} else if i%3 == 0 {
				status = "RENTED"
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO flats(id, building_id, flat_number, floor, status) VALUES (?, ?, ?, ?, ?)", flatID, buildingID, number, floor, status); err != nil {
				return err
			}
			ownerID := "demo-owner-" + code + "-" + number
			// One fictional owner holds two flats to exercise registry relationships.
			if code == "A" && i == 2 {
				ownerID = "demo-owner-A-101"
			} else {
				if _, err := tx.ExecContext(ctx, "INSERT INTO residents VALUES (?, ?)", ownerID, "Demo Owner "+code+"-"+number); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, "INSERT INTO flat_memberships VALUES (?, ?, ?, 'OWNER', '2020-01-01', NULL, 1, 1)", "demo-membership-owner-"+flatID, flatID, ownerID); err != nil {
				return err
			}
			if status == "RENTED" {
				tenantID := "demo-tenant-" + code + "-" + number
				if _, err := tx.ExecContext(ctx, "INSERT INTO residents VALUES (?, ?)", tenantID, "Demo Tenant "+code+"-"+number); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO flat_memberships VALUES (?, ?, ?, 'TENANT', '2020-01-01', NULL, 0, 0)", "demo-membership-tenant-"+flatID, flatID, tenantID); err != nil {
					return err
				}
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO residents VALUES ('demo-joint-owner', 'Demo Joint Owner A-101')"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO flat_memberships VALUES ('demo-joint-membership', 'demo-flat-A-101', 'demo-joint-owner', 'OWNER', '2020-01-01', NULL, 0, 1)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO residents VALUES ('demo-former-tenant', 'Demo Former Tenant A-103')"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO flat_memberships VALUES ('demo-former-membership', 'demo-flat-A-103', 'demo-former-tenant', 'TENANT', '2019-01-01', '2020-01-01', 0, 0)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO app_metadata VALUES ('data_kind', 'synthetic'), ('fixture_version', ?)", FixtureVersion); err != nil {
		return err
	}
	return tx.Commit()
}
