// Atrapa API NBP dla e2e: każda waluta kosztuje stałe 4.0 PLN, notowanie
// z ostatniego dnia zakresu. Serwer Denara dostaje jej adres w DENAR_NBP_URL,
// więc testy nie zależą od sieci. Port z argumentu.
import { createServer } from 'node:http'

const port = Number(process.argv[2])
const rateRoute = /^\/api\/exchangerates\/rates\/a\/([a-z]{3})\/(\d{4}-\d{2}-\d{2})\/(\d{4}-\d{2}-\d{2})\/?$/

createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost')
  if (url.pathname === '/healthz') {
    response.writeHead(200, { 'Content-Type': 'text/plain' }).end('ok')
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
