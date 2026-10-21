package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/paramah/denar/internal/adapter/sqldb"
	"github.com/paramah/denar/internal/platform/config"
)

const dbUsage = `usage: denar db copy --from <database> [--to <database>]

Copies every row of one database into another, empty one, for example from
the SQLite file to PostgreSQL:

  denar db copy --from /data/denar.db --to postgres://denar@db/denar

A database is a SQLite file or a postgres:// URL; --to defaults to DENAR_DB.
The source must be at the schema version of this binary (run denar migrate
on it first); the target is migrated. Stop the server while copying: rows
written meanwhile are not copied.`

// dbCommand runs "denar db copy" (014-postgres). It writes the target in one
// transaction, so a failed copy leaves it empty, and prints the row count of
// every table on both sides.
func dbCommand(ctx context.Context, cfg config.Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "copy" {
		return errors.New(dbUsage)
	}
	fs := flag.NewFlagSet("db copy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, dbUsage) }
	from := fs.String("from", "", "source database")
	to := fs.String("to", cfg.Database, "target database")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *from == "" || *to == "" || fs.NArg() > 0 {
		return errors.New(dbUsage)
	}
	if *from == *to {
		return errors.New("--from and --to name the same database")
	}

	src, err := sqldb.Open(*from)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	defer src.Close()
	dst, err := sqldb.Open(*to)
	if err != nil {
		return fmt.Errorf("target: %w", err)
	}
	defer dst.Close()
	if err := sqldb.Migrate(ctx, dst, dst.Migrations()); err != nil {
		return fmt.Errorf("target: %w", err)
	}

	counts, err := sqldb.Copy(ctx, src, dst)
	if counts != nil {
		w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', tabwriter.AlignRight)
		fmt.Fprintln(w, "table\tsource\ttarget\t")
		for _, c := range counts {
			fmt.Fprintf(w, "%s\t%d\t%d\t\n", c.Table, c.Source, c.Target)
		}
		_ = w.Flush()
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "copied %s to %s\n", config.RedactURL(*from), config.RedactURL(*to))
	return nil
}
