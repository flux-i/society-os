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

	"filippo.io/age"

	"society.local/portal/internal/backup"
	"society.local/portal/internal/database"
	"society.local/portal/internal/documents"
	"society.local/portal/internal/messaging"
	"society.local/portal/internal/security"
	"society.local/portal/internal/server"
)

var version = "0.34.0-dev"

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
		return errors.New("usage: society-server <serve|bootstrap|migrate|seed-demo|inspect|snapshot|restore-check|recover-restore|recover-mfa|recover-origin|recover-nominate|recover-revoke|recover-series-reserve|recover-series-retry|recover-series-reconcile|offsite-keygen|offsite-decrypt|offsite-reconcile|offsite-archive|version> [flags]")
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
	offsiteConfig := flags.String("offsite-config-file", "", "explicit separately held private off-site configuration; destination and public custody recipients remain bound")
	offsiteFixture := flags.Bool("offsite-fixture", false, "explicitly allow numeric-loopback private-storage fixtures only")
	encryptedFile := flags.String("encrypted-file", "", "private encrypted recovery archive")
	expectedChecksum := flags.String("expected-sha256", "", "independently retained encrypted object checksum")
	identitiesFile := flags.String("custody-identities-file", "", "offline private decryption identities; never used by serve")
	backupID := flags.String("backup-id", "", "retained uncertain backup identity for offline reconciliation")
	objectVersion := flags.String("object-version", "", "exact supplied non-null S3 object version")
	resumeArchive := flags.Bool("resume-archive", false, "deliberately verify a completed custody archive and return or record its original disposition")
	userEmail := flags.String("user", "", "account email for offline MFA recovery")
	custodianOne := flags.String("custodian-one", "", "first verified recovery custodian")
	custodianTwo := flags.String("custodian-two", "", "second verified recovery custodian")
	recoveryReason := flags.String("reason", "", "verified offline recovery reason")
	nominationFile := flags.String("nomination-file", "", "private version-1 current recovery nomination JSON")
	reviewerPasswordsFile := flags.String("reviewer-passwords-file", "", "separate private new reviewer passwords JSON")
	revocationFile := flags.String("revocation-file", "", "private version-1 current recovery revocation JSON")
	seriesRequestFile := flags.String("series-request-file", "", "private current-source approved-series reservation JSON")
	seriesRetryFile := flags.String("series-retry-file", "", "private explicit known-rejection series retry JSON")
	seriesConfigFile := flags.String("series-config-file", "", "separately held private series storage/public-recipient configuration")
	seriesFixture := flags.Bool("series-fixture", false, "explicitly allow numeric-loopback series protocol fixtures")
	seriesID := flags.String("series-id", "", "retained unresolved series operation for independent reconciliation")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	seriesCommand := command == "recover-series-reserve" || command == "recover-series-retry" || command == "recover-series-reconcile"
	if seriesCommand {
		allowed := map[string]bool{"db": true, "series-config-file": true, "series-fixture": true}
		switch command {
		case "recover-series-reserve":
			allowed["series-request-file"] = true
		case "recover-series-retry":
			allowed["series-retry-file"] = true
		case "recover-series-reconcile":
			allowed["series-id"], allowed["object-version"], allowed["custody-identities-file"] = true, true, true
		}
		for name := range explicit {
			if !allowed[name] {
				return errors.New("series custody commands refuse serving, unrelated provider and unsupported input flags")
			}
		}
		if !explicit["db"] || *dbPath == "" || *seriesConfigFile == "" ||
			(command == "recover-series-reserve" && *seriesRequestFile == "") ||
			(command == "recover-series-retry" && *seriesRetryFile == "") ||
			(command == "recover-series-reconcile" && (*seriesID == "" || *objectVersion == "" || *identitiesFile == "")) {
			return errors.New("series custody requires an explicit existing --db, separate --series-config-file and the command's private input/identity files")
		}
	} else {
		for _, name := range []string{"series-request-file", "series-retry-file", "series-config-file", "series-fixture", "series-id"} {
			if explicit[name] {
				return errors.New("series reservation inputs are exclusively offline; other commands refuse them")
			}
		}
	}
	if command == "recover-origin" || command == "recover-nominate" || command == "recover-revoke" {
		allowed := map[string]bool{"db": true}
		if command == "recover-nominate" {
			allowed["nomination-file"], allowed["reviewer-passwords-file"] = true, true
		}
		if command == "recover-revoke" {
			allowed["revocation-file"] = true
		}
		for name := range explicit {
			if !allowed[name] {
				return errors.New("recovery review commands refuse serving, provider, custody and unrelated flags")
			}
		}
		if !explicit["db"] || *dbPath == "" || (command == "recover-nominate" && (*nominationFile == "" || *reviewerPasswordsFile == "")) || (command == "recover-revoke" && *revocationFile == "") {
			return errors.New("recovery review requires an explicit existing --db and the command's separate private input files")
		}
	} else if *nominationFile != "" || *reviewerPasswordsFile != "" || *revocationFile != "" {
		return errors.New("recovery nomination inputs are exclusively offline; the server refuses them")
	}
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
	if command == "offsite-keygen" {
		if *output == "" || *offsiteConfig != "" || *identitiesFile != "" {
			return errors.New("offsite-keygen requires a new private --out identity file")
		}
		identity, err := age.GenerateX25519Identity()
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(*output), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("custody identity destination must be new")
		}
		_, writeErr := f.WriteString(identity.String() + "\n")
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			return errors.New("private custody identity could not be retained")
		}
		return printJSON(map[string]string{"public_recipient": identity.Recipient().String()})
	}
	if command == "offsite-decrypt" {
		if *encryptedFile == "" || *expectedChecksum == "" || *identitiesFile == "" || *output == "" {
			return errors.New("offsite-decrypt requires --encrypted-file, --expected-sha256, --custody-identities-file and a new --out bundle")
		}
		identities, err := backup.LoadCustodyIdentities(*identitiesFile)
		if err != nil {
			return err
		}
		info, manifest, err := backup.DecryptArchive(ctx, *encryptedFile, *expectedChecksum, *output, identities)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "decrypted_and_verified", "archive": info, "manifest": manifest})
	}
	if command == "serve" && *identitiesFile != "" {
		return errors.New("the server refuses private decryption custody identities")
	}
	if command == "recover-restore" {
		if *encryptedFile == "" || *expectedChecksum == "" || *identitiesFile == "" || *output == "" || *offsiteConfig != "" || *objectVersion != "" || *offsiteFixture || explicit["db"] || *snapshot != "" || *demo || *workspace {
			return errors.New("offline recovery requires --encrypted-file, --expected-sha256, --custody-identities-file and a fresh --out database; provider and active-workspace flags are refused")
		}
		identities, err := backup.LoadCustodyIdentities(*identitiesFile)
		if err != nil {
			return err
		}
		start := time.Now()
		info, manifest, err := backup.RestoreEncrypted(ctx, *encryptedFile, *expectedChecksum, *output, identities)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "restored_verified_quarantined", "recovery_required": true, "elapsed_ms": float64(time.Since(start).Microseconds()) / 1000, "archive": info, "manifest": manifest})
	}
	if command == "offsite-reconcile" && (!explicit["db"] || *offsiteConfig == "" || *identitiesFile == "" || *backupID == "" || *objectVersion == "" || *output == "" || len(*recoveryReason) < 10 || len(*recoveryReason) > 300) {
		return errors.New("offline reconciliation requires explicit --db, --offsite-config-file, --custody-identities-file, --backup-id, --object-version, --reason and a new --out directory")
	}
	if command == "offsite-archive" && (!explicit["db"] || *identitiesFile == "" || *backupID == "" || *output == "" || (!*resumeArchive && *encryptedFile == "") || len(*recoveryReason) < 10 || len(*recoveryReason) > 300 || *offsiteConfig != "" || *objectVersion != "" || *offsiteFixture) {
		return errors.New("offline archival requires explicit --db, --custody-identities-file, --backup-id, --reason and --out; provide --encrypted-file for a new copy or deliberate --resume-archive")
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
		return printJSON(map[string]any{"status": "restored_verified_quarantined", "recovery_required": true, "elapsed_ms": float64(time.Since(start).Microseconds()) / 1000, "manifest": manifest})
	}
	switch command {
	case "serve", "bootstrap", "migrate", "seed-demo", "inspect", "snapshot", "recover-mfa", "offsite-reconcile", "offsite-archive", "recover-origin", "recover-nominate", "recover-revoke", "recover-series-reserve", "recover-series-retry", "recover-series-reconcile":
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
	if command == "inspect" || command == "snapshot" || command == "recover-mfa" || command == "offsite-reconcile" || command == "offsite-archive" || (command == "serve" && *workspace) {
		if _, err := os.Stat(*dbPath); err != nil {
			return err
		}
	}
	var nomination database.RecoveryNomination
	var reviewerPasswords database.RecoveryPasswords
	var revocation database.RecoveryRevocation
	var seriesRequest database.RecoverySeriesRequest
	var seriesRetry database.RecoverySeriesRetry
	if seriesCommand {
		if info, err := os.Lstat(*dbPath); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("series custody requires an existing regular private database")
		}
		if command == "recover-series-reserve" {
			body, err := readPrivateFile(*seriesRequestFile, 8192)
			if err != nil {
				return err
			}
			seriesRequest, err = database.ParseRecoverySeriesRequest(body)
			if err != nil {
				return err
			}
		}
		if command == "recover-series-retry" {
			body, err := readPrivateFile(*seriesRetryFile, 8192)
			if err != nil {
				return err
			}
			seriesRetry, err = database.ParseRecoverySeriesRetry(body)
			if err != nil {
				return err
			}
		}
	}
	if command == "recover-origin" || command == "recover-nominate" || command == "recover-revoke" {
		if info, err := os.Lstat(*dbPath); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("recovery review requires an existing regular private database")
		}
		if command == "recover-nominate" {
			body, err := readPrivateFile(*nominationFile, 32768)
			if err != nil {
				return err
			}
			nomination, err = database.ParseRecoveryNomination(body)
			if err != nil {
				return err
			}
			body, err = readPrivateFile(*reviewerPasswordsFile, 8192)
			if err != nil {
				return err
			}
			reviewerPasswords, err = database.ParseRecoveryPasswords(body)
			if err != nil {
				return err
			}
		}
		if command == "recover-revoke" {
			body, err := readPrivateFile(*revocationFile, 8192)
			if err != nil {
				return err
			}
			revocation, err = database.ParseRecoveryRevocation(body)
			if err != nil {
				return err
			}
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
	case "recover-series-reserve", "recover-series-retry", "recover-series-reconcile":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		if err := store.RequireRecoverableWorkspace(ctx); err != nil {
			return err
		}
		var result database.RecoverySeriesIntent
		var err error
		switch command {
		case "recover-series-reserve":
			result, err = backup.ReserveRecoverySeries(ctx, store, *seriesConfigFile, *seriesFixture, seriesRequest)
		case "recover-series-retry":
			result, err = backup.RetryRecoverySeries(ctx, store, *seriesConfigFile, *seriesFixture, seriesRetry)
		case "recover-series-reconcile":
			identities, e := backup.LoadCustodyIdentities(*identitiesFile)
			if e != nil {
				return e
			}
			result, err = backup.ReconcileRecoverySeries(ctx, store, *seriesConfigFile, *seriesFixture, *seriesID, *objectVersion, identities)
		}
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"operation_id": result.OperationID, "state": result.State, "attempts": result.Attempts, "prepared_only": true, "can_resume": false})
	case "recover-origin":
		origin, err := store.RecoveryOriginForCustody(ctx)
		if err != nil {
			return err
		}
		return printJSON(origin)
	case "recover-nominate":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		result, err := store.NominateRecovery(ctx, nomination, reviewerPasswords)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "recover-revoke":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		if err := store.RevokeRecoveryNomination(ctx, revocation); err != nil {
			return err
		}
		return printJSON(map[string]string{"status": "recovery_nomination_revoked", "operation_id": revocation.OperationID})
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
	case "offsite-reconcile":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		_, reader, _, hash, err := backup.LoadOffsiteConfig(*offsiteConfig, *offsiteFixture)
		if err != nil {
			return err
		}
		j, err := store.OffsiteJobForCustody(ctx, *backupID)
		if err != nil || j.ConfigHash != hash {
			return errors.New("custody retrieval configuration differs from the retained destination binding")
		}
		identities, err := backup.LoadCustodyIdentities(*identitiesFile)
		if err != nil {
			return err
		}
		info, manifest, err := backup.ReconcileOffsite(ctx, store, reader, *backupID, *objectVersion, *output, *recoveryReason, identities)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "exact_version_decrypted_verified_and_reconciled", "archive": info, "manifest": manifest})
	case "offsite-archive":
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		identities, err := backup.LoadCustodyIdentities(*identitiesFile)
		if err != nil {
			return err
		}
		disposition, err := backup.ArchiveOffsiteCandidate(ctx, store, *backupID, *encryptedFile, *output, *recoveryReason, identities, *resumeArchive)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"status": "independently_archived_and_disposed", "disposition": disposition})
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
		var offsiteWorker *backup.OffsiteWorker
		quarantined, err := store.RecoveryRequired(ctx)
		if err != nil {
			return err
		}
		if quarantined {
			logger.Info("recovery_quarantined", "external_handoffs", "withheld")
		} else if *offsiteConfig != "" {
			config, client, recipients, hash, err := backup.LoadOffsiteConfig(*offsiteConfig, *offsiteFixture)
			if err != nil {
				return err
			}
			if err = store.BindOffsitePolicy(ctx, hash, config.Storage.Mode, time.Now()); err != nil {
				return err
			}
			offsiteWorker = &backup.OffsiteWorker{Store: store, Client: client, Recipients: recipients, ConfigHash: hash, Prefix: config.Storage.Prefix, Directory: backup.DefaultOffsitePath(*dbPath), Version: version}
		} else {
			bound, err := store.OffsiteConfigured(ctx)
			if err != nil {
				return err
			}
			if bound {
				return errors.New("off-site protection is already bound; restart requires its matching private configuration")
			}
		}
		documentStore, err := documents.Open(documents.DefaultPath(*dbPath))
		if err != nil {
			return err
		}
		defer documentStore.Close()
		workerCtx, stopWorker := context.WithCancel(ctx)
		defer stopWorker()
		if !quarantined {
			if err = documentStore.Reconcile(ctx, store); err != nil {
				return err
			}
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
		}
		if offsiteWorker != nil {
			offsiteDone := make(chan struct{})
			go func() { defer close(offsiteDone); offsiteWorker.Run(workerCtx, logger) }()
			defer func() { stopWorker(); <-offsiteDone }()
		}
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
