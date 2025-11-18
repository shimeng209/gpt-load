<script setup lang="ts">
import { keysApi } from "@/api/keys";
import type { Group, ModelMapping, ModelMappingTarget, SubGroupInfo } from "@/types/models";
import { copy } from "@/utils/clipboard";
import {
  Add,
  CheckmarkCircle,
  CopyOutline,
  CreateOutline,
  InformationCircleOutline,
  PlayCircleOutline,
  Search,
  Trash,
} from "@vicons/ionicons5";
import {
  NButton,
  NButtonGroup,
  NEmpty,
  NIcon,
  NInput,
  NSpin,
  NTag,
  NTooltip,
  useDialog,
  useMessage,
} from "naive-ui";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import ModelMappingModal from "./ModelMappingModal.vue";

const { t } = useI18n();
const message = useMessage();
const dialog = useDialog();

interface ModelMappingRow extends ModelMapping {
  totalWeight: number;
}

interface Props {
  selectedGroup: Group | null;
  modelMappings?: ModelMapping[];
  subGroups?: SubGroupInfo[];
  groups?: Group[];
  loading?: boolean;
}

interface Emits {
  (e: "refresh"): void;
  (e: "group-select", groupId: number): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const modalShow = ref(false);
const modalMode = ref<'add' | 'edit'>('add');
const editingModelMapping = ref<ModelMapping | null>(null);

// 搜索和过滤状态
const searchText = ref("");

// 测试状态跟踪
const testingModels = ref<Set<string>>(new Set());
const successfullyTestedModels = ref<Set<string>>(new Set());
const failedTestedModels = ref<Set<string>>(new Set());
const testResults = ref<Map<string, { duration: number; timestamp: number }>>(new Map());

// 监听数据变化，但不要过于频繁地清除测试状态
watch(
  [() => props.modelMappings, () => props.subGroups],
  (newVal, oldVal) => {
    // 只有当数据真正发生变化时才清除测试状态
    // 避免在测试过程中因为数据刷新而清除状态
    if (oldVal[0] && oldVal[1]) {
      // 检查是否是实质性变化，而不是测试触发的刷新
      const mappingsChanged = JSON.stringify(newVal[0]) !== JSON.stringify(oldVal[0]);
      const subGroupsChanged = JSON.stringify(newVal[1]) !== JSON.stringify(oldVal[1]);

      if (mappingsChanged || subGroupsChanged) {
        successfullyTestedModels.value.clear();
        failedTestedModels.value.clear();
        testResults.value.clear();
      }
    }
  },
  { deep: true }
);

// 计算带总权重的模型映射数据，按字母顺序排序
const sortedModelMappings = computed<ModelMappingRow[]>(() => {
  if (!props.modelMappings) {
    return [];
  }

  return props.modelMappings
    .map(mapping => {
      // 计算有效权重（排除没有有效key的子分组）
      const validWeight = mapping.targets.reduce((sum, target) => {
        const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
        // 如果子分组没有有效key，权重归零
        if (subGroup && subGroup.active_keys > 0) {
          return sum + target.weight;
        }
        return sum;
      }, 0);

      return {
        ...mapping,
        totalWeight: validWeight,
      };
    })
    .sort((a, b) => a.model.localeCompare(b.model)); // 按模型名字母顺序排序
});

// 过滤后的模型映射（应用搜索）
const filteredModelMappings = computed<ModelMappingRow[]>(() => {
  let filtered = sortedModelMappings.value;

  // 模型别名搜索过滤（不区分大小写）
  if (searchText.value.trim()) {
    const searchLower = searchText.value.trim().toLowerCase();
    filtered = filtered.filter(mapping => {
      return mapping.model.toLowerCase().includes(searchLower);
    });
  }

  return filtered;
});

function openAddModal() {
  editingModelMapping.value = null;
  modalMode.value = 'add';
  modalShow.value = true;
}

function openEditModal(modelMapping: ModelMapping) {
  editingModelMapping.value = modelMapping;
  modalMode.value = 'edit';
  // 使用 nextTick 确保 editingModelMapping 更新后再显示弹窗
  nextTick(() => {
    modalShow.value = true;
  });
}

async function deleteModelMapping(modelMapping: ModelMapping) {
  if (!props.selectedGroup?.id) {
    return;
  }

  const d = dialog.warning({
    title: t("modelMappings.removeModelMapping"),
    content: t("modelMappings.confirmRemoveModelMapping", { model: modelMapping.model }),
    positiveText: t("common.confirm"),
    negativeText: t("common.cancel"),
    onPositiveClick: async () => {
      if (!props.selectedGroup?.id) {
        return;
      }

      d.loading = true;
      try {
        // 移除指定的模型映射
        const remainingMappings =
          props.modelMappings?.filter(m => m.model !== modelMapping.model) || [];

        await keysApi.updateGroup(props.selectedGroup.id, {
          model_mappings: remainingMappings,
        });

        message.success(t("common.operationSuccess"));
        emit("refresh");
      } catch (error) {
        console.error("Failed to delete model mapping:", error);
        message.error(t("common.operationFailed"));
      } finally {
        d.loading = false;
      }
    },
  });
}

// Handle success after modal operations
function handleSuccess() {
  emit("refresh");
  // 只有在添加/编辑模型映射时才清除测试状态，因为模型映射可能已改变
  successfullyTestedModels.value.clear();
  failedTestedModels.value.clear();
  testResults.value.clear();
}

// Get sub group name by ID
function getSubGroupName(subGroupId: number): string {
  const subGroup = props.subGroups?.find(sg => sg.group.id === subGroupId);
  if (subGroup) {
    return subGroup.group.display_name || subGroup.group.name;
  }

  // 如果在子分组中找不到，尝试从所有分组中查找
  const group = props.groups?.find(g => g.id === subGroupId);
  return group?.display_name || group?.name || `#${subGroupId}`;
}

// Calculate target percentage
function getTargetPercentage(target: ModelMappingTarget, mapping: ModelMappingRow): number {
  // 检查子分组是否有有效key
  const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
  if (!subGroup || subGroup.active_keys === 0) {
    return 0; // 子分组没有有效key，权重归零
  }

  const totalWeight = mapping.totalWeight;
  if (totalWeight === 0) {
    return 0;
  }
  return Math.round((target.weight / totalWeight) * 100);
}

// Test sub group model availability (reuse key testing logic)
async function testSubGroupModel(target: ModelMappingTarget, mapping: ModelMapping) {
  const testKey = `${mapping.model}_${target.model}_${target.sub_group_id}`;

  // 防止重复测试
  if (testingModels.value.has(testKey)) {
    return;
  }

  const subGroup = props.subGroups?.find(sg => sg.group.id === target.sub_group_id);
  if (!subGroup || subGroup.active_keys === 0) {
    message.warning(`子分组 "${getSubGroupName(target.sub_group_id)}" 没有可用的密钥`);
    return;
  }

  if (!props.selectedGroup?.id) {
    message.error("未选择分组");
    return;
  }

  try {
    // 标记为正在测试
    testingModels.value.add(testKey);

    // 创建测试加载状态
    const loadingMessage = message.loading(`正在测试模型 "${target.model}" 的可用性...`, {
      duration: 0,
    });

    // 获取子分组的密钥列表
    const keysResponse = await keysApi.getGroupKeys({
      group_id: target.sub_group_id,
      page: 1,
      page_size: 50, // 获取足够多的密钥来随机选择
      status: "active", // 只获取有效的密钥
    });

    if (!keysResponse.items || keysResponse.items.length === 0) {
      loadingMessage.destroy();
      message.warning(`子分组 "${getSubGroupName(target.sub_group_id)}" 没有可用的有效密钥`);
      return;
    }

    // 随机选择一个密钥进行测试
    const randomKey = keysResponse.items[Math.floor(Math.random() * keysResponse.items.length)];

    // 调用与密钥测试相同的API，传递实际的模型名称
    const response = await keysApi.testKeys(target.sub_group_id, randomKey.key_value, target.model);
    const curValid = response.results?.[0] || {};

    loadingMessage.destroy();

    if (curValid.is_valid) {
      // 标记为测试成功并存储结果
      successfullyTestedModels.value.add(testKey);
      testResults.value.set(testKey, {
        duration: response.total_duration || 0,
        timestamp: Date.now(),
      });

      message.success(
        `模型 "${target.model}" 测试成功！耗时 ${formatDuration(response.total_duration || 0)}`
      );
    } else {
      // 测试失败时标记失败状态，移除成功状态和结果
      failedTestedModels.value.add(testKey);
      successfullyTestedModels.value.delete(testKey);
      testResults.value.delete(testKey);

      message.error(`模型 "${target.model}" 测试失败：${curValid.error || "测试失败"}`, {
        keepAliveOnHover: true,
        duration: 5000,
        closable: true,
      });
    }

    // 模型测试不需要刷新数据，因为测试操作不会修改任何数据
    // 移除测试完成后的自动刷新，避免触发全局loading动画
  } catch (error) {
    console.error("测试模型失败:", error);
    // 标记为失败状态
    failedTestedModels.value.add(testKey);
    successfullyTestedModels.value.delete(testKey);
    testResults.value.delete(testKey);

    message.error(
      `模型 "${target.model}" 测试失败：${error instanceof Error ? error.message : "未知错误"}`
    );
  } finally {
    // 无论成功或失败，都移除测试状态
    testingModels.value.delete(testKey);
  }
}

// Format duration helper function (reused from KeyTable)
function formatDuration(ms: number): string {
  if (ms < 0) {
    return "0ms";
  }

  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);
  const milliseconds = ms % 1000;

