package db

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// checkIndexExists 检查索引是否已存在
func checkIndexExists(db *gorm.DB, tableName, indexName string) (bool, error) {
	var count int64
	query := ""

	if db.Dialector.Name() == "mysql" {
		query = `
			SELECT COUNT(*)
			FROM information_schema.statistics
			WHERE table_schema = DATABASE()
			AND table_name = ?
			AND index_name = ?
		`
	} else if db.Dialector.Name() == "postgres" {
		query = `
			SELECT COUNT(*)
			FROM pg_indexes
			WHERE tablename = ?
			AND indexname = ?
		`
	}

	if query == "" {
		return false, fmt.Errorf("unsupported database type: %s", db.Dialector.Name())
	}

	err := db.Raw(query, tableName, indexName).Scan(&count).Error
	return count > 0, err
}

// createIndexIfNotExists 安全创建索引（如果不存在）
func createIndexIfNotExists(db *gorm.DB, tableName, indexName, indexDefinition string) error {
	exists, err := checkIndexExists(db, tableName, indexName)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}

	if exists {
		logrus.Debugf("Index %s already exists on table %s, skipping...", indexName, tableName)
		return nil
	}

	logrus.Debugf("Creating index %s on table %s...", indexName, tableName)
	return db.Exec(indexDefinition).Error
}

// V1_3_4_OptimizeRequestLogIndexes 添加优化的索引以提升日志查询和删除性能
func V1_3_4_OptimizeRequestLogIndexes(db *gorm.DB) error {
	// 检查数据库类型
	if db.Dialector.Name() == "mysql" {
		// MySQL索引优化 - 使用安全的索引创建方法
		// 为日志删除操作优化：timestamp + id 的复合索引
		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_timestamp_id", `
			CREATE INDEX idx_request_logs_timestamp_id
			ON request_logs (timestamp, id)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_timestamp_id: %w", err)
		}

		// 为日志查询优化：timestamp + group_id 复合索引
		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_timestamp_group_id", `
			CREATE INDEX idx_request_logs_timestamp_group_id
			ON request_logs (timestamp, group_id)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_timestamp_group_id: %w", err)
		}

		// 为统计查询优化：group_id + timestamp 复合索引
		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_group_id_timestamp", `
			CREATE INDEX idx_request_logs_group_id_timestamp
			ON request_logs (group_id, timestamp)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_group_id_timestamp: %w", err)
		}

	} else if db.Dialector.Name() == "postgres" {
		// PostgreSQL索引优化 - 使用安全的索引创建方法
		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_timestamp_id", `
			CREATE INDEX CONCURRENTLY idx_request_logs_timestamp_id
			ON request_logs (timestamp, id)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_timestamp_id: %w", err)
		}

		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_timestamp_group_id", `
			CREATE INDEX CONCURRENTLY idx_request_logs_timestamp_group_id
			ON request_logs (timestamp, group_id)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_timestamp_group_id: %w", err)
		}

		if err := createIndexIfNotExists(db, "request_logs", "idx_request_logs_group_id_timestamp", `
			CREATE INDEX CONCURRENTLY idx_request_logs_group_id_timestamp
			ON request_logs (group_id, timestamp)
		`); err != nil {
			return fmt.Errorf("failed to create idx_request_logs_group_id_timestamp: %w", err)
		}
	}

	return nil
}