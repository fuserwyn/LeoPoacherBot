package database

import (
	"fmt"
)

// Migration представляет миграцию базы данных
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

// Migrations содержит все миграции в порядке версий
var Migrations = []Migration{
	{
		Version:     1,
		Description: "Update timestamp fields to use Moscow timezone",
		UpSQL: `
			-- Обновляем поля created_at и updated_at в message_log
			ALTER TABLE message_log 
			ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
			ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE;
			
			-- Обновляем поля created_at и updated_at в training_log
			ALTER TABLE training_log 
			ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE,
			ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE;
			
			-- Устанавливаем значения по умолчанию для новых записей
			ALTER TABLE message_log 
			ALTER COLUMN created_at SET DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow'),
			ALTER COLUMN updated_at SET DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow');
			
			ALTER TABLE training_log 
			ALTER COLUMN created_at SET DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow'),
			ALTER COLUMN updated_at SET DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow');
		`,
		DownSQL: `
			-- Откатываем изменения для message_log
			ALTER TABLE message_log 
			ALTER COLUMN created_at TYPE TIMESTAMP,
			ALTER COLUMN updated_at TYPE TIMESTAMP;
			
			-- Откатываем изменения для training_log
			ALTER TABLE training_log 
			ALTER COLUMN created_at TYPE TIMESTAMP,
			ALTER COLUMN updated_at TYPE TIMESTAMP;
			
			-- Устанавливаем старые значения по умолчанию
			ALTER TABLE message_log 
			ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
			ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
			
			ALTER TABLE training_log 
			ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP,
			ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
		`,
	},
	{
		Version:     2,
		Description: "Add cups_earned field to message_log table",
		UpSQL: `
			-- Добавляем поле cups_earned в таблицу message_log
			ALTER TABLE message_log 
			ADD COLUMN cups_earned INTEGER DEFAULT 0;
		`,
		DownSQL: `
			-- Удаляем поле cups_earned из таблицы message_log
			ALTER TABLE message_log 
			DROP COLUMN cups_earned;
		`,
	},
}

// MigrationRecord представляет запись о выполненной миграции
type MigrationRecord struct {
	Version     int    `db:"version"`
	Description string `db:"description"`
	AppliedAt   string `db:"applied_at"`
}

// CreateMigrationsTable создает таблицу для отслеживания миграций
func (d *Database) CreateMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() AT TIME ZONE 'Europe/Moscow')
		)
	`

	_, err := d.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// GetAppliedMigrations получает список уже примененных миграций
func (d *Database) GetAppliedMigrations() ([]MigrationRecord, error) {
	query := `SELECT version, description, applied_at FROM migrations ORDER BY version`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var migrations []MigrationRecord
	for rows.Next() {
		var migration MigrationRecord
		err := rows.Scan(&migration.Version, &migration.Description, &migration.AppliedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan migration: %w", err)
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// ApplyMigration применяет миграцию
func (d *Database) ApplyMigration(migration Migration) error {
	// Начинаем транзакцию
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Откатываем в случае ошибки

	// Выполняем SQL миграции
	_, err = tx.Exec(migration.UpSQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
	}

	// Записываем информацию о примененной миграции
	insertQuery := `INSERT INTO migrations (version, description) VALUES ($1, $2)`
	_, err = tx.Exec(insertQuery, migration.Version, migration.Description)
	if err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
	}

	// Подтверждаем транзакцию
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
	}

	return nil
}

// RunMigrations выполняет все необходимые миграции
func (d *Database) RunMigrations() error {
	// Создаем таблицу миграций
	if err := d.CreateMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Получаем уже примененные миграции
	appliedMigrations, err := d.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Создаем map для быстрого поиска
	appliedMap := make(map[int]bool)
	for _, migration := range appliedMigrations {
		appliedMap[migration.Version] = true
	}

	// Применяем новые миграции
	for _, migration := range Migrations {
		if !appliedMap[migration.Version] {
			fmt.Printf("Applying migration %d: %s\n", migration.Version, migration.Description)

			if err := d.ApplyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}

			fmt.Printf("Successfully applied migration %d\n", migration.Version)
		} else {
			fmt.Printf("Migration %d already applied, skipping\n", migration.Version)
		}
	}

	return nil
}
