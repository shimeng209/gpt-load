package services

import (
	"fmt"
	"gpt-load/internal/models"
	"gpt-load/internal/store"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// SubGroupManager manages weighted round-robin selection for all aggregate groups
type SubGroupManager struct {
	store     store.Store
	selectors map[uint]*groupSelectors
	mu        sync.RWMutex
}

type groupSelectors struct {
	modelLevelSelector *modelLevelSelector
}

// modelSelectionItem represents a model with its weight and sub-group info for round-robin
type modelSelectionItem struct {
	model         string
	subGroupID    uint
	subGroupName  string
	weight        int
	currentWeight int
}


// SelectionResult captures the selected model and sub-group info
type SelectionResult struct {
	GroupName     string
	SubGroupID    uint
	SelectedModel string
}

// NewSubGroupManager creates a new sub-group manager service
func NewSubGroupManager(store store.Store) *SubGroupManager {
	return &SubGroupManager{
		store:     store,
		selectors: make(map[uint]*groupSelectors),
	}
}

// SelectSubGroup selects an appropriate sub-group for the given aggregate group
func (m *SubGroupManager) SelectSubGroup(group *models.Group, modelAlias string) (*SelectionResult, error) {
	if group.GroupType != "aggregate" {
		return nil, nil
	}

	selectors := m.getSelectors(group)
	if selectors == nil {
		return nil, fmt.Errorf("no valid selectors available for aggregate group '%s'", group.Name)
	}

	alias := strings.TrimSpace(modelAlias)

	// 只使用模型级别选择器
	if alias != "" && selectors.modelLevelSelector != nil {
		if selectedItem := selectors.modelLevelSelector.selectNextModel(); selectedItem != nil {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
				"selected_model":  selectedItem.model,
				"sub_group":       selectedItem.subGroupName,
			}).Debug("Selected model from aggregate group")

			return &SelectionResult{
				GroupName:     selectedItem.subGroupName,
				SubGroupID:    selectedItem.subGroupID,
				SelectedModel: selectedItem.model,
			}, nil
		}

		return nil, fmt.Errorf("no available models for model alias '%s' in aggregate group '%s'", alias, group.Name)
	}

	return nil, fmt.Errorf("no model mapping found for alias '%s' in aggregate group '%s'", alias, group.Name)
}

// RebuildSelectors rebuild all selectors based on the incoming group
func (m *SubGroupManager) RebuildSelectors(groups map[string]*models.Group) {
	newSelectors := make(map[uint]*groupSelectors)

	for _, group := range groups {
		if group.GroupType == "aggregate" && len(group.SubGroups) > 0 {
			if sel := m.createGroupSelectors(group); sel != nil {
				newSelectors[group.ID] = sel
			}
		}
	}

	m.mu.Lock()
	m.selectors = newSelectors
	m.mu.Unlock()

	logrus.WithField("new_count", len(newSelectors)).Debug("Rebuilt selectors for aggregate groups")
}

