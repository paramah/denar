import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { Ajv2020, type ValidateFunction } from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import { parse } from 'yaml'
import type { components } from '../api/schema.d.ts'

// Checks fixtures and mocked responses against api/openapi.yaml, so a mock
// cannot drift from the contract (design §8.2). The contract is OpenAPI 3.1,
// whose schemas are JSON Schema 2020-12.

type Json = Record<string, unknown>

// Through a variable: a literal `new URL(path, import.meta.url)` is an asset
// import to Vite and would not point at the file.
const contractFile = '../../../api/openapi.yaml'
const contractPath = fileURLToPath(new URL(contractFile, import.meta.url).href)
const contract = parse(readFileSync(contractPath, 'utf8')) as Json

// strict: false, because the document as a whole is not a schema; it is
// added only so that "#/components/..." references resolve.
const ajv = new Ajv2020({ strict: false, allErrors: true })
addFormats(ajv)
ajv.addSchema(contract, 'contract')

const validators = new Map<string, ValidateFunction>()

function check(pointer: string, value: unknown): string[] {
  let validate = validators.get(pointer)
  if (!validate) {
    validate = ajv.compile({ $ref: `contract#${pointer}` })
    validators.set(pointer, validate)
  }
  if (validate(value)) {
    return []
  }
  return (validate.errors ?? []).map((error) => `${error.instancePath || '/'} ${error.message}`)
}

const escape = (segment: string) => segment.replaceAll('~', '~0').replaceAll('/', '~1')
const pointerOf = (...segments: string[]) => '/' + segments.map(escape).join('/')

function at(pointer: string): unknown {
  return pointer
    .split('/')
    .slice(1)
    .map((segment) => segment.replaceAll('~1', '/').replaceAll('~0', '~'))
    .reduce<unknown>((node, segment) => (node as Json | undefined)?.[segment], contract)
}

// Follows "$ref" of a response object; returns where it really is.
function dereference(pointer: string): string {
  const ref = (at(pointer) as Json | undefined)?.$ref
  return typeof ref === 'string' ? dereference(ref.slice(1)) : pointer
}

/** Errors of a value against a schema of the contract; empty when it matches. */
export function validateSchema(name: keyof components['schemas'], value: unknown): string[] {
  return check(pointerOf('components', 'schemas', name), value)
}

const templates = Object.keys(contract.paths as Json)
  .map((template) => ({
    template,
    pattern: new RegExp(`^${template.replace(/\{[^}]+\}/g, '[^/]+')}$`),
    params: template.split('{').length - 1,
  }))
  // A literal segment wins over a parameter.
  .sort((a, b) => a.params - b.params)

export type MockedResponse = {
  method: string
  /** Path below /api, e.g. /issues/RZL-1. */
  path: string
  status: number
  headers: Record<string, string>
  /** Parsed JSON, or text; undefined when the response has no body. */
  body?: unknown
}

const problemPointer = pointerOf('components', 'schemas', 'Problem')

/** Errors of a response against its operation; empty when the contract allows it. */
export function validateResponse(response: MockedResponse): string[] {
  const method = response.method.toLowerCase()
  const match = templates.find(({ pattern }) => pattern.test(response.path))
  const operation = match && pointerOf('paths', match.template, method)
  if (!match || at(operation!) === undefined) {
    return [`${response.method} ${response.path} is not an operation of api/openapi.yaml`]
  }
  const name = `${response.method} ${match.template}`
  const contentType =
    Object.entries(response.headers)
      .find(([header]) => header.toLowerCase() === 'content-type')?.[1]
      .split(';')[0]
      ?.trim() ?? ''
  const prefix = (errors: string[]) => errors.map((error) => `${name} ${response.status}: ${error}`)

  const declared = `${operation}/responses/${response.status}`
  if (at(declared) === undefined) {
    // The contract lists no 5xx, but any operation may fail with a problem.
    if (response.status >= 500 && contentType === 'application/problem+json') {
      return prefix(check(problemPointer, response.body))
    }
    return [`${name} does not declare status ${response.status}`]
  }

  const content = `${dereference(declared)}/content`
  if (at(content) === undefined) {
    return response.body === undefined ? [] : [`${name} ${response.status} declares no body`]
  }
  const schema = `${content}/${escape(contentType)}/schema`
  if (at(schema) === undefined) {
    return [`${name} ${response.status} does not declare content type ${contentType || '(none)'}`]
  }
  return prefix(check(schema, response.body))
}
