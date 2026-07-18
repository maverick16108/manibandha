// Package migrate — простой форвард-раннер SQL-миграций поверх embed.FS.
// Заменяет Alembic: одна baseline-миграция (полная схема) + последующие numbered .up.sql.
//
// Существующую БД (её вёл Alembic) адаптируем: если схема уже есть (таблица users),
// а нашей schema_migrations ещё нет — считаем baseline (1) применённым, не пере-создаём.
//
// Миграции выполняются на ОТДЕЛЬНОМ одиночном соединении, которое закрывается по
// завершении, — чтобы SET search_path из pg_dump не «протёк» в рабочий пул приложения.
package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // драйвер "pgx" для database/sql
)

//go:embed sql/*.up.sql
var files embed.FS

// normalizeDSN приводит SQLAlchemy-DSN к libpq-виду (как database.Connect).
func normalizeDSN(dsn string) string {
	dsn = strings.Replace(dsn, "postgresql+psycopg2://", "postgres://", 1)
	if strings.HasPrefix(dsn, "postgresql://") {
		dsn = strings.Replace(dsn, "postgresql://", "postgres://", 1)
	}
	return dsn
}

// Run применяет все непринятые миграции. Идемпотентно.
func Run(dsn string) error {
	db, err := sql.Open("pgx", normalizeDSN(dsn))
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // одно соединение; закроется вместе с пулом
	db.SetConnMaxLifetime(time.Minute)

	// схему указываем явно (public.*): миграции из pg_dump сбрасывают search_path
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS public.schema_migrations (
		version bigint PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	applied := map[int64]bool{}
	rows, err := db.Query(`SELECT version FROM public.schema_migrations`)
	if err != nil {
		return fmt.Errorf("migrate: read versions: %w", err)
	}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	// адаптация существующей (Alembic) БД: схема есть, но наша учётная таблица пуста
	if len(applied) == 0 {
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass('public.users') IS NOT NULL`).Scan(&exists); err != nil {
			return fmt.Errorf("migrate: probe users: %w", err)
		}
		if exists {
			if _, err := db.Exec(`INSERT INTO public.schema_migrations(version) VALUES (1)`); err != nil {
				return fmt.Errorf("migrate: adopt baseline: %w", err)
			}
			applied[1] = true
			log.Printf("[migrate] существующая БД адаптирована: baseline (1) отмечен применённым")
		}
	}

	entries, err := files.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("migrate: read embed: %w", err)
	}
	type mig struct {
		ver  int64
		name string
	}
	var migs []mig
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".up.sql") {
			continue
		}
		ver, err := strconv.ParseInt(strings.SplitN(n, "_", 2)[0], 10, 64)
		if err != nil {
			return fmt.Errorf("migrate: bad name %q: %w", n, err)
		}
		migs = append(migs, mig{ver, n})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].ver < migs[j].ver })

	for _, mg := range migs {
		if applied[mg.ver] {
			continue
		}
		body, err := files.ReadFile("sql/" + mg.name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: apply %s: %w", mg.name, err)
		}
		// тело миграции (pg_dump) могло сбросить search_path — возвращаем в норму
		if _, err := tx.Exec(`SET search_path TO public`); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`INSERT INTO public.schema_migrations(version) VALUES ($1)`, mg.ver); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("[migrate] применена %s", mg.name)
	}
	return nil
}
