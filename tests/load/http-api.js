import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  vus: 50,
  duration: '2m',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<500'],
  },
};

export function setup() {
  // Login to get a token for subsequent requests
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ username: 'loadtest', password: 'loadtest123' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  const token = loginRes.json('token');
  return { token };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${data.token}`,
  };

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
  }

  sleep(1);
}
