import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = BASE_URL.replace(/^http/, 'ws');

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
  const loginRes = http.post(
    `${BASE_URL}/api/auth/login`,
    JSON.stringify({ username: 'loadtest', password: 'loadtest123' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  const token = loginRes.json('token');

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

  ws.connect(url, {}, function (socket) {
    socket.on('open', () => {
      // Burst: send 10 messages immediately (50 VUs x 10 = 500 messages in ~1s)
      for (let i = 0; i < 10; i++) {
        socket.send(
          JSON.stringify({
            type: 'message',
            content: `Burst message ${i + 1} from VU ${__VU} at ${Date.now()}`,
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
  });

  // Wait between bursts
  sleep(5);
}