  let result = "";
  if (minutes > 0) {
    result += `${minutes}m`;
  }
  if (seconds > 0) {
    result += `${seconds}s`;
  }
  if (milliseconds > 0 || result === "") {
    result += `${milliseconds}ms`;
  }

  return result;
}

// 获取测试按钮类型（根据延迟设置颜色）- 使用自然色系
function getTestButtonType(testKey: string): "default" | "info" | "warning" | "success" | "error" {
  // 优先检查失败状态
  if (failedTestedModels.value.has(testKey)) {
    return "error";
  }

  const result = testResults.value.get(testKey);
  if (!result) {
    return "default";
  }

  const duration = result.duration;

  // 自然色系方案：森林绿 -> 天蓝 -> 柠檬黄 -> 橙色 -> 棕色 -> 红色 -> 紫色 -> 灰色
  // < 200ms: success (绿色) - 极速响应
  // 200-500ms: info (蓝色) - 很快响应
  // 500-1000ms: warning (黄色) - 快速响应
  // 1000-2000ms: default (橙色) - 正常响应
  // 2000-3000ms: warning (棕色) - 较慢响应
  // 3000-4000ms: error (红色) - 慢速响应
  // 4000-5000ms: error (紫色) - 很慢响应
  // > 5000ms: default (灰色) - 超时响应
  if (duration < 200) {
    return "success";
  } // 森林绿
  if (duration < 500) {
    return "info";
  } // 天蓝
  if (duration < 1000) {
    return "warning";
  } // 柠檬黄
  if (duration < 2000) {
    return "default";
  } // 橙色
  if (duration < 3000) {
    return "warning";
  } // 棕色
  if (duration < 4000) {
    return "error";
  } // 红色
  if (duration < 5000) {
    return "error";
  } // 紫色
  return "default"; // 灰色
}

