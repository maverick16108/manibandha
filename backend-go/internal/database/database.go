package database

import (
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect открывает пул к тому же PostgreSQL, что и Python-версия.
// SQLAlchemy DSN (postgresql+psycopg2://) приводим к libpq-виду (postgres://).
func Connect(dsn string) (*gorm.DB, error) {
	dsn = strings.Replace(dsn, "postgresql+psycopg2://", "postgres://", 1)
	if strings.HasPrefix(dsn, "postgresql://") {
		dsn = strings.Replace(dsn, "postgresql://", "postgres://", 1)
	}
	// Примечание: timestamptz сериализуется в UTC на уровне Go (models.Time / tsUTC),
	// т.к. session timezone через DSN здесь не применяется надёжно.
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true,
	})
}
