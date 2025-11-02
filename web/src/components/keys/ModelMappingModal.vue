<script setup lang="ts">
import { keysApi } from "@/api/keys";
import BaseModal from "@/components/common/BaseModal.vue";
import useApi from "@/composables/useApi";
import useLoading from "@/composables/useLoading";
import type { Group, ModelMapping, ModelMappingTargetConfig, SubGroupInfo } from "@/types/models";
import { getGroupDisplayName } from "@/utils/display";
import { Add, Close } from "@vicons/ionicons5";
import {
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NSelect,
  type FormRules,
} from "naive-ui";
import { computed, nextTick, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  mode: 'add' | 'edit';
  aggregateGroup: Group | null;
  modelMapping?: ModelMapping | null; // 编辑模式下传入
  existingModelMappings: ModelMapping[];
  subGroups: SubGroupInfo[];
  groups: Group[];
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "success"): void;
}

interface TargetItem extends ModelMappingTargetConfig {
  id: string;
  editingModel?: string | null; // 正在编辑的模型名称
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const { t } = useI18n();
const { handleApiError, handleApiSuccess } = useApi();
const { loading, withLoading } = useLoading();
const formRef = ref();

// 表单数据
const formData = reactive<{
  model: string;
  targets: TargetItem[];
}>({
  model: "",
  targets: [{ id: "1", sub_group_id: 0, weight: 1, model: "" }],
});

// 原始模型别名（用于检查重复）
const originalModelName = ref("");

// 计算弹窗标题
const modalTitle = computed(() => {
  return props.mode === 'edit'
    ? t('modelMappings.editModelMapping')
    : t('modelMappings.addModelMapping');
});

// 计算可用的子分组选项
const getAvailableSubGroups = computed(() => {
  if (!props.subGroups) {
    return [];
  }

  return props.subGroups.map(subGroup => ({
    label: getGroupDisplayName(subGroup.group),
    value: subGroup.group.id,
  }));
});

// 获取指定索引的选项（移除唯一选择限制，允许重复选择子分组）
function getOptionsForTarget(_index: number) {
  return getAvailableSubGroups.value;
}

// 表单验证规则
const rules: FormRules = {
  model: {
    required: true,
    message: t("modelMappings.modelAliasRequired"),
    trigger: ["blur", "input"],
    validator: (_rule, value) => {
      if (!value || !value.trim()) {
        return new Error(t("modelMappings.modelAliasRequired"));
      }

      // 检查是否与现有模型映射重复
      const isDuplicate = props.existingModelMappings.some(
        mapping => {
          // 编辑模式下排除当前编辑的映射
          if (props.mode === 'edit' && mapping.model === originalModelName.value) {
            return false;
          }
          return mapping.model.toLowerCase() === value.trim().toLowerCase();
        }
      );

      if (isDuplicate) {
        return new Error(t("modelMappings.duplicateModelAlias"));
      }

      return true;
    },
  },
  targets: {
    type: "array",
    required: true,
    validator: (_rule, value: TargetItem[]) => {
      // 检查是否至少有一个有效的目标
      const validTargets = value.filter(target => {
        const hasValidSubGroup = target.sub_group_id > 0;
        const hasValidModel = target.model && target.model.trim() !== "";
        return hasValidSubGroup && hasValidModel;
      });
      if (validTargets.length === 0) {
        return new Error(t("modelMappings.atLeastOneTarget"));
      }

      // 检查权重是否合法
      for (const target of validTargets) {
        if (target.weight < 0) {
          return new Error(t("modelMappings.invalidWeight"));
        }
      }

      // 前端不再验证重复，让后端处理
      console.log(`${props.mode === 'edit' ? '编辑' : '添加'}提交的目标配置:`, validTargets);
      return true;
    },
    trigger: ["blur", "change"],
  },
};

// 监听弹窗显示状态和数据变化
watch([() => props.show, () => props.modelMapping, () => props.mode], ([show, modelMapping, mode]) => {
  if (show) {
    if (modelMapping && (mode === 'edit' || (mode === 'add' && modelMapping.model && modelMapping.targets.length > 0))) {
      // 编辑模式或有预填充数据的添加模式都加载数据
      loadModelMappingData();
    } else {
      resetForm();
    }
  }
});

// 重置表单
function resetForm() {
  formData.model = "";
  // 使用第一个子分组的权重作为默认值，如果没有则使用1
  const defaultWeight = props.subGroups && props.subGroups.length > 0 ? props.subGroups[0].weight : 1;
  formData.targets = [{ id: "1", sub_group_id: 0, weight: defaultWeight, model: "" }];
  originalModelName.value = "";
}

// 加载模型映射数据到表单（编辑模式）
function loadModelMappingData() {
  if (!props.modelMapping) {
    return;
  }

  originalModelName.value = props.modelMapping.model;
  formData.model = props.modelMapping.model;
  formData.targets = props.modelMapping.targets.map((target, index) => ({
    ...target,
    id: (index + 1).toString(),
  }));
}

// 添加目标项
function addTargetItem() {
  const newId = (Math.max(...formData.targets.map(t => parseInt(t.id))) + 1).toString();
  // 使用第一个子分组的权重作为默认值，如果没有则使用1
  const defaultWeight = props.subGroups && props.subGroups.length > 0 ? props.subGroups[0].weight : 1;
  formData.targets.push({
    id: newId,
    sub_group_id: 0,
    weight: defaultWeight,
    model: "",
  });
}

// 删除目标项
function removeTargetItem(id: string) {
  if (formData.targets.length > 1) {
    formData.targets = formData.targets.filter(t => t.id !== id);
  }
}

// 关闭弹窗
function handleClose() {
  emit("update:show", false);
}

// 取消操作
function handleCancel() {
  emit("update:show", false);
}

// 提交表单
async function handleSubmit() {
  if (loading.value || !props.aggregateGroup?.id) {
    return;
  }

  await withLoading(async () => {
    await formRef.value?.validate();

    if (!props.aggregateGroup?.id) {
      throw new Error("Aggregate group not found");
    }

    // 过滤出有效的目标配置
    const validTargets = formData.targets
      .filter(target => {
        const hasValidSubGroup = target.sub_group_id > 0;
        const hasValidModel = target.model && target.model.trim() !== "";
        return hasValidSubGroup && hasValidModel;
      })
      .map(target => ({
        sub_group_id: target.sub_group_id,
        weight: target.weight,
        model: target.model.trim(),
      }));

    if (validTargets.length === 0) {
      handleApiError(t("modelMappings.targetsRequired"));
      throw new Error(t("modelMappings.targetsRequired"));
    }

    // 构建模型映射
    const newMapping: ModelMapping = {
      model: formData.model.trim(),
      targets: validTargets,
    };

    let updatedMappings: ModelMapping[];

    if (props.mode === 'edit') {
      // 编辑模式：更新现有映射
      updatedMappings = props.existingModelMappings.map(mapping =>
        mapping.model === props.modelMapping?.model ? newMapping : mapping
      );
    } else {
      // 添加模式：添加新映射
      updatedMappings = [...props.existingModelMappings, newMapping];
    }

    const response = await keysApi.updateGroup(props.aggregateGroup.id, {
      model_mappings: updatedMappings,
    });

    console.log("Update group response:", response);

    // 检查响应格式，如果是数组则取第一个元素
    if (Array.isArray(response)) {
      console.log("Response is array, taking first element:", response[0]);
    }

    handleApiSuccess();
    emit("success");
    handleClose();
  });
}

// 根据子分组ID自动填充默认模型名称和权重
function handleSubGroupChange(targetId: string, subGroupId: number) {
  const target = formData.targets.find(t => t.id === targetId);
  if (target) {
    // 同步子分组的权重
    const subGroup = props.subGroups.find(sg => sg.group.id === subGroupId);
    if (subGroup) {
      target.weight = subGroup.weight;
      // 如果模型名称为空，自动填充子分组的测试模型
      if (!target.model.trim() && subGroup.group.test_model) {
        target.model = subGroup.group.test_model;
      }
    }
  }
}

// 编辑模型
function editModel(target: TargetItem, event: MouseEvent) {
  target.editingModel = target.model;
  // 使用 nextTick 确保 DOM 更新后再聚焦
  nextTick(() => {
    // 使用事件的目标元素来找到对应的输入框
    const targetElement = event.currentTarget as HTMLElement;
    const input = targetElement?.querySelector(".model-config-input") as HTMLInputElement;
    if (input) {
      input.focus();
      // 选中全部文本方便修改
      input.select();
    }
  });
}

// 完成编辑
function finishEdit(target: TargetItem) {
  if (target.editingModel !== undefined && target.editingModel !== null) {
    const newModelName = target.editingModel.trim();
    target.model = newModelName;
  }
  target.editingModel = null;
}

// 取消编辑
function cancelEdit(target: TargetItem) {
  target.editingModel = null;
}

// 检查是否为移动端
const isMobile = computed(() => {
  return typeof window !== 'undefined' && window.innerWidth <= 768;
});

// 计算弹窗的动态样式
const modalStyle = computed(() => {
  if (isMobile.value) {
    // 手机端适配
    return {
      width: "95vw",
      height: "80vh",
      maxWidth: "95vw",
      maxHeight: "80vh",
      margin: "0",
    };
  }

  // 桌面端：使用统一的宽度，不再根据目标数量动态调整
  // 统一添加和编辑模式的弹窗大小，确保界面一致性
  let width = 950; // 统一的宽度，既能容纳多个目标又不会过宽
  let height = 640; // 统一的高度

  return {
    width: `${width}px`,
    height: `${height}px`,
    maxWidth: "95vw", // 确保在小屏幕上不超过视窗宽度
  };
});
</script>

<template>
  <BaseModal
    :show="show"
    @update:show="handleClose"
    @confirm="handleSubmit"
    @cancel="handleCancel"
    :title="modalTitle"
    :loading="loading"
    :width="modalStyle.width"
    :height="modalStyle.height"
    :max-width="modalStyle.maxWidth"
    :max-height="modalStyle.maxHeight"
    modal-class="model-mapping-modal"
    card-class="model-mapping-card"
  >
    <n-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      :label-placement="isMobile ? 'top' : 'left'"
      :label-width="isMobile ? undefined : '100px'"
      :class="{ 'mobile-form': isMobile }"
    >
      <div class="form-section">
        <n-form-item
          :label="t('modelMappings.modelAlias')"
          path="model"
          :show-feedback="false"
          class="model-alias-form-item"
        >
          <n-input
            v-model:value="formData.model"
            :placeholder="t('modelMappings.modelAliasPlaceholder')"
            clearable
          />
        </n-form-item>
      </div>