// 获取测试按钮的自定义CSS类（用于更精细的颜色控制）
function getTestButtonClass(testKey: string): string {
  // 优先检查失败状态
  if (failedTestedModels.value.has(testKey)) {
    return "test-failed";
  }

  const result = testResults.value.get(testKey);
  if (!result) {
    return "";
  }

  const duration = result.duration;

  // 绿色渐变配色方案：
  // < 200ms: 超快 - 亮绿色（浅色系）
  // 200-500ms: 很快 - 深绿色
  // 500-1000ms: 快 - 黄绿色
  // 1000-2000ms: 正常 - 金黄色
  // 2000-3000ms: 较慢 - 橙黄色
  // 3000-4000ms: 慢 - 橙色
  // 4000-5000ms: 很慢 - 红色
  // > 5000ms: 超时 - 深红色
  if (duration < 200) {
    return "test-very-fast";
  }
  if (duration < 500) {
    return "test-excellent";
  }
  if (duration < 1000) {
    return "test-fast";
  }
  if (duration < 2000) {
    return "test-normal";
  }
  if (duration < 3000) {
    return "test-slow";
  }
  if (duration < 4000) {
    return "test-very-slow";
  }
  if (duration < 5000) {
    return "test-extremely-slow";
  }
  return "test-timeout";
}

// 获取测试按钮文字（显示具体耗时或状态）
function getTestButtonText(testKey: string): string {
  // 如果测试失败，显示"失败"
  if (failedTestedModels.value.has(testKey)) {
    return "失败";
  }

  const result = testResults.value.get(testKey);
  if (!result) {
    return "测试";
  }

  const duration = result.duration;
  const formattedDuration = formatDuration(duration);

  // 根据延迟时间返回对应的耗时
  return formattedDuration;
}

// 复制模型别名功能
async function copyModelAlias(modelAlias: string) {
  const success = await copy(modelAlias);

  if (success) {
    message.success(`已复制模型别名: ${modelAlias}`);
  } else {
    message.error("复制失败，请手动复制");
  }
}

// 生成唯一的模型别名
function generateUniqueAlias(originalAlias: string): string {
  let counter = 1;
  let newAlias = `${originalAlias}-${counter}`;

  while (props.modelMappings?.some((m: ModelMapping) => m.model === newAlias)) {
    counter++;
    newAlias = `${originalAlias}-${counter}`;
  }

  return newAlias;
}

// 复制模型别名卡片功能
async function duplicateModelMapping(mapping: ModelMapping) {
  if (!props.selectedGroup?.id) {
    message.error("未选择分组");
    return;
  }

  // 生成新的模型别名
  const newModelAlias = generateUniqueAlias(mapping.model);

  // 创建复制的映射数据
  const duplicatedMapping: ModelMapping = {
    model: newModelAlias,
    targets: mapping.targets.map(target => ({
      ...target,
      // 确保创建新的目标对象，避免引用问题
    }))
  };

  // 设置为添加模式，并预填充数据
  editingModelMapping.value = duplicatedMapping;
  modalMode.value = 'add';

  // 使用 nextTick 确保数据更新后再显示弹窗
  await nextTick();
  modalShow.value = true;

  message.info(`已复制模型映射配置，请编辑新的模型别名`);
}
</script>

