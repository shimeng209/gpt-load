import i18n from "@/locales";
import { useAuthService } from "@/services/auth";
import axios from "axios";
import { appState } from "./app-state";

// 定义不需要显示 loading 的 API 地址列表
const NO_LOADING_URLS = [
  "/tasks/status",         // 任务状态轮询
  "/keys/test-multiple",   // 测试密钥API
  "/dashboard/stats",      // 仪表盘统计数据
  "/dashboard/chart",      // 仪表盘图表数据
  "/groups/list",          // 分组列表查询
  "/groups/config-options", // 分组配置选项
  "/groups/*/stats",       // 分组统计信息
  "/groups/*/sub-groups",  // 子分组列表
  "/groups/*/parent-aggregate-groups", // 父聚合分组列表
  "/settings",             // 设置查询
  "/channel-types",        // 渠道类型查询
  "/logs",                 // 日志查询
  "/logs/*",               // 日志详情查询
];

// 定义不需要显示 loading 的 API 模式
const NO_LOADING_PATTERNS = [
  /^\/groups\/\d+\/stats$/,           // 分组统计信息
  /^\/groups\/\d+\/sub-groups$/,      // 子分组列表
  /^\/groups\/\d+\/parent-aggregate-groups$/, // 父聚合分组列表
  /^\/logs\/\d+$/,                    // 日志详情
  /^\/keys\?.*group_id=.*&status=active$/, // 密钥测试时的查询
];

declare module "axios" {
  interface AxiosRequestConfig {
    hideMessage?: boolean;
  }
}

const http = axios.create({
  baseURL: "/api",
  timeout: 60000,
  headers: { "Content-Type": "application/json" },
});

// 请求拦截器
http.interceptors.request.use(config => {
  // 检查当前请求的 URL 是否在屏蔽列表中
  let shouldSkipLoading = false;

  if (config.url) {
    // 精确匹配不需要loading的API
    if (NO_LOADING_URLS.includes(config.url)) {
      shouldSkipLoading = true;
    }
    // 通配符匹配
    else if (NO_LOADING_URLS.some(pattern => {
      // 将通配符转换为正则表达式
      const regex = new RegExp('^' + pattern.replace(/\*/g, '[^/]+') + '$');
      return regex.test(config.url!);
    })) {
      shouldSkipLoading = true;
    }
    // 正则模式匹配
    else if (NO_LOADING_PATTERNS.some(pattern => pattern.test(config.url!))) {
      shouldSkipLoading = true;
    }
    // 对于 /keys 接口，只有特定参数下才跳过loading（模型测试时）
    else if (config.url === "/keys" && config.params?.group_id && config.params?.status === "active") {
      shouldSkipLoading = true;
    }
    // 所有GET请求默认不显示全局loading（查询操作）
    else if (config.method?.toLowerCase() === 'get') {
      shouldSkipLoading = true;
    }
  }

  if (!shouldSkipLoading) {
    appState.loading = true;
  }

  const authKey = localStorage.getItem("authKey");
  if (authKey) {
    config.headers.Authorization = `Bearer ${authKey}`;
  }
  // 添加语言头
  const locale = localStorage.getItem("locale") || "zh-CN";
  config.headers["Accept-Language"] = locale;
  return config;
});

// 响应拦截器
http.interceptors.response.use(
  response => {
    appState.loading = false;
    if (response.config.method !== "get" && !response.config.hideMessage) {
      window.$message.success(response.data.message ?? i18n.global.t("common.operationSuccess"));
    }
    return response.data;
  },
  error => {
    appState.loading = false;
    if (error.response) {
      if (error.response.status === 401) {
        if (window.location.pathname !== "/login") {
          const { logout } = useAuthService();
          logout();
          window.location.href = "/login";
        }
      }
      window.$message.error(
        error.response.data?.message ||
          i18n.global.t("common.requestFailed", { status: error.response.status }),
        {
          keepAliveOnHover: true,
          duration: 5000,
          closable: true,
        }
      );
    } else if (error.request) {
      window.$message.error(i18n.global.t("common.networkError"));
    } else {
      window.$message.error(i18n.global.t("common.requestSetupError"));
    }
    return Promise.reject(error);
  }
);

export default http;
