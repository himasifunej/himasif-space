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
		fmt.Fprintln(os.Stderr, "usage: space-server [-env-file FILE] serve|migrate|seed|mail-test -to ADDRESS")
		flag.PrintDefaults()
	}
	flag.Parse()
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
		return serve(ctx, cfg, st, log)
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
		log.Info("listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "web_dir", cfg.WebDir, "mail", cfg.Mail.Driver)
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
