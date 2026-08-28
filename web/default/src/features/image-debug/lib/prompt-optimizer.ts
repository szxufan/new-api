/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  IMAGE_PROMPT_OPTIMIZE_SYSTEM_PROMPT,
  PROMPT_OPTIMIZE_MODEL_STORAGE_KEY,
  VIDEO_PROMPT_OPTIMIZE_SYSTEM_PROMPT,
} from '../constants'
import type {
  ModelOption,
  OptimizeContentPart,
  OptimizePromptRequest,
  OptimizePromptResponse,
  PromptOptimizeType,
} from '../types'

/**
 * 模型候选排序：不区分大小写、按名称（含自然数字序）升序排列。
 * 返回新数组，不修改入参。
 */
export function sortModelOptions(options: ModelOption[]): ModelOption[] {
  return [...options].sort((a, b) =>
    a.value.localeCompare(b.value, undefined, {
      sensitivity: 'base',
      numeric: true,
    })
  )
}

/** 读取上次使用的优化模型（localStorage）；读取失败或不存在时返回空字符串 */
export function getStoredOptimizeModel(): string {
  try {
    return window.localStorage.getItem(PROMPT_OPTIMIZE_MODEL_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

/** 保存本次选择的优化模型到 localStorage；写入失败时静默忽略 */
export function saveStoredOptimizeModel(model: string): void {
  try {
    window.localStorage.setItem(PROMPT_OPTIMIZE_MODEL_STORAGE_KEY, model)
  } catch {
    /* ignore */
  }
}

/**
 * 解析实际选中的优化模型：上次存储的模型仍在候选列表中则使用它，
 * 否则回退到候选第一个；候选为空时返回空字符串。
 */
export function resolveStoredModel(
  stored: string,
  models: ModelOption[]
): string {
  if (models.some((option) => option.value === stored)) return stored
  return models[0]?.value ?? ''
}

/** 视频分镜头单位（秒） */
const VIDEO_SHOT_DURATION_SECONDS = 5

/**
 * 生成视频优化的时长说明行（追加到用户消息文本末尾）。
 * -1 = 智能时长（由 AI 自行设计总时长）；<=0/缺省 = 按默认 5 秒（与 buildVideoPayload 回退一致）。
 */
function buildVideoDurationLine(duration: number | undefined): string {
  if (duration === -1) {
    return '\n\nVideo duration: not fixed (smart duration). Design a reasonable total duration yourself and split it into 5-second shots.'
  }
  const seconds =
    duration && duration > 0 ? duration : VIDEO_SHOT_DURATION_SECONDS
  const shots = Math.ceil(seconds / VIDEO_SHOT_DURATION_SECONDS)
  return `\n\nVideo duration: ${seconds} seconds. Split it into ${shots} shots of 5 seconds each (the last shot may be shorter).`
}

/**
 * 构建提示词优化请求体 — POST /pg/chat/completions（非流式）。
 * group 非空时携带，缺省由后端使用会话默认分组。
 * 视频优化：用户消息追加分镜头时长说明；携带输入图片时 content 为
 * 「文本 + image_url」多模态数组，图片以 dataURL 直传。
 */
export function buildOptimizePayload(options: {
  prompt: string
  type: PromptOptimizeType
  model: string
  group: string
  /** 视频时长（秒）；仅视频优化用于生成分镜头脚本 */
  duration?: number
  /** 视频输入图片 dataURL 列表；仅视频优化时一并传给 AI */
  images?: string[]
}): OptimizePromptRequest {
  const systemPrompt =
    options.type === 'video'
      ? VIDEO_PROMPT_OPTIMIZE_SYSTEM_PROMPT
      : IMAGE_PROMPT_OPTIMIZE_SYSTEM_PROMPT

  let userContent: string | OptimizeContentPart[] = options.prompt
  if (options.type === 'video') {
    const text = options.prompt + buildVideoDurationLine(options.duration)
    const validImages = (options.images ?? []).filter(
      (url) => url.trim() !== ''
    )
    userContent =
      validImages.length === 0
        ? text
        : [
            { type: 'text', text },
            ...validImages.map((url) => ({
              type: 'image_url' as const,
              image_url: { url: url.trim() },
            })),
          ]
  }

  const payload: OptimizePromptRequest = {
    model: options.model,
    messages: [
      { role: 'system', content: systemPrompt },
      { role: 'user', content: userContent },
    ],
    stream: false,
  }
  if (options.group.trim() !== '') {
    payload.group = options.group
  }
  return payload
}

/** 剥离 markdown 代码块围栏（``` 或 ```markdown 等） */
function stripCodeFence(text: string): string {
  const trimmed = text.trim()
  const fenceMatch = trimmed.match(/^```[\w-]*\n?([\s\S]*?)\n?```$/)
  return fenceMatch ? fenceMatch[1].trim() : trimmed
}

/**
 * 从优化响应中提取优化后的提示词；缺失或为空时返回 null。
 * 响应 content 类型为字符串（数组形态仅用于请求侧多模态，此处视为无效）。
 */
export function extractOptimizedPrompt(
  response: OptimizePromptResponse
): string | null {
  const rawContent = response.choices?.[0]?.message?.content
  const content = typeof rawContent === 'string' ? rawContent.trim() : ''
  if (!content) return null
  const optimized = stripCodeFence(content)
  return optimized.length > 0 ? optimized : null
}

/**
 * 提取优化请求失败的错误文案（与页面现有错误提取模式一致）。
 */
export function getOptimizeErrorMessage(
  err: unknown,
  fallback: string
): string {
  const axiosError = err as
    | { response?: { data?: { error?: { message?: string } } } }
    | null
    | undefined
  return (
    axiosError?.response?.data?.error?.message ||
    (err instanceof Error ? err.message : fallback) ||
    fallback
  )
}