      <div class="form-section">
        <h4 class="section-title">
          {{ t("modelMappings.targetConfig") }}
        </h4>

        <div class="targets-list">
          <div v-for="target in formData.targets" :key="target.id" class="target-item">
            <!-- 单行布局 -->
            <div class="target-row">
              <span class="target-label">
                {{ t("modelMappings.targetSubGroup") }} {{ target.id }}
              </span>

              <n-form-item
                class="item-select"
                :path="`targets[${formData.targets.indexOf(target)}].sub_group_id`"
                :show-feedback="false"
              >
                <n-select
                  :value="target.sub_group_id || null"
                  :options="getOptionsForTarget(formData.targets.indexOf(target))"
                  :placeholder="t('subGroups.selectSubGroup')"
                  clearable
                  @update:value="
                    value => {
                      target.sub_group_id = value || 0;
                      handleSubGroupChange(target.id, value || 0);
                    }
                  "
                />
              </n-form-item>

              <n-form-item
                class="item-weight"
                :path="`targets[${formData.targets.indexOf(target)}].weight`"
                :show-feedback="false"
              >
                <n-input-number
                  v-model:value="target.weight"
                  :min="0"
                  :max="100"
                  :step="1"
                  :placeholder="t('keys.weight')"
                  size="small"
                />
              </n-form-item>

