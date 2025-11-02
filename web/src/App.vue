<script setup lang="ts">
import GlobalProviders from "@/components/GlobalProviders.vue";
</script>

<template>
  <global-providers>
    <div id="app-root">
      <router-view />
    </div>
  </global-providers>
</template>

<style>
#app-root {
  width: 100%;
  overflow-x: hidden; /* 只隐藏横向溢出，允许纵向内容 */
  overflow-y: visible; /* 允许纵向内容自然增长 */
}
</style>
