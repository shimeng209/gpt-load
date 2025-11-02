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
	defaultSelector *selector
	aliasSelectors  map[string]*selector
}

// subGroupItem represents a sub-group with its weight and current weight for round-robin
type subGroupItem struct {
	name          string
	subGroupID    uint
	weight        int
	currentWeight int
	modelOverride string
	models        []string        // 多模型支持
	lastModelIndex int            // 轮询索引
}

// SelectionResult captures the selected sub-group info, along with optional overrides.
type SelectionResult struct {
	GroupName     string
	SubGroupID    uint
	ModelOverride string
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
	if selectors == nil || (selectors.defaultSelector == nil && len(selectors.aliasSelectors) == 0) {
		return nil, fmt.Errorf("no valid sub-groups available for aggregate group '%s'", group.Name)
	}

	alias := strings.TrimSpace(modelAlias)
	if alias != "" && len(selectors.aliasSelectors) > 0 {
		if sel, ok := selectors.aliasSelectors[strings.ToLower(alias)]; ok {
			if selectedItem := sel.selectNext(); selectedItem != nil {
				return &SelectionResult{
					GroupName:     selectedItem.name,
					SubGroupID:    selectedItem.subGroupID,
					ModelOverride: sel.selectModelFromTargets(selectedItem),
				}, nil
			}
			logrus.WithFields(logrus.Fields{
				"aggregate_group": group.Name,
				"model_alias":     alias,
			}).Warn("Model alias selector has no sub-groups with active keys, falling back to default selector")
		}
	}

	if selectors.defaultSelector == nil {
		return nil, fmt.Errorf("no sub-groups with active keys for aggregate group '%s'", group.Name)
	}

	selectedItem := selectors.defaultSelector.selectNext()
	if selectedItem == nil {
		return nil, fmt.Errorf("no sub-groups with active keys for aggregate group '%s'", group.Name)
	}

	logrus.WithFields(logrus.Fields{
		"aggregate_group": group.Name,
		"model_alias":     alias,
		"selected_group":  selectedItem.name,
	}).Debug("Selected sub-group from aggregate")

	return &SelectionResult{
		GroupName:     selectedItem.name,
		SubGroupID:    selectedItem.subGroupID,
		ModelOverride: selectors.defaultSelector.selectModelFromTargets(selectedItem),
	}, nil
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

// createGroupSelectors creates default and model-alias selectors for an aggregate group
func (m *SubGroupManager) createGroupSelectors(group *models.Group) *groupSelectors {
	if group.GroupType != "aggregate" || len(group.SubGroups) == 0 {
		return nil
	}

	result := &groupSelectors{
		aliasSelectors: make(map[string]*selector),
	}

	subGroupMap := make(map[uint]models.GroupSubGroup, len(group.SubGroups))
	defaultItems := make([]subGroupItem, 0, len(group.SubGroups))
	for _, sg := range group.SubGroups {
		subGroupMap[sg.SubGroupID] = sg
		name := sg.SubGroupName
		if name == "" {
			name = fmt.Sprintf("group-%d", sg.SubGroupID)
		}
		defaultItems = append(defaultItems, subGroupItem{
			name:          name,
			subGroupID:    sg.SubGroupID,
			weight:        sg.Weight,
			currentWeight: 0,
			modelOverride: "",
			models:        []string{},
			lastModelIndex: 0,
		})
	}

	if len(defaultItems) > 0 {
		result.defaultSelector = newSelector(group, "", defaultItems, m.store)
	}

	if len(group.ModelMappingList) > 0 {
		for _, mapping := range group.ModelMappingList {
			alias := strings.TrimSpace(mapping.Model)
			if alias == "" {
				continue
			}

			items := make([]subGroupItem, 0, len(mapping.Targets))
			seen := make(map[uint]struct{}, len(mapping.Targets))
			for _, target := range mapping.Targets {
				if _, exists := seen[target.SubGroupID]; exists {
					continue
				}
				sg, ok := subGroupMap[target.SubGroupID]
				if !ok {
					logrus.WithFields(logrus.Fields{
						"aggregate_group": group.Name,
						"model_alias":     alias,
						"sub_group_id":    target.SubGroupID,
					}).Warn("Model mapping target references unknown sub-group")
					continue
				}
				weight := target.Weight
				if weight <= 0 {
					weight = sg.Weight
				}
				if weight <= 0 {
					logrus.WithFields(logrus.Fields{
						"aggregate_group": group.Name,
						"model_alias":     alias,
						"sub_group_id":    target.SubGroupID,
					}).Warn("Model mapping target resolved to non-positive weight, skipping")
					continue
				}
				name := sg.SubGroupName
				if name == "" {
					name = fmt.Sprintf("group-%d", sg.SubGroupID)
				}
				// 处理多模型支持，保持向后兼容
				var models []string
				if len(target.Models) > 0 {
					models = target.Models
				} else if target.Model != "" {
					models = []string{target.Model}
				}

				items = append(items, subGroupItem{
					name:          name,
					subGroupID:    target.SubGroupID,
					weight:        weight,
					currentWeight: 0,
					modelOverride: target.Model, // 保持向后兼容
					models:        models,
					lastModelIndex: 0,
				})
				seen[target.SubGroupID] = struct{}{}
			}

			if len(items) == 0 {
				continue
			}

			result.aliasSelectors[strings.ToLower(alias)] = newSelector(group, alias, items, m.store)
		}
	}

	if result.defaultSelector == nil && len(result.aliasSelectors) == 0 {
		return nil
	}

	return result
}

func newSelector(group *models.Group, alias string, items []subGroupItem, store store.Store) *selector {
	return &selector{
		groupID:    group.ID,
		groupName:  group.Name,
		modelAlias: alias,
		subGroups:  items,
		store:      store,
	}
}

// selector encapsulates the weighted round-robin algorithm for a single aggregate group
type selector struct {
	groupID    uint
	groupName  string
	modelAlias string
	subGroups  []subGroupItem
	store      store.Store
	mu         sync.Mutex
}

// selectNext uses weighted round-robin algorithm to select a sub-group with active keys
func (s *selector) selectNext() *subGroupItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.subGroups) == 0 {
		return nil
	}

	if len(s.subGroups) == 1 {
		if s.hasActiveKeys(s.subGroups[0].subGroupID) {
			return &s.subGroups[0]
		}
		logrus.WithFields(logrus.Fields{
			"group_id":   s.subGroups[0].subGroupID,
			"group_name": s.subGroups[0].name,
			"model_alias": func() string {
				if s.modelAlias == "" {
					return ""
				}
				return s.modelAlias
			}(),
		}).Debug("Single sub-group has no active keys")
		return nil
	}

	attempted := make(map[uint]bool)
	for len(attempted) < len(s.subGroups) {
		item := s.selectByWeight()
		if item == nil {
			break
		}

		if attempted[item.subGroupID] {
			continue
		}
		attempted[item.subGroupID] = true

		if s.hasActiveKeys(item.subGroupID) {
			logrus.WithFields(logrus.Fields{
				"aggregate_group": s.groupName,
				"selected_group":  item.name,
				"attempts":        len(attempted),
				"model_alias":     s.modelAlias,
			}).Debug("Selected sub-group with active keys")
			return item
		}

		logrus.WithFields(logrus.Fields{
			"group_id":   item.subGroupID,
			"group_name": item.name,
			"attempts":   len(attempted),
			"model_alias": func() string {
				if s.modelAlias == "" {
					return ""
				}
				return s.modelAlias
			}(),
		}).Debug("Sub-group has no active keys, trying next")
	}

	logrus.WithFields(logrus.Fields{
		"aggregate_group":  s.groupName,
		"total_sub_groups": len(s.subGroups),
		"model_alias":      s.modelAlias,
	}).Warn("No sub-groups with active keys available")

	return nil
}

