// Load test: many clients shorten new urls (MODE=write) or resolve existing codes (MODE=read).
// Run: see README, section "Долгая работа и нагрузка".
import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE || 'http://localhost:8080';
const MODE = __ENV.MODE || 'read';
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

export const options = {
  vus: Number(__ENV.VUS || 200), // concurrent clients
  duration: __ENV.DURATION || '20s',
  thresholds: { http_req_failed: ['rate==0'] }, // any error fails the run
};

// setup creates 1000 links, so reads hit different codes, not one hot row.
export function setup() {
  const codes = [];
  for (let i = 0; i < 1000; i++) {
    const res = http.post(`${BASE}/`, JSON.stringify({ url: `https://example.com/seed/${i}/${Date.now()}` }), JSON_HEADERS);
    codes.push(res.json('short_url').split('/').pop());
  }
  return { codes };
}

export default function (data) {
  if (MODE === 'write') {
    const url = `https://example.com/${__VU}/${__ITER}/${Math.random()}`; // always a new url
    const res = http.post(`${BASE}/`, JSON.stringify({ url }), JSON_HEADERS);
    check(res, { 'created 201': (r) => r.status === 201 });
  } else {
    const code = data.codes[Math.floor(Math.random() * data.codes.length)];
    const res = http.get(`${BASE}/${code}`);
    check(res, { 'found 200': (r) => r.status === 200 });
  }
}
