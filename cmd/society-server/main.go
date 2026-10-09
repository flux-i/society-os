package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"society.local/portal/internal/backup"
	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
	"society.local/portal/internal/server"
)

var version = "0.29.0-dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], logger); err != nil {
		logger.Error("command_failed", "error", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, logger *slog.Logger) error {
	if len(args) == 0 {
		return errors.New("usage: society-server <serve|bootstrap|migrate|seed-demo|inspect|snapshot|restore-check|recover-mfa|version> [flags]")
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	dbPath := flags.String("db", "var/demo/society.db", "local SQLite path")
	demo := flags.Bool("demo", false, "explicitly allow a synthetic local preview")
	workspace := flags.Bool("workspace", false, "serve an explicitly bootstrapped local workspace; no demo accounts")
	setupFile := flags.String("setup-file", "", "private version-1 workspace setup JSON")
	passwordFile := flags.String("password-file", "", "private initial administrator password file; never a command-line password")
	address := flags.String("addr", "127.0.0.1:8080", "loopback listen address")
	webDir := flags.String("web-dir", "build/web", "built frontend directory")
	output := flags.String("out", "", "new snapshot bundle or restored database path")
	snapshot := flags.String("snapshot", "", "snapshot bundle to restore")
	keyPath := flags.String("mfa-key-file", "var/keys/mfa.key", "private MFA encryption key held separately from snapshots")
	messageKeyPath := flags.String("message-key-file", "", "separate private simulation signing key; defaults to keys/messages.key beside the database")
	whatsappFixturePath := flags.String("whatsapp-fixture-config", "", "separately held private official-protocol loopback fixture configuration; no external sends")
	userEmail := flags.String("user", "", "account email for offline MFA recovery")
	custodianOne := flags.String("custodian-one", "", "first verified recovery custodian")
	custodianTwo := flags.String("custodian-two", "", "second verified recovery custodian")
	recoveryReason := flags.String("reason", "", "verified offline recovery reason")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if *demo && *workspace {
		return errors.New("choose --demo or --workspace")
	}
	if command == "bootstrap" || *workspace {
		for _, name := range []string{"db", "mfa-key-file", "message-key-file"} {
			if !explicit[name] {
				return fmt.Errorf("workspace operations require an explicit --%s", name)
			}
		}
		if *dbPath == "" || *keyPath == "" || *messageKeyPath == "" {
			return errors.New("workspace database and private key paths must not be empty")
		}
		paths := map[string]bool{}
		for _, path := range []string{*dbPath, *keyPath, *messageKeyPath} {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if paths[abs] {
				return errors.New("workspace database and key paths must be separate")
			}
			paths[abs] = true
		}
	}
	var setup database.WorkspaceSetup
	var initialPassword string
	if command == "bootstrap" {
		if *demo || *setupFile == "" || *passwordFile == "" {
			return errors.New("bootstrap requires --setup-file and --password-file, and refuses --demo")
		}
		body, err := readPrivateFile(*setupFile, 16384)
		if err != nil {
			return err
		}
		setup, err = database.ParseWorkspaceSetup(body)
		if err != nil {
			return err
		}
		secret, err := readPrivateFile(*passwordFile, 258)
		if err != nil {
			return err
		}
		initialPassword = string(secret)
		initialPassword = strings.TrimSuffix(initialPassword, "\n")
		initialPassword = strings.TrimSuffix(initialPassword, "\r")
	}
	if command == "version" {
		return printJSON(map[string]string{"version": version})
	}
	if command == "restore-check" {
		if *snapshot == "" || *output == "" {
			return errors.New("restore-check requires --snapshot and --out")
		}
		start := time.Now()
		manifest, err := backup.Restore(ctx, *snapshot, *output)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "restored_and_verified", "elapsed_ms": float64(time.Since(start).Microseconds()) / 1000, "manifest": manifest})
	}
	switch command {
	case "serve", "bootstrap", "migrate", "seed-demo", "inspect", "snapshot", "recover-mfa":
	default:
		return errors.New("unknown command")
	}
	if (command == "serve" || command == "seed-demo" || command == "recover-mfa") && !*demo && !*workspace {
		return errors.New("an explicit --demo or configured --workspace is required; public production serving is not enabled")
	}
	if command == "seed-demo" && *workspace {
		return errors.New("demo seeding is unavailable in a workspace")
	}
	if command == "serve" {
		if err := server.ValidateAddress(*address); err != nil {
			return err
		}
		if _, err := os.Stat(*webDir + "/index.html"); err != nil {
			return errors.New("frontend is not built; run make build first")
		}
	}
	// Inspect/snapshot must not silently create a missing source database.
	if command == "inspect" || command == "snapshot" || command == "recover-mfa" || (command == "serve" && *workspace) {
		if _, err := os.Stat(*dbPath); err != nil {
			return err
		}
	}
	store, err := database.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if command == "bootstrap" {
		if err = store.CheckBootstrapTarget(ctx); err != nil {
			return err
		}
	}
	if command == "migrate" || command == "bootstrap" || command == "seed-demo" || command == "serve" {
		if err := store.Migrate(ctx); err != nil {
			return err
		}
	}
	switch command {
	case "bootstrap":
		// A committed setup binds both keys. Never create a missing replacement on
		// retry, even before the first administrator has enrolled an authenticator.
		var configured bool
		if err = store.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM workspace_setup)").Scan(&configured); err != nil {
			return err
		}
		box, err := security.LoadKey(*keyPath, !configured)
		if err != nil {
			return err
		}
		messageKey, err := messaging.LoadKey(*messageKeyPath, !configured)
		if err != nil {
			return err
		}
		info, err := store.BootstrapWorkspace(ctx, setup, initialPassword, box.Fingerprint(), database.InputDigest(messageKey))
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "workspace_configured", "workspace": info, "next": "Sign in with the supplied administrator credentials and complete authenticator enrollment. Registry access does not grant finance access."})
	case "recover-mfa":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		if err := store.OfflineMFARecovery(ctx, *userEmail, *custodianOne, *custodianTwo, *recoveryReason); err != nil {
			return err
		}
		return printJSON(map[string]string{"status": "MFA enrollment required; sessions and account links revoked"})
	case "migrate":
		return printJSON(map[string]any{"schema_version": database.SchemaVersion, "status": "current"})
	case "seed-demo":
		if err := store.SeedDemo(ctx); err != nil {
			return err
		}
		if err := store.SeedDemoAccounts(ctx); err != nil {
			return err
		}
		if err := store.SeedDemoTreasury(ctx); err != nil {
			return err
		}
		counts, err := database.CountRecords(ctx, store.DB)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"fixture": database.FixtureVersion, "counts": counts})
	case "inspect":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		engine, err := store.Engine(ctx)
		if err != nil {
			return err
		}
		counts, err := database.CountRecords(ctx, store.DB)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"application_version": version, "schema_version": database.SchemaVersion, "engine": engine, "counts": counts})
	case "snapshot":
		if *output == "" {
			return errors.New("snapshot requires a new --out bundle directory")
		}
		manifest, err := backup.Snapshot(ctx, store, *output, version)
		if err != nil {
			return err
		}
		return printJSON(manifest)
	case "serve":
		if *workspace {
			if err = store.RequireWorkspace(ctx); err != nil {
				return err
			}
		} else {
			if err := store.RequireDemo(ctx); err != nil {
				return err
			}
			if err := store.SeedDemoAccounts(ctx); err != nil {
				return err
			}
			if err := store.SeedDemoTreasury(ctx); err != nil {
				return err
			}
		}
		var factors int
		if err := store.DB.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM mfa_factors)+(SELECT COUNT(*) FROM mfa_pending)").Scan(&factors); err != nil {
			return err
		}
		box, err := security.LoadKey(*keyPath, factors == 0 && !*workspace)
		if err != nil {
			return err
		}
		store.MFA = box
		if err := store.VerifyMFAKey(ctx); err != nil {
			return err
		}
		var messages, bound int
		if err = store.DB.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM simulation_messages),(SELECT COUNT(*) FROM app_metadata WHERE key='message_key_fingerprint')").Scan(&messages, &bound); err != nil {
			return err
		}
		if *messageKeyPath == "" {
			*messageKeyPath = messaging.DefaultKeyPath(*dbPath)
		}
		signingKey, err := messaging.LoadKey(*messageKeyPath, messages == 0 && bound == 0 && !*workspace)
		if err != nil {
			return err
		}
		if *workspace {
			if err = store.VerifyWorkspaceKeys(ctx, box.Fingerprint(), database.InputDigest(signingKey)); err != nil {
				return err
			}
		}
		messageEngine, err := messaging.New(store, signingKey)
		if err != nil {
			return err
		}
		if err = messageEngine.VerifyKey(ctx, messages == 0 && bound == 0); err != nil {
			return err
		}
		if *whatsappFixturePath != "" {
			config, err := messaging.LoadWhatsAppConfig(*whatsappFixturePath)
			if err != nil {
				return err
			}
			client, err := messaging.NewWhatsAppClient(config, true)
			if err != nil {
				return err
			}
			if err = messageEngine.WithWhatsAppFixture(client); err != nil {
				return err
			}
			if err = messageEngine.VerifyWhatsAppKey(ctx); err != nil {
				return err
			}
		}
		if err = store.RecoverMessageClaims(ctx); err != nil {
			return err
		}
		documentStore, err := documents.Open(documents.DefaultPath(*dbPath))
		if err != nil {
			return err
		}
		defer documentStore.Close()
		if err = documentStore.Reconcile(ctx, store); err != nil {
			return err
		}
		workerCtx, stopWorker := context.WithCancel(ctx)
		workerDone := make(chan struct{})
		go func() { defer close(workerDone); documentStore.Run(workerCtx, store, logger) }()
		validationDone := make(chan struct{})
		go func() { defer close(validationDone); documents.RunValidation(workerCtx, store, logger) }()
		statementValidationDone := make(chan struct{})
		go func() {
			defer close(statementValidationDone)
			documents.RunStatementValidation(workerCtx, store, logger)
		}()
		defer func() { stopWorker(); <-workerDone; <-validationDone; <-statementValidationDone }()
		app := &server.Server{Documents: documentStore, Messages: messageEngine, Store: store, Logger: logger, Version: version, Web: os.DirFS(*webDir)}
		httpServer := &http.Server{Addr: *address, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		errCh := make(chan error, 1)
		go func() { errCh <- httpServer.ListenAndServe() }()
		info, err := store.WorkspaceInfo(ctx)
		if err != nil {
			return err
		}
		logger.Info("local_workspace_started", "address", *address, "version", version, "schema_version", database.SchemaVersion, "mode", info.Mode)
		select {
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return httpServer.Shutdown(shutdownCtx)
		}
	}
	return nil
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func readPrivateFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("the specified private input file is unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > limit {
		return nil, errors.New("input must be a bounded regular private file (mode 0600)")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("private input changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("private input exceeds its size limit")
	}
	return body, nil
}
