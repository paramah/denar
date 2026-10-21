import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { problemResponse } from '../test/msw/handlers.ts'
import { server } from '../test/msw/server.ts'
import { apiUrl } from '../test/msw/url.ts'
import { api, setUnauthenticatedHandler } from './client.ts'
import { ApiError, classify, errorMessage, fieldErrors, NetworkError } from './errors.ts'

afterEach(() => setUnauthenticatedHandler(null))

describe('api client', () => {
  it('turns a problem into an ApiError with reason and field errors', async () => {
    server.use(
      http.post(apiUrl('/households'), () =>
        problemResponse(422, 'validation', { errors: [{ field: 'name', message: 'za długa' }] }),
      ),
    )
    const error = await api
      .POST('/households', { body: { name: 'x', base_currency: 'PLN', timezone: 'UTC', month_start_day: 1 } })
      .catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect(classify(error)).toBe('validation')
    expect(fieldErrors(error)).toEqual({ name: 'za długa' })
    expect((error as ApiError).reason).toBe('validation')
  })

  it('calls the unauthenticated handler on a 401 of a protected call', async () => {
    const handler = vi.fn()
    setUnauthenticatedHandler(handler)
    server.use(http.get(apiUrl('/households'), () => problemResponse(401, 'unauthenticated')))
    await expect(api.GET('/households')).rejects.toBeInstanceOf(ApiError)
    expect(handler).toHaveBeenCalledOnce()
  })

  it('does not treat a wrong password as a lost session', async () => {
    const handler = vi.fn()
    setUnauthenticatedHandler(handler)
    server.use(http.post(apiUrl('/auth/login'), () => problemResponse(401, 'unauthenticated')))
    await expect(api.POST('/auth/login', { body: { email: 'a@b.c', password: 'x' } })).rejects.toBeInstanceOf(ApiError)
    expect(handler).not.toHaveBeenCalled()
  })

  it('reads Retry-After of a 429', async () => {
    server.use(
      http.post(apiUrl('/auth/login'), () => problemResponse(429, 'rate_limited', {}, { 'Retry-After': '42' })),
    )
    const error = await api.POST('/auth/login', { body: { email: 'a@b.c', password: 'x' } }).catch((e: unknown) => e)
    expect(errorMessage(error)).toBe('Za dużo prób. Spróbuj ponownie za 42 s.')
  })

  it('reports a network failure as NetworkError', async () => {
    server.use(http.get(apiUrl('/me'), () => HttpResponse.error()))
    const error = await api.GET('/me').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(NetworkError)
    expect(classify(error)).toBe('network')
  })
})
