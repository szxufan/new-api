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
import { describe, it, expect } from 'vitest'
import { buildSample, type SampleContext } from './model-details-api'

function makeCtx(overrides: Partial<SampleContext> = {}): SampleContext {
  return {
    baseUrl: 'https://api.example.com',
    apiKeyEnv: 'NEW_API_KEY',
    modelName: 'gpt-image-1',
    endpointType: 'image-edit',
    endpointPath: '/v1/images/edit',
    ...overrides,
  }
}

describe('pricing 调用示例：image-edit 端点', () => {
  it('curl 示例应指向 /v1/images/edit，并以 multipart 方式上传图片', () => {
    const code = buildSample('curl', 'image-edit', makeCtx())
    expect(code).toContain('https://api.example.com/v1/images/edit')
    expect(code).toContain('-F image=@./input.png')
    expect(code).toContain('-F model="gpt-image-1"')
    expect(code).toContain('-F "prompt=')
    expect(code).toContain('-F n=1')
  })

  it('python 示例应使用 client.images.edit 并读取本地图片', () => {
    const code = buildSample('python', 'image-edit', makeCtx())
    expect(code).toContain('client.images.edit(')
    expect(code).toContain('image=open("./input.png", "rb")')
    expect(code).toContain('size="1024x1024"')
    expect(code).toContain('print(response.data[0].url)')
  })

  it('typescript 示例应使用 client.images.edit 与 fs.readFileSync', () => {
    const code = buildSample('typescript', 'image-edit', makeCtx())
    expect(code).toContain("import fs from 'node:fs'")
    expect(code).toContain('client.images.edit({')
    expect(code).toContain("image: fs.readFileSync('./input.png')")
    expect(code).toContain('console.log(response.data[0].url)')
  })

  it('javascript 示例应使用 FormData，且不手动设置 Content-Type', () => {
    const code = buildSample('javascript', 'image-edit', makeCtx())
    expect(code).toContain('const formData = new FormData()')
    expect(code).toContain("formData.append('image'")
    expect(code).toContain('body: formData,')
    // multipart 边界由浏览器自动生成，不应出现手动设置的 application/json
    expect(code).not.toContain("'Content-Type': 'application/json'")
  })

  it('示例请求体应包含 model 与提示词', () => {
    const code = buildSample('curl', 'image-edit', makeCtx())
    expect(code).toContain('gpt-image-1')
    expect(code).toContain('A serene koi pond at sunset, ukiyo-e style.')
  })
})

describe('pricing 调用示例：既有端点回归', () => {
  it('image-generation 仍生成 images.generate 示例', () => {
    const code = buildSample(
      'python',
      'image-generation',
      makeCtx({
        endpointType: 'image-generation',
        endpointPath: '/v1/images/generations',
      })
    )
    expect(code).toContain('client.images.generate(')
    expect(code).toContain('base_url="https://api.example.com/v1"')
    expect(code).not.toContain('client.images.edit(')
  })

  it('不支持 image-edit 的模型走 chat 示例时不受影响', () => {
    const code = buildSample(
      'python',
      'openai',
      makeCtx({ endpointType: 'openai', endpointPath: '/v1/chat/completions' })
    )
    expect(code).toContain('client.chat.completions.create(')
  })
})