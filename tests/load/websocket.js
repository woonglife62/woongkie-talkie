import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = BASE_URL.replace(/^http/, 'ws');
const TEST_USER = __ENV.TEST_USER || 'loadtest';
const TEST_PASS = __ENV.TEST_PASS || 'loadtest123';

export const options = {
  vus: 100,
  duration: '2m',
  thresholds: {
    ws_connecting: ['p(95)<1000'],
    ws_msgs_received: ['count>0'],
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

  // Get a room to connect to
  const roomsRes = http.get(`${BASE_URL}/rooms`, {
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
    jar,
  });
  const rooms = roomsRes.json();
  const roomId = rooms && rooms.length > 0 ? (rooms[0]._id || rooms[0].id) : null;

  // Extract cookie string for passing to WS
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

  const res = ws.connect(
    url,
    {
      headers: {
        Cookie: data.cookieHeader,
        'X-Requested-With': 'XMLHttpRequest',
      },
    },
    function (socket) {
      socket.on('open', () => {
        socket.send(
          JSON.stringify({
            Event: 'MSG',
            message: `Load test message from VU ${__VU}`,
            room_id: data.roomId,
          })
        );
      });

      socket.on('message', (msg) => {
        check(msg, {
          'received valid message': (m) => m !== null && m !== '',
        });
      });

      socket.on('error', (e) => {
        console.error(`WebSocket error: ${e}`);
      });

      // Hold connection for 30s then close
      socket.setTimeout(() => {
        socket.close();
      }, 30000);
    }
  );

  check(res, {
    'WebSocket connection established': (r) => r && r.status === 101,
  });

  sleep(1);
}
