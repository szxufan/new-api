import { describe, it, expect } from 'vitest'
import {
  parseJsonToMappingRows,
  convertMappingRowsToJson,
  type MappingRow,
  findMissingModelsInMapping,
  hasModelConfigChanged,
} from './model-mapping-validation'

describe('parseJsonToMappingRows', () => {
  it('should parse valid JSON to mapping rows', () => {
    const json = '{"gpt-3.5-turbo": "gpt-3.5-turbo-0125"}'
    const rows = parseJsonToMappingRows(json)
    expect(rows).toEqual([
      { id: 'row-0-gpt-3.5-turbo', from: 'gpt-3.5-turbo', to: 'gpt-3.5-turbo-0125' },
    ])
  })

  it('should parse multiple entries', () => {
    const json = '{"gpt-4": "gpt-4-0613", "claude-3": "claude-3-opus"}'
    const rows = parseJsonToMappingRows(json)
    expect(rows).toEqual([
      { id: 'row-0-gpt-4', from: 'gpt-4', to: 'gpt-4-0613' },
      { id: 'row-1-claude-3', from: 'claude-3', to: 'claude-3-opus' },
    ])
  })

  it('should return empty array for empty string', () => {
    expect(parseJsonToMappingRows('')).toEqual([])
    expect(parseJsonToMappingRows('   ')).toEqual([])
  })

  it('should return null for invalid JSON', () => {
    expect(parseJsonToMappingRows('{invalid}')).toBeNull()
    expect(parseJsonToMappingRows('not json')).toBeNull()
  })

  it('should return null for non-object JSON', () => {
    expect(parseJsonToMappingRows('[]')).toBeNull()
    expect(parseJsonToMappingRows('"string"')).toBeNull()
    expect(parseJsonToMappingRows('42')).toBeNull()
  })

  it('should convert non-string values to strings', () => {
    const json = '{"model": 123}'
    const rows = parseJsonToMappingRows(json)
    expect(rows).toEqual([
      { id: 'row-0-model', from: 'model', to: '123' },
    ])
  })

  it('should produce stable IDs based on content', () => {
    const json = '{"gpt-4": "gpt-4-0613"}'
    const rows1 = parseJsonToMappingRows(json)
    const rows2 = parseJsonToMappingRows(json)
    expect(rows1).toEqual(rows2)
  })
})

describe('convertMappingRowsToJson', () => {
  it('should convert rows to formatted JSON', () => {
    const rows: MappingRow[] = [
      { id: 'row-0-gpt-4', from: 'gpt-4', to: 'gpt-4-0613' },
    ]
    const json = convertMappingRowsToJson(rows)
    expect(json).toBe(JSON.stringify({ 'gpt-4': 'gpt-4-0613' }, null, 2))
  })

  it('should convert multiple rows', () => {
    const rows: MappingRow[] = [
      { id: 'row-0-gpt-4', from: 'gpt-4', to: 'gpt-4-0613' },
      { id: 'row-1-claude-3', from: 'claude-3', to: 'claude-3-opus' },
    ]
    const json = convertMappingRowsToJson(rows)
    expect(json).toBe(
      JSON.stringify({ 'gpt-4': 'gpt-4-0613', 'claude-3': 'claude-3-opus' }, null, 2)
    )
  })

  it('should return empty string for empty rows', () => {
    expect(convertMappingRowsToJson([])).toBe('')
  })

  it('should skip rows with empty from field', () => {
    const rows: MappingRow[] = [
      { id: 'row-0-', from: '', to: 'something' },
      { id: 'row-1-gpt-4', from: 'gpt-4', to: 'gpt-4-0613' },
    ]
    const json = convertMappingRowsToJson(rows)
    expect(json).toBe(JSON.stringify({ 'gpt-4': 'gpt-4-0613' }, null, 2))
  })

  it('should trim whitespace in keys and values', () => {
    const rows: MappingRow[] = [
      { id: 'row-0-gpt-4', from: '  gpt-4  ', to: '  gpt-4-0613  ' },
    ]
    const json = convertMappingRowsToJson(rows)
    expect(json).toBe(JSON.stringify({ 'gpt-4': 'gpt-4-0613' }, null, 2))
  })
})

