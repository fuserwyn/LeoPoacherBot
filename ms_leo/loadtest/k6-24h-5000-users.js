import http from "k6/http";
import { check, sleep } from "k6";
import exec from "k6/execution";
import crypto from "k6/crypto";

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const BOT_TOKEN = __ENV.BOT_TOKEN || "";
const USERS_TOTAL = Number(__ENV.USERS_TOTAL || 5000);
const TEST_DURATION = __ENV.TEST_DURATION || "24h";
const CHAT_ID = Number(__ENV.MONETIZED_CHAT_ID || 0);
const CHAT_TITLE = __ENV.CHAT_TITLE || "Staya";
const CHAT_INSTANCE = __ENV.CHAT_INSTANCE || "loadtest-chat-instance";

if (!BOT_TOKEN) {
  throw new Error("BOT_TOKEN is required");
}

const USER_POOL = buildUserPool(USERS_TOTAL);

export const options = {
  scenarios: {
    daily_load: {
      executor: "constant-arrival-rate",
      duration: TEST_DURATION,
      timeUnit: "1s",
      rate: Number(__ENV.RATE_PER_SEC || 5),
      preAllocatedVUs: Number(__ENV.PRE_ALLOCATED_VUS || 600),
      maxVUs: Number(__ENV.MAX_VUS || 5000),
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.02"],
    http_req_duration: ["p(95)<2500", "p(99)<5000"],
  },
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
};

export default function () {
  const profile = getActivityProfile();
  const actor = pickActor(profile.activeFraction);
  if (!actor) {
    sleep(randomBetween(0.3, 1.0));
    return;
  }

  const initData = buildTelegramInitData(actor);
  const action = chooseAction(profile.actionWeights);

  if (action === "feed") {
    callJSON("/api/miniapp/feed", { init_data: initData });
  } else if (action === "pack_group_feed") {
    callJSON("/api/miniapp/pack-group/feed", { init_data: initData });
  } else if (action === "pack_group_message") {
    callJSON("/api/miniapp/pack-group/messages", {
      init_data: initData,
      text: randomPackChatText(actor.user_id),
    });
  } else if (action === "leo_dm") {
    callJSON("/api/miniapp/messages", {
      init_data: initData,
      text: randomLeoText(actor.user_id),
    });
  } else if (action === "training_done") {
    callJSON("/api/miniapp/messages", {
      init_data: initData,
      text: randomTrainingDoneText(actor.user_id),
    });
  } else if (action === "profile_load") {
    callJSON("/api/miniapp/profile/load", { init_data: initData });
  }

  sleep(randomBetween(0.1, 1.5));
}

function callJSON(path, payload) {
  const res = http.post(`${BASE_URL}${path}`, JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    timeout: __ENV.REQUEST_TIMEOUT || "30s",
  });

  check(res, {
    [`${path} status is 2xx/4xx expected`]: (r) => r.status >= 200 && r.status < 500,
  });
}

function buildTelegramInitData(user) {
  const authDate = Math.floor(Date.now() / 1000).toString();
  const queryId = `AAE${user.user_id}_loadtest`;
  const userJson = JSON.stringify({
    id: user.user_id,
    first_name: user.first_name,
    last_name: user.last_name,
    username: user.username,
    language_code: "ru",
    is_premium: false,
  });

  const params = {
    auth_date: authDate,
    query_id: queryId,
    user: userJson,
  };

  if (CHAT_ID !== 0) {
    params.chat_type = "supergroup";
    params.chat_instance = CHAT_INSTANCE;
    params.chat = JSON.stringify({
      id: CHAT_ID,
      type: "supergroup",
      title: CHAT_TITLE,
      username: "",
    });
  }

  const dataCheckString = Object.keys(params)
    .sort()
    .map((k) => `${k}=${params[k]}`)
    .join("\n");

  const secret = crypto.hmac("sha256", "WebAppData", BOT_TOKEN, "binary");
  const hash = crypto.hmac("sha256", secret, dataCheckString, "hex");

  const query = Object.keys(params)
    .map((k) => `${k}=${encodeURIComponent(params[k])}`)
    .join("&");
  return `${query}&hash=${hash}`;
}

