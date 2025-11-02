package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ModelMappingOld represents the old model mapping structure
type ModelMappingOld struct {
	Model   string                  `json:"model"`
	Targets []ModelMappingTargetOld `json:"targets"`
}

// ModelMappingTargetOld represents the old target structure
type ModelMappingTargetOld struct {
	SubGroupID   uint   `json:"sub_group_id"`
	Weight       int    `json:"weight"`
	SubGroupName string `json:"sub_group_name,omitempty"`
	Model        string `json:"model"`
}

// ModelMappingNew represents the new model mapping structure with multi-model support
type ModelMappingNew struct {
	Model   string                    `json:"model"`
	Targets []ModelMappingTargetNew   `json:"targets"`
}

// ModelMappingTargetNew represents the new target structure with multi-model support
type ModelMappingTargetNew struct {
	SubGroupID   uint     `json:"sub_group_id"`
	Weight       int      `json:"weight"`
	SubGroupName string   `json:"sub_group_name,omitempty"`
	Model        string   `json:"model"`        // Keep for backward compatibility
	Models       []string `json:"models"`       // New field for multiple models
}

// V1_3_5_SupportMultiModels adds support for multiple models per target
func V1_3_5_SupportMultiModels(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Get all groups with model_mappings
		var groups []struct {
			ID            uint   `gorm:"column:id"`
			ModelMappings string `gorm:"column:model_mappings"`
		}

		if err := tx.Table("groups").
			Select("id, model_mappings").
			Where("model_mappings IS NOT NULL AND model_mappings != ''").
			Find(&groups).Error; err != nil {
			return fmt.Errorf("failed to fetch groups with model mappings: %w", err)
		}

		if len(groups) == 0 {
			logrus.Info("No model mappings found to migrate")
			return nil
		}

		logrus.WithField("groups_count", len(groups)).Info("Starting migration for multi-model support")

		for _, group := range groups {
			if err := migrateGroupModelMappings(tx, group.ID, group.ModelMappings); err != nil {
				logrus.WithFields(logrus.Fields{
					"group_id": group.ID,
					"error":    err,
				}).Error("Failed to migrate group model mappings")
				return err
			}
		}

		logrus.Info("Successfully migrated model mappings for multi-model support")
		return nil
	})
}

// migrateGroupModelMappings migrates a single group's model mappings
func migrateGroupModelMappings(db *gorm.DB, groupID uint, modelMappingsJSON string) error {
	if strings.TrimSpace(modelMappingsJSON) == "" {
		return nil
	}

	var oldMappings []ModelMappingOld
	if err := json.Unmarshal([]byte(modelMappingsJSON), &oldMappings); err != nil {
		// Try parsing as a single mapping object (for backward compatibility)
		var singleMapping ModelMappingOld
		if err := json.Unmarshal([]byte(modelMappingsJSON), &singleMapping); err != nil {
			logrus.WithFields(logrus.Fields{
				"group_id": groupID,
				"error":    err,
			}).Warn("Failed to parse model mappings, skipping migration")
			return nil // Not an error, just skip invalid data
		}
		oldMappings = []ModelMappingOld{singleMapping}
	}

	// Convert to new format
	newMappings := make([]ModelMappingNew, len(oldMappings))
	for i, oldMapping := range oldMappings {
		newTargets := make([]ModelMappingTargetNew, len(oldMapping.Targets))
		for j, oldTarget := range oldMapping.Targets {
			newTargets[j] = ModelMappingTargetNew{
				SubGroupID:   oldTarget.SubGroupID,
				Weight:       oldTarget.Weight,
				SubGroupName: oldTarget.SubGroupName,
				Model:        oldTarget.Model, // Preserve for backward compatibility
				Models:       []string{oldTarget.Model}, // Convert single model to array
			}
		}

		newMappings[i] = ModelMappingNew{
			Model:   oldMapping.Model,
			Targets: newTargets,
		}
	}

	// Marshal back to JSON
	newMappingsJSON, err := json.Marshal(newMappings)
	if err != nil {
		return fmt.Errorf("failed to marshal new model mappings: %w", err)
	}

	// Update the database
	result := db.Table("groups").
		Where("id = ?", groupID).
		Update("model_mappings", string(newMappingsJSON))

	if result.Error != nil {
		return fmt.Errorf("failed to update model mappings: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		logrus.WithField("group_id", groupID).Warn("No rows affected during model mapping migration")
	}

	logrus.WithFields(logrus.Fields{
		"group_id":      groupID,
		"mappings_count": len(newMappings),
	}).Debug("Successfully migrated group model mappings")

	return nil
}