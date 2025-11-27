## http-resiliance-tester

A small Go HTTP service that intentionally behaves unreliably to help test client resiliency:
- **Routes**: `GET /players`, `POST /players`
- **Flaky behavior**: 70% success, 30% HTTP 500
- **Rate limit**: 20 requests/second (global); over limit returns HTTP 429
- **Required headers**: `Content-Type: application/json`, `Authorization: Bearer <token>`

### Requirements
- Go 1.24+

### Build
```bash
cd http-resiliance-tester
go build -o http-resiliance-tester
```

### Run
```bash
# Optional: customize port and token
export PORT=8080
export AUTH_TOKEN=secret-token
./http-resiliance-tester
```

On startup, the server logs the required bearer token value. By default:
- Port: `:8080`
- Bearer token: `secret-token`

### Behavior
- The service enforces a global rate limit of 20 requests/second. Exceeding this returns:
  - `429 Too Many Requests` with JSON error.
- Each request independently has a 30% chance to return:
  - `500 Internal Server Error` with JSON error (`"simulated failure"`).
- Both `GET` and `POST` require:
  - Header `Content-Type: application/json`
  - Header `Authorization: Bearer <token>`

Note: Requiring `Content-Type` for `GET` is intentional for testing strict clients.

### Endpoints
- `GET /players` → returns a JSON array of fictional rugby players.
- `POST /players` → returns the same JSON array (body is ignored).

### cURL Examples
Replace `<TOKEN>` with your configured token (default: `secret-token`).

```bash
# GET
curl -i \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  "http://localhost:8080/players"

# POST
curl -i \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{}' \
  "http://localhost:8080/players"
```

### Configuration
- `PORT`: server listen port (default `8080`)
- `AUTH_TOKEN`: bearer token required in `Authorization` header (default `secret-token`)

### Example Responses
Success (200):
```json
[
  {"id":1,"name":"Jack O'Connell","position":"Lock","country":"Ireland"},
  {"id":2,"name":"Mako Vunipola","position":"Prop","country":"England"}
  // ...
]
```

Error (429):
```json
{"error":"rate limit exceeded (20 req/s)"}
```

Error (500):
```json
{"error":"simulated failure"}
```

Error (401):
```json
{"error":"invalid token"}
```


