import { describe, expect, it } from 'vitest'
import type { TaskLog } from '../types'
import {
  canDownloadTaskResult,
  extFromContentType,
  parseTaskDataArray,
  resolveTaskDownloads,
} from './download'

function buildTaskLog(overrides: Partial<TaskLog> = {}): TaskLog {
  return {
    id: 1,
    user_id: 42,
    platform: 'kling',
    task_id: 'task_abc123',
    action: 'GENERATE',
    channel_id: 1,
    submit_time: 1700000000,
    status: 'SUCCESS',
    ...overrides,
  }
}

describe('parseTaskDataArray', () => {
  it('returns arrays as-is', () => {
    expect(parseTaskDataArray([{ audio_url: 'a' }])).toEqual([
      { audio_url: 'a' },
    ])
  })

  it('parses JSON string arrays', () => {
    expect(parseTaskDataArray('[{"audio_url":"a"}]')).toEqual([
      { audio_url: 'a' },
    ])
  })

  it('returns empty array for invalid JSON strings', () => {
    expect(parseTaskDataArray('not-json')).toEqual([])
  })

  it('returns empty array for non-array objects', () => {
    expect(parseTaskDataArray({ audio_url: 'a' })).toEqual([])
    expect(parseTaskDataArray(undefined)).toEqual([])
    expect(parseTaskDataArray('{"audio_url":"a"}')).toEqual([])
  })
})

describe('resolveTaskDownloads', () => {
  it('builds proxy URL for non-suno tasks', () => {
    const entries = resolveTaskDownloads(buildTaskLog())
    expect(entries).toEqual([
      { url: '/v1/videos/task_abc123/content', filename: 'task_abc123.mp4' },
    ])
  })

  it('extracts suno audio clips from object data', () => {
    const log = buildTaskLog({
      platform: 'suno',
      data: [
        { audio_url: 'https://cdn1.suno.ai/one.mp3' },
        { audio_url: 'https://cdn1.suno.ai/two.mp3' },
      ] as unknown as string,
    })
    expect(resolveTaskDownloads(log)).toEqual([
      { url: 'https://cdn1.suno.ai/one.mp3', filename: 'one.mp3' },
      { url: 'https://cdn1.suno.ai/two.mp3', filename: 'two.mp3' },
    ])
  })

  it('extracts suno audio clips from JSON string data', () => {
    const log = buildTaskLog({
      platform: 'suno',
      data: '[{"audio_url":"https://cdn1.suno.ai/song.mp3"}]',
    })
    expect(resolveTaskDownloads(log)).toEqual([
      { url: 'https://cdn1.suno.ai/song.mp3', filename: 'song.mp3' },
    ])
  })

  it('falls back to generated filename when suno URL has no extension', () => {
    const log = buildTaskLog({
      platform: 'suno',
      data: '[{"audio_url":"https://cdn1.suno.ai/stream/xyz"}]',
    })
    expect(resolveTaskDownloads(log)).toEqual([
      { url: 'https://cdn1.suno.ai/stream/xyz', filename: 'task_abc123_1.mp3' },
    ])
  })

  it('returns empty list when suno data has no audio clips', () => {
    const log = buildTaskLog({ platform: 'suno', data: '[{"video_url":"v"}]' })
    expect(resolveTaskDownloads(log)).toEqual([])
  })
})

describe('canDownloadTaskResult', () => {
  it('allows owner of a successful task', () => {
    expect(canDownloadTaskResult(buildTaskLog(), 42)).toBe(true)
  })

  it('rejects other users (including admins viewing foreign tasks)', () => {
    expect(canDownloadTaskResult(buildTaskLog(), 7)).toBe(false)
  })

  it('rejects when current user is unknown', () => {
    expect(canDownloadTaskResult(buildTaskLog(), undefined)).toBe(false)
  })

  it('rejects non-success statuses', () => {
    expect(
      canDownloadTaskResult(buildTaskLog({ status: 'IN_PROGRESS' }), 42)
    ).toBe(false)
    expect(canDownloadTaskResult(buildTaskLog({ status: 'FAILURE' }), 42)).toBe(
      false
    )
  })
})

describe('extFromContentType', () => {
  it('maps common media types', () => {
    expect(extFromContentType('video/mp4')).toBe('.mp4')
    expect(extFromContentType('audio/mpeg')).toBe('.mp3')
    expect(extFromContentType('image/png')).toBe('.png')
  })

  it('ignores content-type parameters and case', () => {
    expect(extFromContentType('Video/MP4; charset=utf-8')).toBe('.mp4')
  })

  it('returns empty string for unknown or empty types', () => {
    expect(extFromContentType('')).toBe('')
    expect(extFromContentType('application/octet-stream')).toBe('')
  })
})