// getSelectors retrieves or creates selectors for the aggregate group
func (m *SubGroupManager) getSelectors(group *models.Group) *groupSelectors {
	m.mu.RLock()
	if sel, exists := m.selectors[group.ID]; exists {
		m.mu.RUnlock()
		return sel
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if sel, exists := m.selectors[group.ID]; exists {
		return sel
	}

	sel := m.createGroupSelectors(group)
	if sel != nil {
		m.selectors[group.ID] = sel
		logrus.WithFields(logrus.Fields{
			"group_id":   group.ID,
			"group_name": group.Name,
		}).Debug("Created sub-group selectors")
	}

	return sel
}

// createGroupSelectors creates model-level selectors for an aggregate group
func (m *SubGroupManager) createGroupSelectors(group *models.Group) *groupSelectors {
	if group.GroupType != "aggregate" || len(group.SubGroups) == 0 {
		return nil
	}

	if len(group.ModelMappingList) == 0 {
		return nil
	}

	result := &groupSelectors{}

	// 创建子分组映射
	subGroupMap := make(map[uint]models.GroupSubGroup, len(group.SubGroups))
	for _, sg := range group.SubGroups {
		subGroupMap[sg.SubGroupID] = sg
	}

	// 只处理第一个模型映射（简化实现）
	mapping := group.ModelMappingList[0]
	alias := strings.TrimSpace(mapping.Model)
	if alias == "" {
		return nil
	}

	// 创建模型级别的选择器
	var modelItems []modelSelectionItem
	for _, target := range mapping.Targets {
		sg, ok := subGroupMap[target.SubGroupID]
		if !ok {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
				"sub_group_id":    target.SubGroupID,
			}).Warn("Model mapping target references unknown sub-group")
			continue
		}

		// 计算最终权重：子分组权重 × 模型映射权重
		modelMappingWeight := target.Weight
		if modelMappingWeight <= 0 {
			modelMappingWeight = 1  // 默认权重
		}

		subGroupWeight := sg.Weight
		if subGroupWeight <= 0 {
			subGroupWeight = 1  // 默认权重
		}

		finalWeight := subGroupWeight * modelMappingWeight

		logrus.WithFields(logrus.Fields{
			"aggregate_group":    group.Name,
			"model_alias":        alias,
			"sub_group_id":       target.SubGroupID,
			"sub_group_weight":   subGroupWeight,
			"model_mapping_weight": modelMappingWeight,
			"final_weight":       finalWeight,
		}).Debug("Calculated final model weight")

		name := sg.SubGroupName
		if name == "" {
			name = fmt.Sprintf("group-%d", sg.SubGroupID)
		}

		// 展平模型列表 - 每个模型作为一个独立的选择项
		var models []string
		if len(target.Models) > 0 {
			models = target.Models
		} else if target.Model != "" {
			models = []string{target.Model}
		}

		for _, model := range models {
			if model == "" {
				continue
			}
			modelItems = append(modelItems, modelSelectionItem{
				model:         model,
				subGroupID:    target.SubGroupID,
				subGroupName:  name,
				weight:        finalWeight,
				currentWeight: 0,
			})
		}
	}

	// 如果有模型项，创建模型级别选择器
	if len(modelItems) > 0 {
		result.modelLevelSelector = newModelLevelSelector(group, alias, modelItems, m.store)
		return result
	}

	return nil
}

func newModelLevelSelector(group *models.Group, alias string, items []modelSelectionItem, store store.Store) *modelLevelSelector {
	return &modelLevelSelector{
		groupID:    group.ID,
		groupName:  group.Name,
		modelAlias: alias,
		modelItems: items,
		store:      store,
	}
}

// modelLevelSelector encapsulates the weighted round-robin algorithm at model level
type modelLevelSelector struct {
	groupID     uint
	groupName   string
	modelAlias  string
	modelItems  []modelSelectionItem
	store       store.Store
	mu          sync.Mutex
}

// selectNextModel uses weighted round-robin algorithm to select a model with active keys
func (m *modelLevelSelector) selectNextModel() *modelSelectionItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.modelItems) == 0 {
		return nil
	}

	if len(m.modelItems) == 1 {
		if m.hasActiveKeys(m.modelItems[0].subGroupID) {
			return &m.modelItems[0]
		}
		return nil
	}

	attempted := make(map[string]bool) // 使用model作为key
	for len(attempted) < len(m.modelItems) {
		item := m.selectModelByWeight()
		if item == nil {
			break
		}

		if attempted[item.model] {
			continue
		}
		attempted[item.model] = true

		if m.hasActiveKeys(item.subGroupID) {
			return item
		}
	}

	return nil
}

// selectModelByWeight implements smooth weighted round-robin algorithm for models
func (m *modelLevelSelector) selectModelByWeight() *modelSelectionItem {
	totalWeight := 0
	var best *modelSelectionItem

	for i := range m.modelItems {
		item := &m.modelItems[i]
		totalWeight += item.weight
		item.currentWeight += item.weight

		if best == nil || item.currentWeight > best.currentWeight {
			best = item
		}
	}

	if best == nil {
		return &m.modelItems[0]
	}

	best.currentWeight -= totalWeight
	return best
}

// hasActiveKeys checks if a sub-group has available API keys
func (m *modelLevelSelector) hasActiveKeys(groupID uint) bool {
	key := fmt.Sprintf("group:%d:active_keys", groupID)
	length, err := m.store.LLen(key)
	if err != nil {
		return true
	}
	return length > 0
}
