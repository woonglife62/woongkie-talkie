import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = BASE_URL.replace(/^http/, 'ws');

export const options = {
  vus: 100,
  duration: '2m',
  thresholds: {
    ws_connecting: ['p(95)<1000'],
    ws_msgs_received: ['count>0'],
  },
};

export function setup() {
  const loginRes = http.post(
    `${BASE_URL}/api/auth/login`,
    JSON.stringify({ username: 'loadtest', password: 'loadtest123' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  const token = loginRes.json('token');

  // Get a room to connect to
  const roomsRes = http.get(`${BASE_URL}/api/rooms`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const rooms = roomsRes.json();
  const roomId = rooms && rooms.length > 0 ? (rooms[0]._id || rooms[0].id) : null;

  return { token, roomId };
}

export default function (data) {
  if (!data.roomId) {
    sleep(1);
    return;
  }

  const url = `${WS_URL}/ws/${data.roomId}?token=${data.token}`;

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      // Send a test message after connection
      socket.send(
        JSON.stringify({
          type: 'message',
          content: `Load test message from VU ${__VU}`,
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
  });

  check(res, {
    'WebSocket connection established': (r) => r && r.status === 101,
  });

  sleep(1);
}
