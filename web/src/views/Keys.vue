<script setup lang="ts">
import { keysApi } from "@/api/keys";
import EncryptionMismatchAlert from "@/components/EncryptionMismatchAlert.vue";
import GroupInfoCard from "@/components/keys/GroupInfoCard.vue";
import GroupList from "@/components/keys/GroupList.vue";
import KeyTable from "@/components/keys/KeyTable.vue";
import ModelMappingTable from "@/components/keys/ModelMappingTable.vue";
import SubGroupTable from "@/components/keys/SubGroupTable.vue";
import type { Group, ModelMapping, SubGroupInfo } from "@/types/models";
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

const groups = ref<Group[]>([]);
const loading = ref(false);
const selectedGroup = ref<Group | null>(null);
const subGroups = ref<SubGroupInfo[]>([]);
const modelMappings = ref<ModelMapping[]>([]);
const loadingSubGroups = ref(false);
const router = useRouter();
const route = useRoute();

onMounted(async () => {
  await loadGroups();
});

async function loadGroups() {
  try {
    loading.value = true;
    groups.value = await keysApi.getGroups();
    // 选择默认分组
    if (groups.value.length > 0 && !selectedGroup.value) {
      const groupId = route.query.groupId;
      const found = groups.value.find(g => String(g.id) === String(groupId));
      if (found) {
        selectedGroup.value = found;
      } else {
        handleGroupSelect(groups.value[0]);
      }
    }
  } catch (error) {
    console.error("Failed to load groups:", error);
    window.$message?.error("加载分组列表失败");
  } finally {
    loading.value = false;
  }
}

async function loadSubGroups() {
  if (!selectedGroup.value?.id || selectedGroup.value.group_type !== "aggregate") {
    subGroups.value = [];
    return;
  }

  try {
    loadingSubGroups.value = true;
    subGroups.value = await keysApi.getSubGroups(selectedGroup.value.id);
  } catch (error) {
    console.error("Failed to load sub groups:", error);
    window.$message?.error("加载子分组失败");
    subGroups.value = [];
  } finally {
    loadingSubGroups.value = false;
  }
}

// 加载模型映射数据
function loadModelMappings() {
  if (!selectedGroup.value?.id || selectedGroup.value.group_type !== "aggregate") {
    modelMappings.value = [];
    return;
  }

  // 从选中的分组中获取模型映射数据
  modelMappings.value = selectedGroup.value.model_mappings || [];
}

// 监听选中分组变化，加载子分组数据和模型映射数据
watch(selectedGroup, async newGroup => {
  if (newGroup?.group_type === "aggregate") {
    await loadSubGroups();
    loadModelMappings();
  } else {
    subGroups.value = [];
    modelMappings.value = [];
  }
});

function handleGroupSelect(group: Group | null) {
  selectedGroup.value = group || null;
  if (String(group?.id) !== String(route.query.groupId)) {
    router.push({ name: "keys", query: { groupId: group?.id || "" } });
  }
}

async function refreshGroupsAndSelect(targetGroupId?: number, selectFirst = true) {
  await loadGroups();

  if (targetGroupId) {
    const targetGroup = groups.value.find(g => g.id === targetGroupId);
    if (targetGroup) {
      handleGroupSelect(targetGroup);
      return;
    }
  }

  if (selectedGroup.value) {
    const currentGroup = groups.value.find(g => g.id === selectedGroup.value?.id);
    if (currentGroup) {
      handleGroupSelect(currentGroup);
      if (currentGroup.group_type === "aggregate") {
        await loadSubGroups();
        loadModelMappings();
      }
      return;
    }
  }

  if (selectFirst && groups.value.length > 0) {
    handleGroupSelect(groups.value[0]);
  }
}

// 处理子分组选择，跳转到对应的分组
function handleSubGroupSelect(groupId: number) {
  const targetGroup = groups.value.find(g => g.id === groupId);
  if (targetGroup) {
    handleGroupSelect(targetGroup);
  }
}

// 处理聚合分组跳转，跳转到对应的聚合分组
function handleNavigateToGroup(groupId: number) {
  const targetGroup = groups.value.find(g => g.id === groupId);
  if (targetGroup) {
    handleGroupSelect(targetGroup);
  }
}

// 处理模型映射刷新
async function handleModelMappingRefresh() {
  if (selectedGroup.value?.group_type === "aggregate") {
    // 重新加载分组数据以获取最新的模型映射
    await loadGroups();
    const currentGroup = groups.value.find(g => g.id === selectedGroup.value?.id);
    if (currentGroup) {
      handleGroupSelect(currentGroup);
    }
  }
}
</script>

<template>
  <div>
    <!-- 加密配置错误警告 -->
    <encryption-mismatch-alert style="margin-bottom: 16px" />

    <div class="keys-container">
      <div class="sidebar">
        <group-list
          :groups="groups"
          :selected-group="selectedGroup"
          :loading="loading"
          @group-select="handleGroupSelect"
          @refresh="() => refreshGroupsAndSelect()"
          @refresh-and-select="id => refreshGroupsAndSelect(id)"
        />
      </div>

      <!-- 右侧主内容区域，占80% -->
      <div class="main-content">
        <!-- 分组信息卡片，更紧凑 -->
        <div class="group-info">
          <group-info-card
            :group="selectedGroup"
            :groups="groups"
            :sub-groups="subGroups"
            @refresh="() => refreshGroupsAndSelect()"
            @delete="() => refreshGroupsAndSelect(undefined, true)"
            @copy-success="group => refreshGroupsAndSelect(group.id)"
            @navigate-to-group="handleNavigateToGroup"
          />
        </div>

        <!-- 密钥表格区域 / 模型映射区域 / 子分组列表区域 -->
        <div class="key-table-section">
          <!-- 标准分组显示密钥列表 -->
          <key-table
            v-if="!selectedGroup || selectedGroup.group_type !== 'aggregate'"
            :selected-group="selectedGroup"
          />

          <!-- 聚合分组显示模型映射和子分组列表 -->
          <div v-else class="aggregate-sections">
            <!-- 模型映射管理区域 -->
            <model-mapping-table
              :selected-group="selectedGroup"
              :model-mappings="modelMappings"
              :sub-groups="subGroups"
              :groups="groups"
              :loading="loadingSubGroups"
              @refresh="handleModelMappingRefresh"
              @group-select="handleSubGroupSelect"
            />

            <!-- 子分组列表区域 -->
            <sub-group-table
              :selected-group="selectedGroup"
              :sub-groups="subGroups"
              :groups="groups"
              :loading="loadingSubGroups"
              @refresh="loadSubGroups"
              @group-select="handleSubGroupSelect"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.keys-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.sidebar {
  width: 100%;
  flex-shrink: 0;
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.group-info {
  flex-shrink: 0;
}

.key-table-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: auto; /* 改为 auto，允许根据内容增长 */
  height: auto; /* 允许高度增长 */
  overflow: visible; /* 确保内容可见 */
}

@media (min-width: 768px) {
  .keys-container {
    flex-direction: row;
  }

  .sidebar {
    width: 240px;
    height: calc(100vh - 159px);
  }
}

/* 聚合分组的区域布局 */
.aggregate-sections {
  display: flex;
  flex-direction: column;
  gap: 16px;
  height: auto; /* 允许根据内容增长 */
  min-height: auto; /* 移除最小高度限制 */
  overflow: visible; /* 确保内容可见 */
}

.aggregate-sections > * {
  flex: 0 1 auto;
  min-height: auto;
  /* 允许子组件根据内容增长 */
  height: auto;
}
</style>
