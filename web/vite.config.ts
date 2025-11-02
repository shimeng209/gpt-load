import vue from "@vitejs/plugin-vue";
import path from "path";
import { defineConfig, loadEnv } from "vite";

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, path.resolve(__dirname, "../"), "");

  return {
    plugins: [vue()],
    // 解析配置
    resolve: {
      // 配置路径别名
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    // 开发服务器配置
    server: {
      // 代理配置示例
      proxy: {
        "/api": {
          target: env.VITE_API_BASE_URL || "http://127.0.0.1:3001",
          changeOrigin: true,
        },
      },
    },
    // 构建配置
    build: {
      outDir: "dist",
      assetsDir: "assets",
      // 调整chunk大小警告阈值（默认500kb），UI库通常较大
      chunkSizeWarningLimit: 1500,
      target: "es2015",
      // 代码分割配置
      rollupOptions: {
        output: {
          // 手动分割vendor库
          manualChunks: {
            // Vue生态系统
            "vue-vendor": ["vue", "vue-router", "vue-i18n"],
            // Naive UI及其图标库
            "naive-ui": ["naive-ui", "@vicons/ionicons5"],
            // 其他第三方库
            vendor: ["axios", "@vueuse/core"],
          },
        },
      },
    },
    // 定义常量，注入版本号
    define: {
      __APP_VERSION__: JSON.stringify(env.VITE_VERSION || "1.0.0"),
    },
  };
});
