<script setup lang="ts">
import { Close } from "@vicons/ionicons5";
import {
  NButton,
  NCard,
  NIcon,
  NModal,
} from "naive-ui";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

interface Props {
  show: boolean;
  title?: string;
  confirmText?: string;
  cancelText?: string;
  loading?: boolean;
  width?: string | number;
  maxWidth?: string;
  height?: string | number;
  maxHeight?: string;
  modalClass?: string;
  cardClass?: string;
  showCloseButton?: boolean;
  closable?: boolean;
  maskClosable?: boolean;
  preventCloseOnLoading?: boolean;
  footer?: boolean;
}

interface Emits {
  (e: "update:show", value: boolean): void;
  (e: "confirm"): void;
  (e: "cancel"): void;
  (e: "close"): void;
}

const props = withDefaults(defineProps<Props>(), {
  title: "",
  confirmText: "",
  cancelText: "",
  loading: false,
  width: "800px",
  maxWidth: "95vw",
  height: "auto",
  maxHeight: "80vh",
  modalClass: "",
  cardClass: "",
  showCloseButton: true,
  closable: true,
  maskClosable: true,
  preventCloseOnLoading: true,
  footer: true,
});

const emit = defineEmits<Emits>();

const { t } = useI18n();

// 计算确认按钮文本
const confirmButtonText = computed(() => {
  return props.confirmText || t("common.confirm");
});

// 计算取消按钮文本
const cancelButtonText = computed(() => {
  return props.cancelText || t("common.cancel");
});

// 计算弹窗样式
const modalStyle = computed(() => {
  const style: Record<string, string> = {};

  if (typeof props.width === 'number') {
    style.width = `${props.width}px`;
  } else if (props.width) {
    style.width = props.width;
  }

  if (typeof props.height === 'number') {
    style.height = `${props.height}px`;
  } else if (props.height && props.height !== 'auto') {
    style.height = props.height;
  }

  if (props.maxWidth) {
    style.maxWidth = props.maxWidth;
  }

  if (props.maxHeight) {
    style.maxHeight = props.maxHeight;
  }

  return style;
});

// 计算是否可以关闭
const canClose = computed(() => {
  return !props.loading || !props.preventCloseOnLoading;
});

// 处理关闭弹窗
function handleClose() {
  if (!canClose.value) {
    return;
  }
  emit("update:show", false);
  emit("close");
}

// 处理取消
function handleCancel() {
  if (!canClose.value) {
    return;
  }
  emit("cancel");
  handleClose();
}

// 处理确认
function handleConfirm() {
  if (props.loading) {
    return;
  }
  emit("confirm");
}

</script>

<template>
  <n-modal
    :show="show"
    @update:show="handleClose"
    :mask-closable="maskClosable && canClose"
    :class="['base-modal', modalClass]"
  >
    <n-card
      :class="['base-modal-card', cardClass]"
      :title="title"
      :bordered="false"
      size="huge"
      role="dialog"
      aria-modal="true"
      :style="modalStyle"
      :closable="false"
    >
      <!-- 头部额外按钮 -->
      <template #header-extra v-if="showCloseButton">
        <n-button
          quaternary
          circle
          @click="handleClose"
          :disabled="!canClose"
        >
          <template #icon>
            <n-icon :component="Close" />
          </template>
        </n-button>
      </template>

      <!-- 默认插槽 - 表单内容 -->
      <div class="base-modal-content">
        <slot></slot>
      </div>

      <!-- 底部操作按钮 -->
      <template #footer v-if="footer">
        <div class="base-modal-footer">
          <n-button @click="handleCancel" :disabled="!canClose">
            {{ cancelButtonText }}
          </n-button>
          <n-button
            type="primary"
            @click="handleConfirm"
            :loading="loading"
            :disabled="loading"
          >
            {{ confirmButtonText }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.base-modal {
  /* 基础modal样式 */
}

.base-modal-card {
  border-radius: var(--border-radius-lg) !important;
  box-shadow: var(--shadow-lg) !important;
  transition: all 0.3s ease !important;
}

.base-modal-card :deep(.n-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 16px 24px;
  background: var(--card-bg-solid);
  border-radius: var(--border-radius-lg) var(--border-radius-lg) 0 0;
}

.base-modal-card :deep(.n-card-header__main) {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.base-modal-content {
  max-height: calc(80vh - 140px); /* 减去头部和底部的高度 */
  overflow-y: auto;
  padding: 0;
}

.base-modal-card :deep(.n-card__content) {
  padding: 24px;
  max-height: v-bind('props.maxHeight || "80vh"');
  overflow-y: auto;
}

.base-modal-card :deep(.n-card__footer) {
  border-top: 1px solid var(--border-color);
  padding: 16px 24px;
  background: var(--card-bg-solid);
  border-radius: 0 0 var(--border-radius-lg) var(--border-radius-lg);
}

.base-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  align-items: center;
}

.base-modal-footer :deep(.n-button) {
  min-width: 80px;
  font-weight: 500;
  border-radius: var(--border-radius-md);
}

/* 响应式适配 */
@media (max-width: 768px) {
  .base-modal-card {
    width: 95vw !important;
    max-width: 95vw !important;
    margin: 0;
    border-radius: var(--border-radius-md) !important;
  }

  .base-modal-card :deep(.n-card__header) {
    padding: 12px 16px;
  }

  .base-modal-card :deep(.n-card__content) {
    padding: 16px;
    max-height: 70vh;
  }

  .base-modal-card :deep(.n-card__footer) {
    padding: 12px 16px;
  }

  .base-modal-footer {
    flex-direction: column-reverse;
    gap: 8px;
  }

  .base-modal-footer :deep(.n-button) {
    width: 100%;
    margin: 0;
  }
}

/* 超小屏幕适配 */
@media (max-width: 480px) {
  .base-modal-card {
    width: 98vw !important;
    max-width: 98vw !important;
  }

  .base-modal-card :deep(.n-card__header) {
    padding: 10px 12px;
  }

  .base-modal-card :deep(.n-card__content) {
    padding: 12px;
    max-height: 65vh;
  }

  .base-modal-card :deep(.n-card__footer) {
    padding: 10px 12px;
  }
}

/* 暗黑模式适配 */
:root.dark .base-modal-card {
  background: var(--card-bg-solid);
  border: 1px solid var(--border-color);
}

:root.dark .base-modal-card :deep(.n-card-header) {
  background: var(--card-bg-solid);
  border-bottom-color: var(--border-color);
}

:root.dark .base-modal-card :deep(.n-card__footer) {
  background: var(--card-bg-solid);
  border-top-color: var(--border-color);
}

/* 动画效果 */
.base-modal-card :deep(.n-card) {
  animation: modalSlideIn 0.3s ease-out;
}

@keyframes modalSlideIn {
  from {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* 加载状态样式 */
.base-modal-footer :deep(.n-button--loading) {
  position: relative;
}

.base-modal-footer :deep(.n-button--disabled) {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>