import { describe, expect, it } from 'vitest'

import { handleDropdownMenuItemSelect } from './dropdown-menu-events'

function createMenuEvent() {
  let defaultPrevented = false
  let baseUIHandlerPrevented = false

  return {
    get defaultPrevented() {
      return defaultPrevented
    },
    preventDefault() {
      defaultPrevented = true
    },
    preventBaseUIHandler() {
      baseUIHandlerPrevented = true
    },
    get baseUIHandlerPrevented() {
      return baseUIHandlerPrevented
    },
  } as unknown as Parameters<typeof handleDropdownMenuItemSelect>[0] & {
    baseUIHandlerPrevented: boolean
  }
}

describe('DropdownMenuItem onSelect compatibility', () => {
  it('calls the Radix-style onSelect handler on item click', () => {
    const event = createMenuEvent()
    let selected = false

    handleDropdownMenuItemSelect(event, undefined, () => {
      selected = true
    })

    expect(selected).toBe(true)
    expect(event.baseUIHandlerPrevented).toBe(false)
  })

  it('keeps the Base UI menu open when onSelect prevents default', () => {
    const event = createMenuEvent()

    handleDropdownMenuItemSelect(event, undefined, (selectEvent) => {
      selectEvent.preventDefault()
    })

    expect(event.defaultPrevented).toBe(true)
    expect(event.baseUIHandlerPrevented).toBe(true)
  })
})
