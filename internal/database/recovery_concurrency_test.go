package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestBackgroundRecoveryChecksWaitForConcurrentPortalWriterThenRemainUsable(t *testing.T) {
	for _, kind := range []string{"receipt", "document", "statement", "offsite"} {
		t.Run(kind, func(t *testing.T) {
			s := fixture(t)
			ctx := context.Background()
			// Real serving already has multiple warmed connections. Opening a
			// new connection under a writer can hide the later claim race behind
			// connection-initialisation locking, so prepare both first.
			first, err := s.DB.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.DB.Conn(ctx)
			if err != nil {
				first.Close()
				t.Fatal(err)
			}
			first.Close()
			second.Close()
			if kind == "offsite" {
				bindOffsite(t, s, time.Now())
			}
			writer, err := s.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			if _, err := writer.ExecContext(ctx, "UPDATE app_metadata SET value=value WHERE key='fixture_version'"); err != nil {
				t.Fatal(err)
			}
			started, finished := make(chan struct{}), make(chan error, 1)
			go func() {
				close(started)
				var err error
				switch kind {
				case "receipt":
					_, err = s.ClaimReceipt(ctx, time.Now())
				case "document":
					_, err = s.ClaimDocumentCheck(ctx, time.Now())
				case "statement":
					_, err = s.ClaimStatementCheck(ctx, time.Now())
				case "offsite":
					_, err = s.ClaimOffsite(ctx, testOffsiteBinding, "concurrent-portal-write", time.Now())
				}
				finished <- err
			}()
			<-started
			select {
			case err := <-finished:
				t.Fatal("A background claim failed before the bounded portal write released its reservation", err)
			case <-time.After(100 * time.Millisecond):
			}
			if err := writer.Commit(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if kind == "offsite" {
					if err != nil {
						t.Fatal("A scheduled capture could not proceed after the portal write", err)
					}
				} else if !errors.Is(err, sql.ErrNoRows) {
					t.Fatal("An ordinary idle claim became a database failure after the portal write", err)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("A background claim remained blocked after the portal write ended")
			}
		})
	}
}
