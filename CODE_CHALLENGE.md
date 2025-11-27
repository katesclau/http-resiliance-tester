## Code Challenge: Client Resilience Tester

Build a Node.js polling client that exercises the `http-resiliance-tester` service.

- Polls `GET /players` every 10ms (enqueue frequency)
- Respects a global client-side rate limit of 20 requests/second
- Retries on errors (HTTP 5xx/429/network)
- Uses exponential backoff with jitter for retries
- Sends required headers: `Content-Type: application/json`, `Authorization: Bearer <token>`

### Prerequisites
- Node.js 18+ (for built-in `fetch`)
- Running server from this repo (default `PORT=8080`, `AUTH_TOKEN=secret-token`)

### Setup
Create a new folder for the client (outside or inside this repo is fine), then create `index.js` with the code below.

Set environment variables as needed:

```bash
export ENDPOINT="http://localhost:8080/players"
export TOKEN="secret-token" # must match AUTH_TOKEN used by the server
node index.js
```

### index.js
```javascript
// Node 18+ required (global fetch)

const ENDPOINT = process.env.ENDPOINT || "http://localhost:8080/players";
const TOKEN = process.env.TOKEN || "secret-token";

// Enqueue a poll task every 10ms (fast producer)
const POLL_INTERVAL_MS = 10;

// Client-side rate limit: 20 requests/second
const RATE_LIMIT_RPS = 20;
const SCHEDULER_TICK_MS = Math.floor(1000 / RATE_LIMIT_RPS); // ~50ms

// Backoff settings
const BASE_BACKOFF_MS = 100;      // initial backoff for first retry
const BACKOFF_FACTOR = 2;         // exponential factor
const MAX_BACKOFF_MS = 5000;      // cap

// Utility: bounded exponential backoff with full jitter
function computeBackoffDelayMs(attempt) {
  const exp = Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * Math.pow(BACKOFF_FACTOR, attempt));
  return Math.floor(Math.random() * exp); // full jitter
}

class RequestTask {
  constructor(id, attempt = 0) {
    this.id = id;
    this.attempt = attempt;
  }
}

class RequestQueue {
  constructor() {
    this.queue = [];
  }
  push(task) {
    this.queue.push(task);
  }
  shift() {
    return this.queue.shift();
  }
  get length() {
    return this.queue.length;
  }
}

const requestQueue = new RequestQueue();
let produced = 0;
let sent = 0;
let succeeded = 0;
let failed = 0;

async function doRequest(task) {
  try {
    const res = await fetch(ENDPOINT, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${TOKEN}`
      }
    });

    if (res.status === 200) {
      // Parse body once to simulate real client work
      await res.json().catch(() => undefined);
      succeeded++;
      return;
    }

    // Retry on 429 (rate limit) and 5xx errors
    if (res.status === 429 || (res.status >= 500 && res.status < 600)) {
      throw new Error(`HTTP ${res.status}`);
    }

    // Other statuses considered terminal failures (no retry)
    failed++;
  } catch (err) {
    // Network or retriable HTTP errors -> schedule retry with backoff
    const nextAttempt = task.attempt + 1;
    const delay = computeBackoffDelayMs(nextAttempt);
    setTimeout(() => {
      requestQueue.push(new RequestTask(task.id, nextAttempt));
    }, delay);
  }
}

// Scheduler enforces 20 rps by sending at most one request each tick
setInterval(() => {
  const task = requestQueue.shift();
  if (!task) return;
  sent++;
  void doRequest(task);
}, SCHEDULER_TICK_MS);

// Producer: enqueue a new task every 10ms
setInterval(() => {
  requestQueue.push(new RequestTask(++produced, 0));
}, POLL_INTERVAL_MS);

// Periodic stats
setInterval(() => {
  console.log(JSON.stringify({
    ts: new Date().toISOString(),
    queueDepth: requestQueue.length,
    produced,
    sent,
    succeeded,
    failed
  }));
}, 1000);

console.log(`Polling ${ENDPOINT} (enqueue every ${POLL_INTERVAL_MS}ms), respecting ${RATE_LIMIT_RPS} rps`);
```

### Notes
- The producer enqueues tasks every 10ms to simulate aggressive polling. The scheduler enforces the 20 rps cap so actual requests comply with the server's rate limit.
- Retries use exponential backoff with jitter to avoid thundering herds, especially after bursts of 500/429 responses.
- The server requires `Content-Type: application/json` for both GET and POST by design for testing strict clients. The client sets it for GET requests as well.

### Optional Enhancements
- Parse and honor `Retry-After` headers if provided by servers.
- Add bounded concurrency (e.g., up to N in-flight requests) while still respecting the 20 rps send rate.
- Persist and visualize the stats for further analysis.


