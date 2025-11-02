import { useI18n } from "vue-i18n";

/**
 * 时间格式化工具函数
 */

/**
 * 格式化持续时间
 * @param ms 毫秒数
 * @returns 格式化后的时间字符串
 */
export function formatDuration(ms: number): string {
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

/**
 * 格式化相对时间
 * @param date 日期字符串或Date对象
 * @returns 格式化后的相对时间字符串
 */
export function formatRelativeTime(date: string | Date): string {
  const { t } = useI18n();
  if (!date) {
    return t("keys.never");
  }

  const dateObj = typeof date === 'string' ? new Date(date) : date;
  const now = new Date();
  const diffMs = now.getTime() - dateObj.getTime();
  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSeconds < 60) {
    return t("keys.justNow");
  } else if (diffMinutes < 60) {
    return t("keys.minutesAgo", { count: diffMinutes });
  } else if (diffHours < 24) {
    return t("keys.hoursAgo", { count: diffHours });
  } else if (diffDays < 30) {
    return t("keys.daysAgo", { count: diffDays });
  } else {
    return dateObj.toLocaleDateString();
  }
}

/**
 * 数据格式化工具函数
 */

/**
 * 安全地解析 JSON 字符串
 * @param jsonString JSON 字符串
 * @param defaultValue 默认值
 * @returns 解析后的对象或默认值
 */
export function safeParseJSON<T = any>(jsonString: string, defaultValue: T): T {
  try {
    return JSON.parse(jsonString);
  } catch {
    return defaultValue;
  }
}

/**
 * 截断字符串
 * @param str 原始字符串
 * @param maxLength 最大长度
 * @param suffix 后缀
 * @returns 截断后的字符串
 */
export function truncateString(str: string, maxLength: number, suffix: string = "..."): string {
  if (!str || str.length <= maxLength) {
    return str;
  }
  return str.substring(0, maxLength - suffix.length) + suffix;
}

/**
 * 格式化文件大小
 * @param bytes 字节数
 * @returns 格式化后的文件大小字符串
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes';

  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * 验证工具函数
 */

/**
 * 验证邮箱格式
 * @param email 邮箱地址
 * @returns 是否为有效邮箱
 */
export function isValidEmail(email: string): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(email);
}

/**
 * 验证 URL 格式
 * @param url URL 字符串
 * @returns 是否为有效 URL
 */
export function isValidUrl(url: string): boolean {
  try {
    new URL(url);
    return true;
  } catch {
    return false;
  }
}

/**
 * 验证模型名称格式
 * @param modelName 模型名称
 * @returns 是否为有效模型名称
 */
export function isValidModelName(modelName: string): boolean {
  // 基本的模型名称验证：允许字母、数字、点、下划线、连字符
  const modelNameRegex = /^[a-zA-Z0-9._-]+$/;
  return modelNameRegex.test(modelName) && modelName.length > 0 && modelName.length <= 100;
}

/**
 * 数组工具函数
 */

/**
 * 数组去重
 * @param array 原始数组
 * @param keyFn 可选的键提取函数
 * @returns 去重后的数组
 */
export function uniqueArray<T>(array: T[], keyFn?: (item: T) => any): T[] {
  if (!keyFn) {
    return [...new Set(array)];
  }

  const seen = new Set();
  return array.filter(item => {
    const key = keyFn(item);
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

/**
 * 分组数组
 * @param array 原始数组
 * @param keyFn 分组键提取函数
 * @returns 分组后的对象
 */
export function groupBy<T>(array: T[], keyFn: (item: T) => string): Record<string, T[]> {
  return array.reduce((groups, item) => {
    const key = keyFn(item);
    if (!groups[key]) {
      groups[key] = [];
    }
    groups[key].push(item);
    return groups;
  }, {} as Record<string, T[]>);
}

/**
 * 对象工具函数
 */

/**
 * 深度克隆对象
 * @param obj 要克隆的对象
 * @returns 克隆后的对象
 */
export function deepClone<T>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (obj instanceof Date) {
    return new Date(obj.getTime()) as unknown as T;
  }

  if (obj instanceof Array) {
    return obj.map(item => deepClone(item)) as unknown as T;
  }

  if (typeof obj === 'object') {
    const cloned = {} as T;
    for (const key in obj) {
      if (obj.hasOwnProperty(key)) {
        cloned[key] = deepClone(obj[key]);
      }
    }
    return cloned;
  }

  return obj;
}

/**
 * 获取嵌套对象属性
 * @param obj 对象
 * @param path 属性路径，如 'a.b.c'
 * @param defaultValue 默认值
 * @returns 属性值或默认值
 */
export function getNestedProperty<T = any>(
  obj: any,
  path: string,
  defaultValue: T
): T {
  const keys = path.split('.');
  let current = obj;

  for (const key of keys) {
    if (current === null || current === undefined || !(key in current)) {
      return defaultValue;
    }
    current = current[key];
  }

  return current !== undefined ? current : defaultValue;
}

/**
 * 字符串工具函数
 */

/**
 * 首字母大写
 * @param str 字符串
 * @returns 首字母大写的字符串
 */
export function capitalize(str: string): string {
  if (!str) return str;
  return str.charAt(0).toUpperCase() + str.slice(1).toLowerCase();
}

/**
 * 驼峰命名转短横线命名
 * @param str 驼峰命名字符串
 * @returns 短横线命名字符串
 */
export function camelToKebab(str: string): string {
  return str.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
}

/**
 * 短横线命名转驼峰命名
 * @param str 短横线命名字符串
 * @returns 驼峰命名字符串
 */
export function kebabToCamel(str: string): string {
  return str.replace(/-([a-z])/g, (g) => g[1].toUpperCase());
}

/**
 * 生成随机字符串
 * @param length 长度
 * @param prefix 前缀
 * @returns 随机字符串
 */
export function generateRandomString(length: number = 8, prefix: string = ""): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = prefix;
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}