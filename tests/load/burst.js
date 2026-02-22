import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = BASE_URL.replace(/^http/, 'ws');
const TEST_USER = __ENV.TEST_USER || 'loadtest';
const TEST_PASS = __ENV.TEST_PASS || 'loadtest123';

// Burst scenario: 50 users each send 10 messages as fast as possible (500 total in ~1s)
export const options = {
  vus: 50,
  duration: '30s',
  thresholds: {
    ws_msgs_received: ['count>0'],
    checks: ['rate>0.9'],
  },
};

export function setup() {
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

  const roomsRes = http.get(`${BASE_URL}/rooms`, {
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
    jar,
  });
  const rooms = roomsRes.json();
  const roomId = rooms && rooms.length > 0 ? (rooms[0]._id || rooms[0].id) : null;

  const cookies = jar.cookiesForURL(BASE_URL);
  const cookieHeader = Object.entries(cookies)
    .map(([k, v]) => `${k}=${v[0]}`)
    .join('; ');

  return { cookieHeader, roomId };
}

export default function (data) {
  if (!data.roomId) {
    sleep(1);
    return;
  }

  const url = `${WS_URL}/rooms/${data.roomId}/ws`;

  ws.connect(
    url,
    {
      headers: {
        Cookie: data.cookieHeader,
        'X-Requested-With': 'XMLHttpRequest',
      },
    },
    function (socket) {
      socket.on('open', () => {
        // Burst: send 10 messages immediately (50 VUs x 10 = 500 messages in ~1s)
        for (let i = 0; i < 10; i++) {
          socket.send(
            JSON.stringify({
              Event: 'MSG',
              message: `Burst message ${i + 1} from VU ${__VU} at ${Date.now()}`,
              room_id: data.roomId,
            })
          );
        }

        // Close after sending burst
        socket.setTimeout(() => socket.close(), 2000);
      });

      socket.on('message', (msg) => {
        check(msg, {
          'received burst message': (m) => m !== null,
        });
      });
    }
  );

  // Wait between bursts
  sleep(5);
}
