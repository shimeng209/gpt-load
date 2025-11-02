package db

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// V1_3_3_AddModelMappingColumn adds model_mappings column to groups table
func V1_3_3_AddModelMappingColumn(db *gorm.DB) error {
	// Check if column already exists first
	var count int64
	if db.Dialector.Name() == "mysql" {
		db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'groups' AND COLUMN_NAME = 'model_mappings'").Count(&count)
	} else if db.Dialector.Name() == "sqlite" {
		db.Raw("SELECT COUNT(*) FROM pragma_table_info('groups') WHERE name = 'model_mappings'").Count(&count)
	}

	if count > 0 {
		logrus.Info("model_mappings column already exists, skipping")
		return nil
	}

	// Try to add the column
	if err := db.Exec("ALTER TABLE `groups` ADD COLUMN model_mappings JSON").Error; err != nil {
		// Fallback error handling for different error formats
		errStr := err.Error()
		if (db.Dialector.Name() == "sqlite" && (strings.Contains(errStr, "duplicate column name") || strings.Contains(errStr, "SQL logic error"))) ||
			(db.Dialector.Name() == "mysql" && (strings.Contains(errStr, "Duplicate column name") || strings.Contains(errStr, "1060"))) {
			logrus.Info("model_mappings column already exists (detected by error message), skipping")
			return nil
		} else {
			return fmt.Errorf("failed to add model_mappings column: %w", err)
		}
	}

	logrus.Info("Added model_mappings column to groups table")
	return nil
}