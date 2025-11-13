import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 20 },
    { duration: '2m', target: 50 },
    { duration: '1m', target: 0 },
  ],
};

const host = __ENV.HOST || 'http://dev.local';

export default function () {
  http.get(`${host}/v1/appoints/available-schedule?professional_id=4102&unit_id=108`);
  http.get(`${host}/v2/appoints/available-schedule?professional_id=512&unit_id=42`);
  sleep(0.2);
}