              <!-- 实际模型配置区域 -->
              <n-form-item class="item-models" :show-feedback="false">
                <div class="model-config-container">
                  <div
                    class="model-config-tag"
                    :class="{ 'editing': target.editingModel !== undefined && target.editingModel !== null }"
                    @click="editModel(target, $event)"
                  >
                    <input
                      v-if="target.editingModel !== undefined && target.editingModel !== null"
                      v-model="target.editingModel"
                      @blur="finishEdit(target)"
                      @keyup.enter="finishEdit(target)"
                      @keyup.esc="cancelEdit(target)"
                      class="model-config-input"
                      ref="modelInput"
                    />
                    <span v-else>{{ target.model || t("modelMappings.clickToEditModel") }}</span>
                  </div>
                </div>
              </n-form-item>

              <n-button
                @click="removeTargetItem(target.id)"
                type="error"
                quaternary
                circle
                size="small"
                class="item-delete"
                :style="{ visibility: formData.targets.length > 1 ? 'visible' : 'hidden' }"
              >
                <template #icon>
                  <n-icon :component="Close" />
                </template>
              </n-button>
            </div>
          </div>
        </div>

        <div class="add-item-section">
          <n-button @click="addTargetItem" dashed style="width: 100%">
            <template #icon>
              <n-icon :component="Add" />
            </template>
            {{ t("modelMappings.addTarget") }}
          </n-button>
        </div>
      </div>
    </n-form>
  </BaseModal>
</template>

<style scoped>
.form-section {
  margin-top: 0;
  margin-bottom: 16px;
}

.form-section:first-child {
  margin-top: 0;
  margin-bottom: 12px;
}

.section-title {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border-color);
}