// selectByWeight implements smooth weighted round-robin algorithm
func (s *selector) selectByWeight() *subGroupItem {
	totalWeight := 0
	var best *subGroupItem

	for i := range s.subGroups {
		item := &s.subGroups[i]
		totalWeight += item.weight
		item.currentWeight += item.weight

		if best == nil || item.currentWeight > best.currentWeight {
			best = item
		}
	}

	if best == nil {
		return &s.subGroups[0]
	}

	best.currentWeight -= totalWeight
	return best
}

// hasActiveKeys checks if a sub-group has available API keys
func (s *selector) hasActiveKeys(groupID uint) bool {
	key := fmt.Sprintf("group:%d:active_keys", groupID)
	length, err := s.store.LLen(key)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"group_id": groupID,
			"error":    err,
		}).Debug("Error checking active keys, assuming available")
		return true
	}
	return length > 0
}

// selectModelFromTargets 从多个模型中选择一个（轮询）
func (s *selector) selectModelFromTargets(item *subGroupItem) string {
	// 优先使用单个模型（向后兼容）
	if item.modelOverride != "" {
		return item.modelOverride
	}

	// 处理多模型情况
	if len(item.models) == 0 {
		return ""
	}

	if len(item.models) == 1 {
		return item.models[0]
	}

	// 轮询选择
	selectedModel := item.models[item.lastModelIndex%len(item.models)]
	item.lastModelIndex++

	return selectedModel
}