<template>
  <div class="model-mapping-table-container">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <n-button
          type="info"
          size="small"
          @click="openAddModal"
          :disabled="!subGroups || subGroups.length === 0"
        >
          <template #icon>
            <n-icon :component="Add" />
          </template>
          {{ t("modelMappings.addModelMapping") }}
        </n-button>
      </div>
      <div class="toolbar-right">
        <n-input
          v-model:value="searchText"
          :placeholder="t('modelMappings.modelAlias')"
          size="small"
          style="width: 200px"
          clearable
        >
          <template #prefix>
            <n-icon :component="Search" />
          </template>
        </n-input>
      </div>
    </div>

    <!-- 模型映射卡片网格 -->
    <div class="model-mappings-grid-container">
      <n-spin :show="props.loading || false">
        <div
          v-if="!props.modelMappings || props.modelMappings.length === 0"
          class="empty-container"
        >
          <n-empty :description="t('modelMappings.noModelMappings')" />
        </div>
        <div v-else-if="filteredModelMappings.length === 0" class="empty-container">
          <n-empty :description="t('keys.noMatchingKeys')" />
        </div>
        <div v-else class="model-mappings-grid">
          <div
            v-for="mapping in filteredModelMappings"
            :key="mapping.model"
            class="key-card status-model-mapping"
          >
            <!-- 主标题行 -->
            <div class="key-main">
              <div class="key-section">
                <div class="quick-actions">
                  <n-tag type="info" size="small">模型别名</n-tag>
                </div>
                <div class="model-mapping-names">
                  <span class="display-name">{{ mapping.model }}</span>
                </div>
                <!-- 复制按钮 -->
                <n-button
                  size="tiny"
                  text
                  @click="copyModelAlias(mapping.model)"
                  title="复制模型别名"
                  class="copy-model-alias-btn"
                >
                  <template #icon>
                    <n-icon :component="CopyOutline" />
                  </template>
                </n-button>
              </div>
            </div>

            <!-- 子分组进度条显示（显示所有子分组） -->
            <div class="weight-display">
              <div class="subgroup-progress-bars">
                <div
                  v-for="(target, index) in mapping.targets"
                  :key="index"
                  class="target-progress-item"
                >
                  <!-- 模型名称行 -->
                  <div class="target-model-row">
                    <span class="target-model-name">{{ target.model }}</span>
                    <n-button
                      size="tiny"
                      :type="getTestButtonType(`${mapping.model}_${target.model}_${target.sub_group_id}`)"
                      :class="[
                        'target-test-button',
                        getTestButtonClass(`${mapping.model}_${target.model}_${target.sub_group_id}`),
                      ]"
                      ghost
                      @click="testSubGroupModel(target, mapping)"
                      :disabled="
                        (subGroups?.find(sg => sg.group.id === target.sub_group_id)?.active_keys ||
                          0) === 0 || testingModels.has(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                      "
                      :loading="testingModels.has(`${mapping.model}_${target.model}_${target.sub_group_id}`)"
                    >
                      <template
                        #icon
                        v-if="!testingModels.has(`${mapping.model}_${target.model}_${target.sub_group_id}`)"
                      >
                        <n-icon
                          :component="
                            successfullyTestedModels.has(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                              ? CheckmarkCircle
                              : PlayCircleOutline
                          "
                        />
                      </template>
                      {{
                        testingModels.has(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                          ? "测试中"
                          : getTestButtonText(`${mapping.model}_${target.model}_${target.sub_group_id}`)
                      }}
                    </n-button>
                  </div>
                  <!-- 进度条行 -->
                  <div class="target-progress-bar">
                    <div
                      class="weight-fill"
                      :class="{
                        'weight-fill-active':
                          (subGroups?.find(sg => sg.group.id === target.sub_group_id)
                            ?.active_keys || 0) > 0,
                        'weight-fill-unavailable':
                          (subGroups?.find(sg => sg.group.id === target.sub_group_id)
                            ?.active_keys || 0) === 0,
                      }"
                      :style="{ width: `${Math.min(getTargetPercentage(target, mapping), 100)}%` }"
                    />
                  </div>
                </div>
              </div>
            </div>

            <!-- 操作按钮行 -->
            <div class="key-bottom">
              <!-- 左侧悬浮查看详情按钮 -->
              <n-tooltip trigger="hover" placement="top">
                <template #trigger>
                  <n-button round tertiary type="default" size="tiny">
                    <template #icon>
                      <n-icon :component="InformationCircleOutline" />
                    </template>
                  </n-button>
                </template>
                <div class="model-mapping-info-tooltip">
                  <!-- 模型映射详情 -->
                  <div class="info-header">
                    <div class="info-title">{{ mapping.model }}</div>
                    <n-tag type="info" size="small">模型映射</n-tag>
                  </div>

                  <!-- 详细信息 -->
                  <div class="info-details">
                    <div class="info-row">
                      <span class="info-label">总权重:</span>
                      <span class="info-value">{{ mapping.totalWeight }}</span>
                    </div>
                    <div class="info-row">
                      <span class="info-label">目标数量:</span>
                      <span class="info-value">{{ mapping.targets.length }}</span>
                    </div>

                    <!-- 每个目标的详细信息 -->
                    <div
                      v-for="(target, index) in mapping.targets"
                      :key="index"
                      class="target-detail"
                    >
                      <div class="info-row">
                        <span class="info-label">子分组 {{ index + 1 }}:</span>
                        <span class="info-value">{{ getSubGroupName(target.sub_group_id) }}</span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">目标模型:</span>
                        <span class="info-value target-model-detail">{{ target.model }}</span>
                      </div>
                      <div class="info-row">
                        <span class="info-label">权重:</span>
                        <span class="info-value">
                          {{ target.weight }} ({{ getTargetPercentage(target, mapping) }}%)
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </n-tooltip>

              <!-- 右侧编辑、复制和删除按钮组 -->
              <n-button-group class="key-actions">
                <n-button
                  round
                  tertiary
                  type="info"
                  size="tiny"
                  @click="openEditModal(mapping)"
                  :title="t('modelMappings.editModelMapping')"
                >
                  <template #icon>
                    <n-icon :component="CreateOutline" />
                  </template>
                  {{ t("common.edit") }}
                </n-button>
                <n-button
                  round
                  tertiary
                  type="warning"
                  size="tiny"
                  @click="duplicateModelMapping(mapping)"
                  title="复制模型映射"
                >
                  <template #icon>
                    <n-icon :component="CopyOutline" />
                  </template>
                  复制
                </n-button>
                <n-button
                  round
                  tertiary
                  size="tiny"
                  type="error"
                  @click="deleteModelMapping(mapping)"
                  :title="t('modelMappings.removeModelMapping')"
                >
                  <template #icon>
                    <n-icon :component="Trash" />
                  </template>
                  {{ t("subGroups.remove") }}
                </n-button>
              </n-button-group>
            </div>
          </div>
        </div>
      </n-spin>
    </div>

    <!-- 底部信息 -->
    <div class="pagination-container">
      <div class="pagination-info">
        <span>
          {{ t("modelMappings.totalMappings", { total: filteredModelMappings.length }) }}
          <template v-if="filteredModelMappings.length !== (props.modelMappings?.length || 0)">
            / {{ props.modelMappings?.length || 0 }}
          </template>
        </span>
      </div>
      <div class="pagination-controls">
        <span class="page-info">按模型名字母顺序排序</span>
      </div>
    </div>

    <!-- 统一的模型映射弹窗 -->
    <model-mapping-modal
      v-if="selectedGroup?.id"
      v-model:show="modalShow"
      :mode="modalMode"
      :aggregate-group="selectedGroup"
      :model-mapping="editingModelMapping"
      :existing-model-mappings="modelMappings || []"
      :sub-groups="subGroups || []"
      :groups="groups || []"
      @success="handleSuccess"
      @update:show="
        show => {
          if (!show) editingModelMapping = null;
        }
      "
    />
  </div>
</template>

<style scoped>
/* 直接复用SubGroupTable的所有样式 */
.model-mapping-table-container {
  background: var(--card-bg-solid);
  border-radius: 8px;
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  overflow: hidden;
  /* 改为最小高度，允许根据内容增长 */
  min-height: 100%;
  height: auto;
  display: flex;
  flex-direction: column;
  /* 确保容器能够包裹所有内容 */
  contain: layout;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: var(--card-bg-solid);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  gap: 16px;
  min-height: 64px;
}

.toolbar :deep(.n-button) {
  font-weight: 500;
}

.toolbar-left {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.toolbar-right {
  display: flex;
  gap: 12px;
  align-items: center;
  flex: 1;
  justify-content: flex-end;
  min-width: 0;
}

.model-mappings-grid-container {
  flex: 1;
  /* 让容器能够根据卡片高度自然展开 */
  overflow-y: visible; /* 改为可见，不限制内容 */
  overflow-x: hidden; /* 防止横向溢出 */
  padding: 16px;
  min-height: 0;
  /* 确保容器能够随内容增高 */
  height: auto;
  /* 允许容器在父容器内增长 */
  max-height: none;
}

.model-mappings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  /* 确保网格能够正确处理不同高度的卡片 */
  grid-auto-rows: min-content;
  /* 让网格能够自然增长 */
  align-items: start;
  /* 确保内容不会被截断 */
  overflow: visible;
}

.key-card {
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 16px;
  transition: all 0.2s;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  width: 100%;
  min-height: auto;
  height: auto; /* 确保高度完全由内容决定 */
  display: flex;
  flex-direction: column;
  /* 防止内容溢出 */
  overflow: hidden;
  /* 确保内容不会超出卡片边界 */
  contain: layout;
}

.key-card:hover {
  box-shadow: var(--shadow-md);
  transform: translateY(-1px);
}

.key-card.status-valid {
  border-color: var(--success-border);
  background: var(--success-bg);
  border-width: 1.5px;
}

/* 模型映射专用样式 - 蓝色主题（与子分组保持一致） */
.key-card.status-model-mapping {
  border-color: #2080f0;
  background: #f0f7ff;
  border-width: 1.5px;
}

/* 暗黑模式下的模型映射样式 */
:root.dark .key-card.status-model-mapping {
  border-color: #4098fc;
  background: #1a2332;
}

/* 模型映射名称样式 */
.model-mapping-names {
  display: flex;
  align-items: baseline;
  flex: 1;
  min-width: 0;
}

.display-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.model-name {
  font-size: 13px;
  font-weight: 500;
  color: #722ed1;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  background: #f0e6ff;
  padding: 2px 6px;
  border-radius: 4px;
  white-space: nowrap;
  flex-shrink: 0;
}

/* 暗黑模式下的模型名样式 */
:root.dark .model-name {
  background: #2a2a3e;
  color: #b37feb;
}

/* 权重显示样式 */
.weight-display {
  margin: 4px 0;
  /* 允许内容在卡片内增长，但有合理的高度限制 */
  overflow-y: auto; /* 当内容过多时显示滚动条 */
  max-height: 300px; /* 设置合理的最大高度，确保编辑按钮可见 */
  /* 确保在卡片内正确显示 */
  flex: 1;
  min-height: 0; /* 允许收缩 */
  /* 美化滚动条 */
  scrollbar-width: thin;
  scrollbar-color: var(--border-color) transparent;
}

/* Webkit 滚动条样式 */
.weight-display::-webkit-scrollbar {
  width: 6px;
}

.weight-display::-webkit-scrollbar-track {
  background: transparent;
}

.weight-display::-webkit-scrollbar-thumb {
  background-color: var(--border-color);
  border-radius: 3px;
  border: 2px solid transparent;
}

.weight-display::-webkit-scrollbar-thumb:hover {
  background-color: var(--text-tertiary);
}

/* 子分组进度条容器样式 */
.subgroup-progress-bars {
  display: flex;
  flex-direction: column;
  gap: 3px;
  /* 确保内容能够正确展开 */
  height: auto;
  min-height: 0; /* 允许收缩 */
  /* 确保内容不会溢出 */
  overflow-y: auto;
}

.target-progress-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  min-height: 22px; /* 确保每个进度项有最小高度 */
}

.target-model-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.target-model-name {
  font-size: 10px;
  color: var(--text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  background: var(--bg-secondary);
  padding: 2px 6px; /* 减少内边距，与弹窗保持一致 */
  border-radius: 16px; /* 调整为圆角标签样式 */
  white-space: normal; /* 允许换行 */
  word-wrap: break-word;
  word-break: break-all;
  width: 100%;
  display: inline-flex; /* 改为 flex 布局以更好地控制对齐 */
  align-items: center;
  font-weight: 500;
  min-height: 20px; /* 设置最小高度与弹窗保持一致 */
  border: 1px solid var(--border-color); /* 添加边框以与弹窗样式保持一致 */
  transition: all 0.2s ease; /* 添加过渡效果 */
  line-height: 1.3; /* 设置合适的行高 */
}

.target-percentage {
  font-size: 11px;
  color: var(--text-secondary);
  flex-shrink: 0;
  margin-left: 8px;
}

/* 测试按钮样式 */
.target-test-button {
  margin-left: 8px;
  padding: 0 6px;
  height: 18px;
  font-size: 10px;
  flex-shrink: 0;
  font-weight: 500;
}

.target-test-button :deep(.n-button__content) {
  display: flex;
  align-items: center;
  gap: 3px;
}

/* 测试按钮样式优化 */
.target-test-button {
  margin-left: 8px;
  padding: 0 6px;
  height: 18px;
  font-size: 10px;
  flex-shrink: 0;
  font-weight: 500;
  min-width: 60px; /* 确保有足够空间显示文字 */
  /* 确保按钮动画不受外部影响 */
  transition: all 0.2s ease !important;
  /* 防止全局loading覆盖按钮样式 */
  pointer-events: auto !important;
}

.target-test-button :deep(.n-button__content) {
  display: flex;
  align-items: center;
  gap: 2px;
  transition: inherit;
}

.target-test-button :deep(.n-icon) {
  font-size: 10px;
  transition: inherit;
}

/* 确保测试按钮在loading状态下的动画流畅性 */
.target-test-button :deep(.n-base-loading) {
  transition: opacity 0.2s ease;
}

/* 防止全局loading bar影响按钮的加载状态 */
.target-test-button[loading] {
  position: relative;
  z-index: 1;
}

/* 测试成功状态的样式 */
.target-test-button.n-button--success-type {
  border-color: var(--success-color);
  color: var(--success-color);
  font-weight: 500;
}

.target-test-button.n-button--success-type:hover {
  background: rgba(24, 160, 88, 0.1);
}

/* 方案1：绿色渐变系配色（以200-500ms的绿色为基准色） */
.target-test-button.test-super-fast {
  border-color: #95de64;
  color: #52c41a;
  background: rgba(149, 222, 100, 0.08);
}

.target-test-button.test-super-fast:hover {
  background: rgba(149, 222, 100, 0.18);
}

.target-test-button.test-excellent {
  border-color: #52c41a;
  color: #52c41a;
  background: rgba(82, 196, 26, 0.10);
}

.target-test-button.test-excellent:hover {
  background: rgba(82, 196, 26, 0.20);
}

.target-test-button.test-good {
  border-color: #a8e61a;
  color: #7cb342;
  background: rgba(168, 230, 26, 0.08);
}

.target-test-button.test-good:hover {
  background: rgba(168, 230, 26, 0.18);
}

/* 1000-2000ms：黄绿色过渡 */
.target-test-button.test-normal {
  border-color: #a0d911;
  color: #8b9a00;
  background: rgba(160, 217, 17, 0.08);
}

.target-test-button.test-normal:hover {
  background: rgba(160, 217, 17, 0.18);
}

.target-test-button.test-slow {
  border-color: #fa8c16;
  color: #d46b08;
  background: rgba(250, 140, 22, 0.08);
}

.target-test-button.test-slow:hover {
  background: rgba(250, 140, 22, 0.18);
}

.target-test-button.test-very-slow {
  border-color: #f5222d;
  color: #cf1322;
  background: rgba(245, 34, 45, 0.08);
}

.target-test-button.test-very-slow:hover {
  background: rgba(245, 34, 45, 0.18);
}

/* 4000-5000ms：深红色 */
.target-test-button.test-extremely-slow {
  border-color: #a8071a;
  color: #a8071a;
  background: rgba(168, 7, 26, 0.08);
}

.target-test-button.test-extremely-slow:hover {
  background: rgba(168, 7, 26, 0.18);
}

/* >5000ms：超时状态 */
.target-test-button.test-timeout {
  border-color: #595959;
  color: #595959;
  background: rgba(89, 89, 89, 0.08);
}

.target-test-button.test-timeout:hover {
  background: rgba(89, 89, 89, 0.18);
}

/* 错误状态：灰色 */
.target-test-button.test-failed {
  border-color: #8c8c8c;
  color: #8c8c8c;
  background: rgba(140, 140, 140, 0.08);
}

.target-test-button.test-failed:hover {
  background: rgba(140, 140, 140, 0.18);
}

/* 暗黑模式下的绿色渐变配色 */
:root.dark .target-test-button.test-excellent {
  border-color: #5cdb95;
  color: #5cdb95;
  background: rgba(92, 219, 149, 0.1);
}

:root.dark .target-test-button.test-very-fast {
  border-color: #5cdb95;
  color: #73d13d;
  background: rgba(115, 209, 61, 0.12);
}

:root.dark .target-test-button.test-fast {
  border-color: #95de64;
  color: #95de64;
  background: rgba(149, 222, 100, 0.1);
}

:root.dark .target-test-button.test-normal {
  border-color: #ffd666;
  color: #ffd666;
  background: rgba(255, 214, 102, 0.1);
}

:root.dark .target-test-button.test-slow {
  border-color: #ffc53d;
  color: #ffc53d;
  background: rgba(255, 197, 61, 0.1);
}

:root.dark .target-test-button.test-very-slow {
  border-color: #ffa940;
  color: #ffa940;
  background: rgba(255, 169, 64, 0.1);
}

:root.dark .target-test-button.test-extremely-slow {
  border-color: #ff7a45;
  color: #ff7a45;
  background: rgba(255, 122, 69, 0.1);
}

:root.dark .target-test-button.test-timeout {
  border-color: #ff4d4f;
  color: #ff4d4f;
  background: rgba(255, 77, 79, 0.1);
}

:root.dark .target-test-button.test-failed {
  border-color: #909399;
  color: #909399;
  background: rgba(144, 147, 153, 0.1);
}

.target-progress-bar {
  width: 100%;
  height: 8px;
  background: var(--bg-tertiary);
  border-radius: 4px;
  overflow: hidden;
}

.weight-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

/* 暗黑模式下的模型名样式 */
:root.dark .target-model-name {
  background: var(--bg-tertiary);
}

.weight-bar-container {
  display: flex;
  align-items: center;
  gap: 12px;
}

.weight-label {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.weight-label strong {
  color: var(--text-primary);
  font-weight: 600;
}

.key-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.key-section {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.key-bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-top: auto;
  padding-top: 8px;
  /* 确保操作按钮始终可见 */
  flex-shrink: 0;
  background: inherit;
  position: relative;
  z-index: 1;
}

.key-stats {
  display: flex;
  gap: 8px;
  font-size: 12px;
  overflow: hidden;
  color: var(--text-secondary);
  flex: 1;
  min-width: 0;
}

.stat-item {
  white-space: nowrap;
  color: var(--text-secondary);
}

.stat-item strong {
  color: var(--text-primary);
  font-weight: 600;
}

.key-actions {
  flex-shrink: 0;
}

.key-actions :deep(.n-button) {
  padding: 0 4px;
}

.key-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-weight: 500;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

:root:not(.dark) .key-text {
  color: #495057;
  background: #f8f9fa;
}

:root.dark .key-text {
  color: var(--text-primary);
  background: var(--bg-tertiary);
}

:deep(.n-input__input-el) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
}

.quick-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.sub-group-id {
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: 2px 6px;
  border-radius: 4px;
}

.weight-bar {
  flex: 1;
  height: 8px;
  background: var(--bg-tertiary);
  border-radius: 4px;
  overflow: hidden;
}

.weight-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

/* Active state - 绿色渐变（与子分组保持一致） */
.key-card .weight-fill-active {
  background: linear-gradient(90deg, #0e7a43, #18a058, #36ad6a, #5fd299) !important;
}

:root.dark .key-card .weight-fill-active {
  background: linear-gradient(90deg, #4aba7d, #63e2b7, #7fe7c4, #a3f5d0) !important;
}

/* Unavailable state - striped pattern (red/orange warning) */
.key-card .weight-fill-unavailable {
  background: repeating-linear-gradient(
    45deg,
    #f5a9a9,
    #f5a9a9 8px,
    #e88592 8px,
    #e88592 16px
  ) !important;
  opacity: 0.85;
}

:root.dark .key-card .weight-fill-unavailable {
  background: repeating-linear-gradient(
    45deg,
    #8b3a3a,
    #8b3a3a 8px,
    #a04848 8px,
    #a04848 16px
  ) !important;
  opacity: 0.8;
}

.weight-text {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 14px;
  min-width: 40px;
  text-align: right;
}

.pagination-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--card-bg-solid);
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;
  border-radius: 0 0 8px 8px;
}

.pagination-info {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary);
}

.pagination-controls {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-info {
  font-size: 12px;
  color: var(--text-secondary);
}

.empty-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
}

@media (max-width: 768px) {
  .toolbar {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    justify-content: space-between;
  }

  .model-mappings-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .model-mapping-names {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
  }

  .quick-actions {
    flex-shrink: 0;
  }

  .key-actions {
    flex-shrink: 0;
  }

  .target-model-name {
    font-size: 8px;
    padding: 1px 4px; /* 移动端更紧凑的内边距 */
    min-height: 18px; /* 移动端更小的高度 */
    border-radius: 12px; /* 移动端更小的圆角 */
  }

  .target-percentage {
    font-size: 10px;
    margin-left: 6px;
  }

  .target-test-button {
    margin-left: 6px;
    padding: 0 4px;
    height: 16px;
    font-size: 9px;
  }

  .target-test-button :deep(.n-icon) {
    font-size: 9px;
  }

  .target-progress-bar {
    height: 6px;
  }

  .subgroup-progress-bars {
    gap: 2px;
  }

  .weight-display {
    /* 移动端设置合理的最大高度 */
    max-height: 200px;
    overflow-y: auto;
  }

  .target-progress-item {
    min-height: 20px; /* 移动端降低最小高度 */
  }

  /* 移动端复制按钮适配 */
  .copy-model-alias-btn {
    min-width: 18px;
    width: 18px;
    height: 18px;
    margin-left: 6px;
  }

  .copy-model-alias-btn :deep(.n-icon) {
    font-size: 10px;
  }
}

/* Tooltip 样式 */
.model-mapping-info-tooltip {
  min-width: 300px;
  max-width: 90vw;
  width: auto;
  padding: 8px;
  max-height: 70vh;
  overflow-y: auto;
}

/* 移动端适配 */
@media (max-width: 768px) {
  .model-mapping-info-tooltip {
    min-width: 280px;
    max-width: 85vw;
    font-size: 12px;
  }

  .info-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .info-title {
    font-size: 13px;
  }

  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .info-label {
    min-width: auto;
    font-size: 11px;
  }

  .info-value {
    text-align: left;
    font-size: 12px;
  }

  .target-model-detail {
    font-size: 12px;
  }
}

.info-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 10px;
  margin-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.15);
}

:root:not(.dark) .info-header {
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.info-title {
  font-size: 14px;
  font-weight: 600;
  color: inherit;
}

.info-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  line-height: 1.5;
  gap: 12px;
}

.info-label {
  color: inherit;
  opacity: 0.7;
  flex-shrink: 0;
  min-width: 100px;
  font-weight: 500;
}

.info-value {
  color: inherit;
  font-weight: 500;
  text-align: right;
  word-break: break-word;
  flex: 1;
}

.target-detail {
  margin-bottom: 4px;
  font-size: 12px;
}

.target-model-detail {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
  font-size: 13px;
  opacity: 0.8;
  font-weight: 400;
}

/* 复制模型别名按钮样式 */
.copy-model-alias-btn {
  flex-shrink: 0;
  margin-left: 8px;
  padding: 2px;
  min-width: 20px;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  transition: all 0.2s ease;
}

.copy-model-alias-btn:hover {
  background: var(--bg-secondary);
  color: var(--primary-color);
}

.copy-model-alias-btn :deep(.n-icon) {
  font-size: 12px;
}

/* 暗黑模式下的复制按钮样式 */
:root.dark .copy-model-alias-btn:hover {
  background: var(--bg-tertiary);
}
</style>
