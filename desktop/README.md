# Fat Leopard для macOS и Windows

Нативное окно (Tauri 2 + WKWebView / WebView2) вокруг того же Mini App, который бот отдаёт
из `bot/webapp.py`. Вход — через Telegram, без ввода пароля.

```bash
npm install
npm run dev         # запустить приложение
npm run build       # macOS: .app и .dmg в src-tauri/target/release/bundle
npm run build:win   # Windows: NSIS-установщик (собирать на Windows)
```

Нужен Rust: `curl https://sh.rustup.rs -sSf | sh`. Если `cargo` не в `PATH`,
добавь в `~/.zshrc`: `. "$HOME/.cargo/env"`.

## Как устроен вход

В обычном окне на маке нет `Telegram.WebApp`, поэтому подписанный `initData`
получить негде. Вместо него — подтверждение в чате с ботом:

1. приложение генерит одноразовый `nonce` (24 случайных байта, base64url);
2. открывает `https://t.me/<bot>?start=auth_<nonce>` в системном Telegram;
3. юзер жмёт в чате «Войти» — бот привязывает `nonce` к своему `telegram_id`
   и выдаёт токен сессии;
4. приложение опрашивает `/api/auth/desktop/poll?nonce=…` раз в 2 секунды
   (таймаут 5 минут) и забирает токен;
5. токен ложится в **Keychain** и подставляется в тот же заголовок
   `X-Init-Data`, что и `initData` внутри Telegram.

Подтверждение кнопкой в чате обязательно: без него любой, кто подсунет юзеру
ссылку с чужим `nonce`, войдёт под ним.

## Что должен реализовать сервер

Пока этих эндпоинтов в `bot/webapp.py` нет — это следующий шаг.

### `GET /api/auth/desktop/poll?nonce=<nonce>`

Без авторизации, поэтому должен быть исключён из `_auth_mw` (он требует
`X-Init-Data` на всё под `/api/`). Ответы:

| Ситуация | Код | Тело |
|---|---|---|
| бот ещё не получил `/start auth_<nonce>` | 404 или 200 | `{"status":"pending"}` |
| юзер подтвердил вход | 200 | `{"status":"ok","token":"mvl1_…","user":{"id":123,"first_name":"…","username":"…"}}` |
| `nonce` просрочен или уже использован | 410 | `{"status":"expired"}` |

Токен отдаётся **один раз**: после выдачи `nonce` гасится.

### `POST /api/auth/desktop/logout`

Принимает токен в `X-Init-Data`, отзывает его. Ошибки приложение игнорирует —
Keychain чистится локально в любом случае.

### Проверка токена в `_auth_mw`

`_verify_init_data` проверяет HMAC от `initData` и токен не поймёт, поэтому
нужна вторая ветка. Различать по префиксу: `initData` — это query-строка с
`hash=`, десктопный токен — `mvl1_<случайное>`.

### Хендлер `/start auth_<nonce>`

`on_start` в `bot/main.py` уже разбирает deep-link payload через
`referrals.parse_start_payload` (префикс `ref`), так что ветка `auth_` рядом.
Логика: показать кнопку «Войти на Mac», по нажатию создать сессию.

Разумные ограничения: TTL `nonce` ~5 минут, формат `^[A-Za-z0-9_-]{22,64}$`,
rate-limit опроса по IP, в базе хранить `sha256` токена, а не сам токен.

## Настройки

Адрес сервера и username бота ищутся по порядку:

1. переменные окружения `MYVIBELAB_BASE_URL`, `MYVIBELAB_BOT` — для дева;
2. `~/Library/Application Support/app.myvibelab.desktop/config.json` — правится
   на экране входа в разделе «Настройки подключения»;
3. значения, вшитые при сборке: `MYVIBELAB_BASE_URL=… npm run build`.

Адрес — тот же, что у кнопки «Открыть IDE» в боте (`WEBAPP_URL`). Для отладки
можно указать локальный `http://127.0.0.1:8080`.

## Устройство

```
src/                    экран входа (локальный, без сборщика)
src-tauri/src/
  lib.rs                окна, команды, перехват навигации
  auth.rs               nonce, опрос сервера, Keychain
  settings.rs           адрес сервера и бот
  telegram_shim.js      эмуляция Telegram.WebApp для окна мини-аппа
```

Два окна с разными правами:

- `login` — локальная страница, только ей выданы команды Tauri
  (`capabilities/default.json`);
- `main` — удалённая страница мини-аппа, **без** доступа к IPC. Перед загрузкой
  в неё инжектится `telegram_shim.js` с токеном в `Telegram.WebApp.initData`,
  поэтому SPA работает без единой правки.

Ссылки на чужие origin (t.me, github.com, оплата) перехватывает `on_navigation`
в `lib.rs`: открывает их в системе и отменяет переход, чтобы мини-апп не
уезжал со своей страницы и не терял состояние. Из-за этого `tg.openInvoice`
намеренно не реализован — мини-апп сам уходит в ветку «оплати по ссылке».

## Подпись и распространение

На Windows `npm run build:win` даёт NSIS-установщик
`src-tauri/target/release/bundle/nsis/*.exe`. WebView2 подтянется сам, если его
ещё нет. Сборку с Mac не кросс-компилируем — ставит workflow
`.github/workflows/desktop-windows.yml` (артефакт `Fat Leopard-windows`).

`npm run build` даёт `.dmg`, но без подписи Gatekeeper на чужой машине скажет
«не удаётся проверить разработчика». Варианты:

- для себя и беты: `xattr -dr com.apple.quarantine /Applications/Fat Leopard.app`;
- для людей: Apple Developer ($99/год), сертификат Developer ID Application,
  затем подпись с hardened runtime, `notarytool submit --wait` и
  `xcrun stapler staple`. Tauri умеет это сам через переменные
  `APPLE_SIGNING_IDENTITY`, `APPLE_ID`, `APPLE_PASSWORD`, `APPLE_TEAM_ID`.

## Что дальше

- эндпоинты и хендлер `/start auth_…` на стороне бота;
- локальный режим: агент правит папку на маке, а не клон в контейнере
  (в конфиге бота уже есть `CURSOR_RUNTIME=local`);
- автообновление (`tauri-plugin-updater`) — иначе юзеры останутся на первой версии.
