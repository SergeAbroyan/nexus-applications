import http from "k6/http";
import { check, sleep } from "k6";

const baseUrl = (__ENV.BASE_URL || "http://localhost:8081").replace(/\/$/, "");
const users = Number(__ENV.VUS || 10);
const json = { headers: { "Content-Type": "application/json" } };

export const options = {
  scenarios: {
    traffic: {
      executor: "ramping-vus",
      stages: [
        { duration: __ENV.RAMP_UP || "30s", target: users },
        { duration: __ENV.DURATION || "2m", target: users },
        { duration: "15s", target: 0 },
      ],
      gracefulRampDown: "10s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.005"],
    "http_req_duration{expected_response:true}": ["p(99)<500"],
    checks: ["rate>0.99"],
  },
};

export default function () {
  const listed = http.get(`${baseUrl}/api/items`, { tags: { name: "GET /api/items" } });
  check(listed, {
    "items are listed": (response) => response.status === 200 && Array.isArray(response.json("items")),
  });

  if (Math.random() < 0.2) {
    const created = http.post(
      `${baseUrl}/api/items`,
      JSON.stringify({ name: `k6 ${__VU}-${__ITER}` }),
      { ...json, tags: { name: "POST /api/items" } },
    );
    check(created, { "an item is created": (response) => response.status === 201 });
  }

  if (Math.random() < 0.1) {
    const queued = http.post(
      `${baseUrl}/api/notify`,
      JSON.stringify({ message: `k6 ${__VU}-${__ITER}` }),
      { ...json, tags: { name: "POST /api/notify" } },
    );
    check(queued, { "a notification is queued": (response) => response.status === 202 });
  }

  sleep(1);
}