function buildUserPool(total) {
  const out = new Array(total);
  for (let i = 0; i < total; i += 1) {
    const id = 900000000 + i;
    out[i] = {
      user_id: id,
      first_name: `Leo${i}`,
      last_name: "Load",
      username: `leo_load_${i}`,
    };
  }
  return out;
}

function pickActor(activeFraction) {
  if (Math.random() > activeFraction) {
    return null;
  }
  const idx = Math.floor(Math.random() * USER_POOL.length);
  return USER_POOL[idx];
}

function getActivityProfile() {
  const elapsed = exec.scenario.startTime ? Date.now() - exec.scenario.startTime : 0;
  const hour = Math.floor((elapsed / 3600000) % 24);

  // Суточный профиль "стаи": ночь тихо, утро/вечер пик.
  if (hour >= 0 && hour < 6) {
    return {
      activeFraction: 0.10,
      actionWeights: {
        feed: 0.35,
        pack_group_feed: 0.20,
        pack_group_message: 0.10,
        leo_dm: 0.15,
        training_done: 0.12,
        profile_load: 0.08,
      },
    };
  }
  if (hour >= 6 && hour < 10) {
    return {
      activeFraction: 0.35,
      actionWeights: {
        feed: 0.25,
        pack_group_feed: 0.20,
        pack_group_message: 0.15,
        leo_dm: 0.10,
        training_done: 0.22,
        profile_load: 0.08,
      },
    };
  }
  if (hour >= 10 && hour < 18) {
    return {
      activeFraction: 0.25,
      actionWeights: {
        feed: 0.30,
        pack_group_feed: 0.18,
        pack_group_message: 0.18,
        leo_dm: 0.12,
        training_done: 0.15,
        profile_load: 0.07,
      },
    };
  }
  return {
    activeFraction: 0.45,
    actionWeights: {
      feed: 0.22,
      pack_group_feed: 0.18,
      pack_group_message: 0.24,
      leo_dm: 0.12,
      training_done: 0.18,
      profile_load: 0.06,
    },
  };
}

function chooseAction(weights) {
  const roll = Math.random();
  let cumulative = 0;
  const keys = Object.keys(weights);
  for (let i = 0; i < keys.length; i += 1) {
    const key = keys[i];
    cumulative += weights[key];
    if (roll <= cumulative) {
      return key;
    }
  }
  return keys[keys.length - 1];
}

function randomTrainingDoneText(userID) {
  const variants = [
    `#training_done 45 минут, user=${userID}`,
    `#training_done Сделал кардио и растяжку`,
    `#training_done Тренировка выполнена, самочувствие отличное`,
  ];
  return variants[Math.floor(Math.random() * variants.length)];
}

function randomPackChatText(userID) {
  const variants = [
    `Стая, я в деле! uid=${userID}`,
    "Делаем сегодня тренировку вместе?",
    "Кто уже закрыл норму за сегодня?",
    "@leo дай короткий мотивационный пинок",
  ];
  return variants[Math.floor(Math.random() * variants.length)];
}

function randomLeoText(userID) {
  const variants = [
    `@leo как держать дисциплину? uid=${userID}`,
    "@leo что делать, если пропустил один день?",
    "@leo дай мини-план на завтра",
  ];
  return variants[Math.floor(Math.random() * variants.length)];
}

function randomBetween(min, max) {
  return min + Math.random() * (max - min);
}

export function handleSummary(data) {
  const summary = {
    timestamp_utc: new Date().toISOString(),
    users_total: USERS_TOTAL,
    rate_per_sec: Number(__ENV.RATE_PER_SEC || 5),
    duration: TEST_DURATION,
    metrics: data.metrics,
  };
  return {
    stdout: `\nLoad test finished. Requests: ${data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0}\n`,
    "loadtest-summary.json": JSON.stringify(summary, null, 2),
  };
}
