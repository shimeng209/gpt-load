package handler

import (
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/i18n"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// LogResponse defines the structure for log entries in the API response
type LogResponse struct {
	models.RequestLog
}

// GetLogs handles fetching request logs with filtering and pagination.
func (s *Server) GetLogs(c *gin.Context) {
	query := s.LogService.GetLogsQuery(c)

	var logs []models.RequestLog
	query = query.Order("timestamp desc")
	pagination, err := response.Paginate(c, query, &logs)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	// 解密所有日志中的密钥用于前端显示
	for i := range logs {
		if logs[i].KeyValue != "" {
			decryptedValue, err := s.EncryptionSvc.Decrypt(logs[i].KeyValue)
			if err != nil {
				logrus.WithError(err).WithField("log_id", logs[i].ID).Error("Failed to decrypt log key value")
				logs[i].KeyValue = "failed-to-decrypt"
			} else {
				logs[i].KeyValue = decryptedValue
			}
		}
	}

	pagination.Items = logs
	response.Success(c, pagination)
}

// GetLogDetail handles fetching a single log entry by ID with all fields including request_body.
func (s *Server) GetLogDetail(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.Error(c, app_errors.NewValidationError("log ID is required"))
		return
	}

	var log models.RequestLog
	err := s.DB.Where("id = ?", id).First(&log).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, app_errors.NewNotFoundError("log not found"))
		} else {
			response.Error(c, app_errors.ParseDBError(err))
		}
		return
	}

	// 解密密钥用于前端显示
	if log.KeyValue != "" {
		decryptedValue, err := s.EncryptionSvc.Decrypt(log.KeyValue)
		if err != nil {
			logrus.WithError(err).WithField("log_id", log.ID).Error("Failed to decrypt log key value")
			log.KeyValue = "failed-to-decrypt"
		} else {
			log.KeyValue = decryptedValue
		}
	}

	response.Success(c, log)
}

// ExportLogs handles exporting filtered log keys to a CSV file.
func (s *Server) ExportLogs(c *gin.Context) {
	filename := fmt.Sprintf("log_keys_export_%s.csv", time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// Stream the response
	err := s.LogService.StreamLogKeysToCSV(c, c.Writer)
	if err != nil {
		log.Printf("Failed to stream log keys to CSV: %v", err)
		c.JSON(500, gin.H{"error": i18n.Message(c, "error.export_logs")})
		return
	}
}
