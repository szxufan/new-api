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
import { useCallback, useEffect, useRef, useState } from 'react'

export interface AudioRecorderState {
  isRecording: boolean
  /** 已录制秒数（每秒递增） */
  duration: number
  /** 录制完成的音频文件（webm） */
  file: File | null
  /** 错误文案的 i18n key（由展示层用 t() 翻译），无错误时为 null */
  errorKey: string | null
  start: () => Promise<void>
  stop: () => void
  /** 清空已录制文件与错误 */
  reset: () => void
}

const ERROR_UNSUPPORTED = 'Recording is not supported in this browser'
const ERROR_PERMISSION_DENIED = 'Microphone permission denied'
const ERROR_ACCESS_FAILED = 'Failed to access microphone'

/**
 * 麦克风录音 hook：基于 MediaRecorder，产出 webm 音频 File 供 ASR 调试上传。
 * 组件卸载时自动停止录音并释放资源。
 */
export function useAudioRecorder(): AudioRecorderState {
  const [isRecording, setIsRecording] = useState(false)
  const [duration, setDuration] = useState(0)
  const [file, setFile] = useState<File | null>(null)
  const [errorKey, setErrorKey] = useState<string | null>(null)

  const recorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const cleanup = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
    const recorder = recorderRef.current
    if (recorder && recorder.state !== 'inactive') {
      recorder.stop()
    }
    recorderRef.current?.stream.getTracks().forEach((track) => track.stop())
    recorderRef.current = null
    setIsRecording(false)
  }, [])

  useEffect(() => {
    return cleanup
  }, [cleanup])

  const start = useCallback(async () => {
    setErrorKey(null)
    if (
      typeof navigator === 'undefined' ||
      !navigator.mediaDevices?.getUserMedia ||
      typeof MediaRecorder === 'undefined'
    ) {
      setErrorKey(ERROR_UNSUPPORTED)
      return
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mimeType = MediaRecorder.isTypeSupported('audio/webm')
        ? 'audio/webm'
        : ''
      const recorder = mimeType
        ? new MediaRecorder(stream, { mimeType })
        : new MediaRecorder(stream)
      chunksRef.current = []

      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) chunksRef.current.push(event.data)
      }
      recorder.onstop = () => {
        const type = recorder.mimeType || 'audio/webm'
        const blob = new Blob(chunksRef.current, { type })
        const ext = type.includes('ogg') ? 'ogg' : 'webm'
        setFile(new File([blob], `recording.${ext}`, { type }))
        stream.getTracks().forEach((track) => track.stop())
      }

      recorder.start()
      recorderRef.current = recorder
      setDuration(0)
      setIsRecording(true)
      timerRef.current = setInterval(() => {
        setDuration((prev) => prev + 1)
      }, 1000)
    } catch (err) {
      // 用户拒绝授权时给出明确提示，其余失败统一提示无法访问麦克风
      const denied =
        err instanceof Error &&
        (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError')
      setErrorKey(denied ? ERROR_PERMISSION_DENIED : ERROR_ACCESS_FAILED)
    }
  }, [])

  const stop = useCallback(() => {
    cleanup()
  }, [cleanup])

  const reset = useCallback(() => {
    setFile(null)
    setErrorKey(null)
    setDuration(0)
  }, [])

  return { isRecording, duration, file, errorKey, start, stop, reset }
}
