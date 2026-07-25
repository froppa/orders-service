package db

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/froppa/orders-service/internal/application/ports"
)

func TestMigrateAppliesNewFiles(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer sqlDB.Close()

	database := &DB{db: sqlDB}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM schema_migrations WHERE version = \\$1\\)").
		WithArgs("0001_init.sql").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS orders").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(version\\) VALUES \\(\\$1\\)").
		WithArgs("0001_init.sql").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !IsUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation")
	}
	if IsUniqueViolation(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestOpenInvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := Open(ctx, "postgres://orders:orders@127.0.0.1:1/orders?sslmode=disable"); err == nil {
		t.Fatal("expected open error")
	}
}

func TestWithinTxAndPing(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer sqlDB.Close()

	database := &DB{db: sqlDB}
	mock.ExpectPing()
	if err := database.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if database.DB() == nil {
		t.Fatal("expected db handle")
	}

	mock.ExpectBegin()
	mock.ExpectCommit()
	if err := database.WithinTx(context.Background(), func(context.Context, ports.DBTX) error { return nil }); err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectRollback()
	if err := database.WithinTx(context.Background(), func(context.Context, ports.DBTX) error { return context.Canceled }); err == nil {
		t.Fatal("expected rollback error")
	}

	mock.ExpectClose()
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
