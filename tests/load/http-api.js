import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TEST_USER = __ENV.TEST_USER || 'loadtest';
const TEST_PASS = __ENV.TEST_PASS || 'loadtest123';

export const options = {
  scenarios: {
    api_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '2m',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<500'],
  },
};

export function setup() {
  // Login to get a session cookie
  const jar = http.cookieJar();

  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ username: TEST_USER, password: TEST_PASS }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      jar,
    }
  );

  check(loginRes, { 'login succeeded': (r) => r.status === 200 });

  const token = loginRes.json('token');
  const cookies = jar.cookiesForURL(BASE_URL);
  const cookieHeader = Object.entries(cookies)
    .map(([k, v]) => `${k}=${v[0]}`)
    .join('; ');

  return { token, cookieHeader };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'X-Requested-With': 'XMLHttpRequest',
    Authorization: `Bearer ${data.token}`,
    Cookie: data.cookieHeader,
  };

  // Health check
  const healthRes = http.get(`${BASE_URL}/health`, { headers });
  check(healthRes, {
    'health check status 200': (r) => r.status === 200,
  });

  // List chat rooms
  const roomsRes = http.get(`${BASE_URL}/rooms`, { headers });
  check(roomsRes, {
    'list rooms status 200': (r) => r.status === 200,
  });

  // Get messages from first room if available
  const rooms = roomsRes.json();
  if (rooms && rooms.length > 0) {
    const roomId = rooms[0]._id || rooms[0].id;

    const msgsRes = http.get(`${BASE_URL}/rooms/${roomId}/messages`, { headers });
    check(msgsRes, {
      'get messages status 200': (r) => r.status === 200,
    });

    // Search messages
    const searchRes = http.get(
      `${BASE_URL}/rooms/${roomId}/messages?q=test`,
      { headers }
    );
    check(searchRes, {
      'search messages status 200': (r) => r.status === 200 || r.status === 404,
    });

    // Get room details
    const roomRes = http.get(`${BASE_URL}/rooms/${roomId}`, { headers });
    check(roomRes, {
      'get room details status 200': (r) => r.status === 200,
    });
  }

  sleep(1);
}