.model-alias-form-item {
  margin-bottom: 8px;
}

.targets-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 20px;
}

.target-item {
  padding: 12px 50px 12px 16px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius-md);
  transition: all 0.2s ease;
  position: relative;
}

.target-row {
  display: flex;
  align-items: center;
  gap: 6px; /* 减少间距 */
  min-height: 32px;
}

.target-label {
  font-weight: 500;
  color: var(--text-primary);
  font-size: 0.9rem;
  flex-shrink: 0;
  width: 100px; /* 大幅减少标签宽度 */
  height: 32px; /* 与控件高度保持一致 */
  display: flex;
  align-items: center;
  /* 紧凑布局 */
  margin-right: 4px;
}

.target-item:hover {
  border-color: var(--primary-color);
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.1);
}

.item-select {
  flex: 1;
  min-width: 150px; /* 减少最小宽度 */
}

.item-select :deep(.n-form-item-feedback-wrapper) {
  display: none; /* 隐藏反馈信息以保持32px高度 */
}

.item-select :deep(.n-base-selection) {
  height: 32px;
  min-height: 32px;
  /* 让选择框更紧凑 */
  padding-left: 8px;
  padding-right: 8px;
}

.item-select :deep(.n-base-selection .n-base-selection-label) {
  padding: 0 4px;
}

.item-weight {
  width: 80px; /* 恢复原来的宽度以确保数值完整显示 */
  flex-shrink: 0;
}

.item-weight :deep(.n-form-item-feedback-wrapper) {
  display: none; /* 隐藏反馈信息以保持32px高度 */
}

.item-weight :deep(.n-input-number) {
  height: 32px;
  min-height: 32px;
}

