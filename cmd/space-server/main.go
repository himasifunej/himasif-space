// Command space-server runs Space's API server.
//
//	space-server [-env-file FILE] serve      # migrate, then serve the API (and the web app)
//	space-server [-env-file FILE] migrate    # apply the database migrations
//	space-server [-env-file FILE] seed       # add the first period, divisions and admin account
//	space-server [-env-file FILE] mail-test -to ADDRESS
//
// Configuration comes from the environment (see .env.example); -env-file
// reads a file of KEY=VALUE lines first, for development.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/himasifunej/himasif-space/internal/config"
	"github.com/himasifunej/himasif-space/internal/mail"
	"github.com/himasifunej/himasif-space/internal/seed"
	"github.com/himasifunej/himasif-space/internal/server"
	"github.com/himasifunej/himasif-space/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "space-server:", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env-file", "", "read KEY=VALUE lines from `file` before the environment's own")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: space-server [-env-file FILE] version|serve|migrate|seed|activate-restore|quarantine-restore -database-name NAME -reason TEXT|mail-test -to ADDRESS")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 1 && flag.Arg(0) == "version" {
		return json.NewEncoder(os.Stdout).Encode(server.BuildInfo())
	}
	if *envFile != "" {
		if err := config.LoadEnvFile(*envFile); err != nil {
			return err
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.Development() {
		log = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, args := "serve", []string(nil)
	if flag.NArg() > 0 {
		cmd, args = flag.Arg(0), flag.Args()[1:]
	}
	if cmd == "mail-test" {
		return mailTest(ctx, cfg, log, args)
	}

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	switch cmd {
	case "activate-restore":
		return activateRestore(ctx, cfg, st, args)
	case "quarantine-restore":
		return quarantineRestore(ctx, cfg, st, log, args)
	case "migrate":
		return migrate(ctx, st, log)
	case "seed":
		if err := migrate(ctx, st, log); err != nil {
			return err
		}
		rep, err := seed.Run(ctx, st, seed.Options{AdminEmail: cfg.AdminEmail, AdminPassword: cfg.AdminPassword})
		if err != nil {
			return err
		}
		log.Info("seeded", "period_added", rep.Period, "divisions_added", rep.Divisions, "spaces_added", rep.Spaces, "admin_added", rep.Admin)
		return nil
	case "serve":
		if err := migrate(ctx, st, log); err != nil {
			return err
		}
		return st.WithRuntimeLease(ctx, cfg.RunMode, func(leased context.Context) error { return serve(leased, cfg, st, log) })
	}
	flag.Usage()
	return fmt.Errorf("unknown command %q", cmd)
}

func migrate(ctx context.Context, st *store.Store, log *slog.Logger) error {
	applied, err := st.Migrate(ctx)
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		log.Info("migrations applied", "names", applied)
	}
	return nil
}

func serve(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger) error {
	mailer, err := mail.New(cfg.Mail, log)
	if err != nil {
		return err
	}
	api := server.New(cfg, st, mailer, log)
	schedulerCtx, stopScheduler := context.WithCancel(ctx)
	monitorCtx, stopMonitor := context.WithCancel(context.WithoutCancel(ctx))
	monitorDone := make(chan struct{})
	go func() { defer close(monitorDone); api.RunOperationsMonitor(monitorCtx) }()
	realtimeDone := make(chan struct{})
	go func() { defer close(realtimeDone); api.RunRealtime(schedulerCtx) }()
	schedulerDone := make(chan struct{})
	go func() { defer close(schedulerDone); api.RunScheduler(schedulerCtx) }()
	connectorsDone := make(chan struct{})
	go func() { defer close(connectorsDone); api.RunConnectors(schedulerCtx) }()
	whatsappDone := make(chan struct{})
	go func() { defer close(whatsappDone); api.RunWhatsApp(schedulerCtx) }()
	sopDone := make(chan struct{})
	go func() { defer close(sopDone); api.RunSOP(schedulerCtx) }()
	defer func() {
		stopScheduler()
		<-schedulerDone
		<-connectorsDone
		<-whatsappDone
		<-sopDone
		<-realtimeDone
		stopMonitor()
		<-monitorDone
	}()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "run_mode", cfg.RunMode, "web_dir", cfg.WebDir, "mail", cfg.Mail.Driver)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-schedulerDone
	// Recovery emails still being sent finish before the store closes.
	api.Wait()
	return nil
}

func mailTest(ctx context.Context, cfg config.Config, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("mail-test", flag.ContinueOnError)
	to := fs.String("to", "", "send the test email to `address`")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("mail-test: -to is required")
	}
	m, err := mail.New(cfg.Mail, log)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := m.Send(ctx, mail.Message{
		To:      *to,
		Subject: "Uji email Space — Ruang Kerja HIMASIF",
		Text:    "Halo,\n\nIni email uji dari server Space. Jika kamu menerimanya, pengiriman email sudah berfungsi.\n\n— Space",
	}); err != nil {
		return err
	}
	log.Info("test email sent", "to", *to, "driver", cfg.Mail.Driver)
	return nil
}

func quarantineRestore(ctx context.Context, cfg config.Config, st *store.Store, log *slog.Logger, args []string) error {
	flags := flag.NewFlagSet("quarantine-restore", flag.ContinueOnError)
	name := flags.String("database-name", "", "confirm the freshly restored space_restore_ / space_shadow_ database name")
	reason := flags.String("reason", "", "reason for this isolated restore")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if cfg.RunMode != config.RunIsolatedRestore {
		return errors.New("quarantine-restore requires SPACE_RUN_MODE=isolated_restore")
	}
	if *reason == "" || len(*reason) > 4000 {
		return errors.New("quarantine-restore requires -reason (1 to 4000 bytes)")
	}
	if err := st.RequireRestoreDatabase(ctx, *name); err != nil {
		return err
	}
	if err := migrate(ctx, st, log); err != nil {
		return err
	}
	control, err := st.QuarantineRestore(ctx, *name, *reason)
	if err != nil {
		return err
	}
	log.Info("restored database quarantined", "restore_id", control.RestoreID, "mode", control.Mode)
	return nil
}

// Schema preparation is separate: even the preview never runs migrations.
func activateRestore(ctx context.Context, cfg config.Config, st *store.Store, args []string) error {
	fs := flag.NewFlagSet("activate-restore", flag.ContinueOnError)
	name := fs.String("database-name", "", "confirm restored database name")
	restoreID := fs.String("restore-id", "", "confirm the restore ID from quarantine")
	operator := fs.String("operator", "", "operator attribution")
	reason := fs.String("reason", "", "review/cutover reference and reason")
	cutoffText := fs.String("cutoff", "", "RFC3339 cutoff within the last 15 minutes")
	applyHash := fs.String("apply-plan", "", "apply exactly this preview SHA256; absent means read-only preview")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected activation arguments")
	}
	if cfg.RunMode != config.RunMaintenance {
		return errors.New("activate-restore requires SPACE_RUN_MODE=maintenance and all servers stopped")
	}
	cutoff, err := time.Parse(time.RFC3339Nano, *cutoffText)
	if err != nil {
		return fmt.Errorf("invalid cutoff: %w", err)
	}
	plan, err := st.ActivateRestore(ctx, *name, *restoreID, *operator, *reason, cutoff, *applyHash)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}