describe('roundtrip: parseJsonToMappingRows -> convertMappingRowsToJson', () => {
  it('should preserve mapping data through roundtrip', () => {
    const originalJson = '{"gpt-4": "gpt-4-0613", "claude-3": "claude-3-opus"}'
    const rows = parseJsonToMappingRows(originalJson)
    expect(rows).not.toBeNull()
    const roundtripJson = convertMappingRowsToJson(rows!)
    const originalParsed = JSON.parse(originalJson)
    const roundtripParsed = JSON.parse(roundtripJson)
    expect(roundtripParsed).toEqual(originalParsed)
  })

  it('should preserve single mapping through roundtrip', () => {
    const originalJson = '{"gpt-3.5-turbo": "gpt-3.5-turbo-0125"}'
    const rows = parseJsonToMappingRows(originalJson)
    expect(rows).not.toBeNull()
    const roundtripJson = convertMappingRowsToJson(rows!)
    expect(JSON.parse(roundtripJson)).toEqual(JSON.parse(originalJson))
  })
})

describe('findMissingModelsInMapping', () => {
  it('should return empty array for empty model_mapping', () => {
    expect(findMissingModelsInMapping('', ['gpt-4'])).toEqual([])
    expect(findMissingModelsInMapping('   ', ['gpt-4'])).toEqual([])
  })

  it('should return empty array for invalid JSON', () => {
    expect(findMissingModelsInMapping('{invalid}', ['gpt-4'])).toEqual([])
    expect(findMissingModelsInMapping('not json', ['gpt-4'])).toEqual([])
  })

  it('should return empty array for non-object JSON', () => {
    expect(findMissingModelsInMapping('[]', ['gpt-4'])).toEqual([])
    expect(findMissingModelsInMapping('"string"', ['gpt-4'])).toEqual([])
    expect(findMissingModelsInMapping('42', ['gpt-4'])).toEqual([])
  })

  it('should return empty when all mapping keys exist in models', () => {
    const mapping = JSON.stringify({ 'gpt-4': 'gpt-4-turbo' })
    expect(findMissingModelsInMapping(mapping, ['gpt-4'])).toEqual([])
  })

  it('should return mapping keys missing from models', () => {
    const mapping = JSON.stringify({
      'gpt-4': 'gpt-4-turbo',
      'claude-3': 'claude-3-opus',
    })
    expect(findMissingModelsInMapping(mapping, ['gpt-4'])).toEqual([
      'claude-3',
    ])
  })

  it('should return all keys when models list is empty', () => {
    const mapping = JSON.stringify({
      'gpt-4': 'gpt-4-turbo',
      'claude-3': 'claude-3-opus',
    })
    expect(findMissingModelsInMapping(mapping, [])).toEqual([
      'gpt-4',
      'claude-3',
    ])
  })

  it('should deduplicate and trim mapping keys', () => {
    const mapping = JSON.stringify({ ' gpt-4 ': 'gpt-4-turbo' })
    expect(findMissingModelsInMapping(mapping, [])).toEqual(['gpt-4'])
  })
})

describe('hasModelConfigChanged', () => {
  it('should return true for new channel (empty initial)', () => {
    expect(hasModelConfigChanged(['gpt-4'], '', [], '')).toBe(true)
  })

  it('should return true when models length changed', () => {
    expect(
      hasModelConfigChanged(['gpt-4', 'gpt-3.5'], '', ['gpt-4'], '')
    ).toBe(true)
  })

  it('should return true when a model entry changed', () => {
    expect(hasModelConfigChanged(['gpt-4o'], '', ['gpt-4'], '')).toBe(true)
  })

  it('should return true when model_mapping changed', () => {
    const mapping = JSON.stringify({ 'gpt-4': 'gpt-4-turbo' })
    expect(hasModelConfigChanged(['gpt-4'], mapping, ['gpt-4'], '')).toBe(true)
  })

  it('should return false when nothing changed', () => {
    const mapping = JSON.stringify({ 'gpt-4': 'gpt-4-turbo' })
    expect(hasModelConfigChanged(['gpt-4'], mapping, ['gpt-4'], mapping)).toBe(
      false
    )
  })
})