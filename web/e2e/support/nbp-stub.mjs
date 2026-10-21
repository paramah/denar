// Atrapa API NBP dla e2e: każda waluta kosztuje stałe 4.0 PLN, notowanie
// z ostatniego dnia zakresu. Serwer Denara dostaje jej adres w DENAR_NBP_URL,
// więc testy nie zależą od sieci. Port z argumentu.
//
// Ten sam serwer udaje odbiorcę webhooków powiadomień (etap 005):
// POST /webhook/<nazwa> zapisuje żądanie (nagłówki i surową treść) i odpowiada 200,
// POST /webhook-fail/<nazwa> odpowiada 500, GET /webhook/<nazwa> zwraca zapisane żądania.
import { createServer } from 'node:http'

/** @type {Map<string, {headers: Record<string, string | string[] | undefined>, body: string}[]>} */
const received = new Map()
const hookRoute = /^\/(webhook|webhook-fail)\/([\w-]+)$/

const port = Number(process.argv[2])
const rateRoute = /^\/api\/exchangerates\/rates\/a\/([a-z]{3})\/(\d{4}-\d{2}-\d{2})\/(\d{4}-\d{2}-\d{2})\/?$/

createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost')
  if (url.pathname === '/healthz') {
    response.writeHead(200, { 'Content-Type': 'text/plain' }).end('ok')
    return
  }
  const hook = hookRoute.exec(url.pathname)
  if (hook) {
    const [, kind, name] = hook
    if (request.method === 'GET') {
      response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(received.get(name) ?? []))
      return
    }
    const chunks = []
    request.on('data', (chunk) => chunks.push(chunk))
    request.on('end', () => {
      if (kind === 'webhook-fail') {
        response.writeHead(500, { 'Content-Type': 'text/plain' }).end('stub failure')
        return
      }
      const list = received.get(name) ?? []
      list.push({ headers: request.headers, body: Buffer.concat(chunks).toString('utf8') })
      received.set(name, list)
      response.writeHead(200, { 'Content-Type': 'text/plain' }).end('ok')
    })
    return
  }
  const match = rateRoute.exec(url.pathname)
  if (!match) {
    response.writeHead(404, { 'Content-Type': 'text/plain' }).end('404 NotFound')
    return
  }
  const [, code, , end] = match
  // "mid" jako surowy literał JSON — serwer czyta go bez floatów.
  const body = `{"table":"A","currency":"stub","code":"${code.toUpperCase()}","rates":[{"no":"001/A/NBP/2026","effectiveDate":"${end}","mid":4.0000}]}`
  response.writeHead(200, { 'Content-Type': 'application/json' }).end(body)
}).listen(port, '127.0.0.1')