.item-weight :deep(.n-input-number .n-input__input) {
  height: 32px;
  line-height: 32px;
}

.item-models {
  flex: 2;
  min-width: 180px; /* 进一步减少最小宽度以适应紧凑布局 */
}

.item-models :deep(.n-form-item-feedback-wrapper) {
  display: none; /* 隐藏反馈信息以保持32px高度 */
}

.model-config-container {
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 32px;
}

.label-text {
  color: var(--text-primary);
  font-size: 0.9rem;
  font-weight: 500;
}

.model-config-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  padding: 0 8px; /* 减少内边距 */
  background: var(--bg-tertiary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  font-size: 0.85rem;
  font-weight: 500;
  transition: all 0.2s ease;
  cursor: text;
  /* 减少最小宽度 */
  min-width: 150px;
  max-width: 100%;
  flex: 1;
}

/* 编辑模式下的样式 */
.model-config-tag.editing {
  height: auto;
  min-height: 32px;
  padding: 8px 12px;
  background: var(--bg-primary);
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px var(--primary-color-20);
}

.model-config-tag:hover {
  background: var(--bg-secondary);
  border-color: var(--primary-color);
}

.model-config-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 0.85rem;
  font-weight: 500;
  outline: none;
  width: 100%;
  min-width: 120px; /* 减少最小宽度 */
  /* 编辑时的样式 */
  padding: 2px 4px;
  border-radius: 4px;
  background: var(--bg-secondary);
  /* 确保输入框足够大以显示完整内容 */
  min-height: 24px;
  line-height: 1.4;
}

.model-config-input:focus {
  background: var(--bg-primary);
  box-shadow: 0 0 0 1px var(--primary-color);
}

.model-config-input::placeholder {
  color: var(--text-tertiary);
}

.item-delete {
  flex-shrink: 0;
  margin-left: 8px;
}

.item-delete :deep(.n-button) {
  width: 32px;
  height: 32px;
  min-height: 32px;
}

.add-item-section {
  margin-top: 16px;
}

/* 响应式适配 */
@media (max-width: 768px) {
  .mobile-form {
    padding: 8px;
  }

  .mobile-form .form-section {
    margin-bottom: 20px;
  }

  .mobile-form .model-alias-form-item {
    margin-bottom: 16px;
  }

  .target-item {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
    padding: 16px;
    margin-bottom: 12px;
  }

  .target-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
    min-height: auto;
  }

  .target-label {
    width: 100%;
    text-align: left;
    margin-bottom: 4px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .item-select,
  .item-weight,
  .item-models {
    width: 100%;
    min-width: auto;
    flex: none;
  }

  .item-select {
    order: 1;
  }

  .item-weight {
    order: 2;
  }

  .item-models {
    order: 3;
  }

  .item-delete {
    order: 4;
    align-self: flex-end;
    margin-top: 8px;
  }

  .model-config-container {
    width: 100%;
    justify-content: stretch;
  }

  .model-config-tag {
    width: 100%;
    justify-content: stretch;
    min-height: 40px;
    padding: 8px 16px;
    /* 移动端允许更多空间 */
    min-width: auto;
  }

  .model-config-tag.editing {
    min-height: 48px;
    padding: 12px 16px;
  }

  .model-config-input {
    text-align: left;
    min-width: auto;
    width: 100%;
    /* 移动端更大的输入框 */
    min-height: 32px;
    padding: 6px 8px;
    font-size: 0.9rem;
  }

  .add-item-section {
    margin-top: 20px;
  }

  }

/* 超小屏幕适配 */
@media (max-width: 480px) {
  .mobile-form {
    padding: 4px;
  }

  .target-item {
    padding: 12px;
  }

  .model-config-tag {
    min-height: 44px;
    font-size: 0.9rem;
  }
}

/* 暗黑模式适配 */
:root.dark .target-item {
  background: var(--bg-tertiary);
  border-color: var(--border-color);
}
</style>