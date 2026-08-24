import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// vitest 未开启 globals，@testing-library/react 的自动清理不会生效，
// 这里手动在每个用例后卸载组件，避免前序渲染残留影响后续查询
afterEach(() => {
  cleanup()
})
