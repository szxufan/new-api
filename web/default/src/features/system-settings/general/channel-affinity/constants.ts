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
import type { AffinityRule } from './types'

const CODEX_CLI_HEADER_PASSTHROUGH_HEADERS = [
  'Originator',
  'Session_id',
  'User-Agent',
  'X-Codex-Beta-Features',
  'X-Codex-Turn-Metadata',
]

const CLAUDE_CLI_HEADER_PASSTHROUGH_HEADERS = [
  'X-Stainless-Arch',
  'X-Stainless-Lang',
  'X-Stainless-Os',
  'X-Stainless-Package-Version',
  'X-Stainless-Retry-Count',
  'X-Stainless-Runtime',
  'X-Stainless-Runtime-Version',
  'X-Stainless-Timeout',
  'User-Agent',
  'X-App',
  'Anthropic-Beta',
  'Anthropic-Dangerous-Direct-Browser-Access',
  'Anthropic-Version',
]

function buildPassHeadersTemplate(headers: string[]) {
  return {
    operations: [
      {
        mode: 'pass_headers',
        value: [...headers],
        keep_origin: true,
      },
    ],
  }
}

export type RuleTemplate = Omit<AffinityRule, 'id'>

export const RULE_TEMPLATES: Record<string, RuleTemplate> = {
  codexCli: {
    name: 'codex cli trace',
    model_regex: ['^gpt-.*$'],
    path_regex: ['/v1/responses'],
    key_sources: [{ type: 'gjson', path: 'prompt_cache_key' }],
    param_override_template: buildPassHeadersTemplate(
      CODEX_CLI_HEADER_PASSTHROUGH_HEADERS
    ),
    value_regex: '',
    ttl_seconds: 0,
    skip_retry_on_failure: true,
    include_using_group: true,
    include_model_name: false,
    include_rule_name: true,
  },
  claudeCli: {
    name: 'claude cli trace',
    model_regex: ['^claude-.*$'],
    path_regex: ['/v1/messages'],
    key_sources: [{ type: 'gjson', path: 'metadata.user_id' }],
    param_override_template: buildPassHeadersTemplate(
      CLAUDE_CLI_HEADER_PASSTHROUGH_HEADERS
    ),
    value_regex: '',
    ttl_seconds: 0,
    skip_retry_on_failure: true,
    include_using_group: true,
    include_model_name: false,
    include_rule_name: true,
  },
}

export function makeUniqueName(
  existingNames: Set<string>,
  baseName: string
): string {
  const base = (baseName || '').trim() || 'rule'
  if (!existingNames.has(base)) return base
  for (let i = 2; i < 1000; i++) {
    const n = `${base}-${i}`
    if (!existingNames.has(n)) return n
  }
  return `${base}-${Date.now()}`
}

export function cloneTemplate<T>(template: T): T {
  return JSON.parse(JSON.stringify(template))
}

// 网关已知的 relay 端点（来源：router/relay-router.go、router/video-router.go）。
// 值会原样存入 path_regex / path_regex_exclude，后端按子串式正则匹配，
// 纯路径串即"前缀匹配"（如 /v1/videos 同时覆盖任务查询与 remix）。
export interface RelayPathGroup {
  group: string
  paths: { value: string; label: string }[]
}

export const RELAY_PATH_GROUPS: RelayPathGroup[] = [
  {
    group: 'Chat / Text',
    paths: [
      { value: '/v1/chat/completions', label: 'Chat Completions' },
      { value: '/v1/completions', label: 'Completions (Legacy)' },
      { value: '/v1/responses', label: 'Responses' },
      { value: '/v1/responses/compact', label: 'Responses Compact' },
      { value: '/v1/messages', label: 'Claude Messages' },
      { value: '/v1/realtime', label: 'Realtime (WebSocket)' },
      { value: '/v1beta/models/', label: 'Gemini Native' },
    ],
  },
  {
    group: 'Image',
    paths: [
      { value: '/v1/images/generations', label: 'Images Generations' },
      { value: '/v1/images/edits', label: 'Images Edits' },
      { value: '/v1/edits', label: 'Edits (Legacy)' },
    ],
  },
  {
    group: 'Video',
    paths: [
      { value: '/v1/videos', label: 'Videos (OpenAI Compatible)' },
      { value: '/v1/video/generations', label: 'Video Generations' },
      { value: '/kling/v1/videos', label: 'Kling Videos' },
    ],
  },
  {
    group: 'Audio',
    paths: [
      { value: '/v1/audio/speech', label: 'Audio Speech (TTS)' },
      {
        value: '/v1/audio/transcriptions',
        label: 'Audio Transcriptions (ASR)',
      },
      { value: '/v1/audio/translations', label: 'Audio Translations' },
    ],
  },
  {
    group: 'Other',
    paths: [
      { value: '/v1/embeddings', label: 'Embeddings' },
      { value: '/v1/rerank', label: 'Rerank' },
      { value: '/v1/moderations', label: 'Moderations' },
      { value: '/mj/submit', label: 'Midjourney Submit' },
      { value: '/suno/submit', label: 'Suno Submit' },
    ],
  },
]

// 供 MultiSelect 使用的扁平选项；组名走 i18n，标签带路径便于搜索与识别
export function buildRelayPathOptions(
  t: (key: string) => string
): { label: string; value: string }[] {
  return RELAY_PATH_GROUPS.flatMap((g) =>
    g.paths.map((p) => ({
      value: p.value,
      label: `${t(g.group)} · ${p.value}`,
    }))
  )
}
