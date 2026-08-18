//! Логин через Telegram без initData.
//!
//! В обычном окне на маке объекта Telegram.WebApp нет, поэтому подтвердить
//! личность нечем. Схема такая:
//!
//!   1. приложение генерит одноразовый nonce;
//!   2. открывает t.me/<bot>?start=auth_<nonce> в системе — юзер жмёт «Войти» в чате;
//!   3. бот привязывает nonce к своему telegram_id и выдаёт токен сессии;
//!   4. приложение опрашивает /api/auth/desktop/poll и забирает токен;
//!   5. токен ложится в Keychain и дальше уезжает в тот же заголовок X-Init-Data,
//!      что и initData в Telegram — сервер различает их по префиксу токена.
//!
//! Nonce живёт только в памяти процесса: перезапуск во время логина = новый вход.

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use rand::RngCore;
use serde::{Deserialize, Serialize};
#[cfg(target_os = "android")]
use tauri::Manager as _;
use tauri::AppHandle;

/// Префикс start-payload. Telegram разрешает в нём только [A-Za-z0-9_-] до 64 символов,
/// поэтому nonce — base64url без padding.
const PAYLOAD_PREFIX: &str = "auth_";
const KEYCHAIN_SERVICE: &str = "app.fatleopard.desktop";
const KEYCHAIN_ACCOUNT: &str = "session";
const POLL_PATH: &str = "/api/miniapp/auth/desktop/poll";
const LOGOUT_PATH: &str = "/api/miniapp/auth/desktop/logout";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: i64,
    #[serde(default)]
    pub first_name: String,
    #[serde(default)]
    pub username: Option<String>,
    #[serde(default)]
    pub photo_url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub token: String,
    pub user: User,
}

/// Что вернул опрос. `Pending` — юзер ещё не подтвердил в чате.
#[derive(Debug, Clone, Serialize)]
#[serde(tag = "status", rename_all = "lowercase")]
pub enum PollOutcome {
    Pending,
    Expired,
    Ok { user: User },
}

#[derive(Debug, Deserialize)]
struct PollBody {
    #[serde(default)]
    status: String,
    #[serde(default)]
    token: Option<String>,
    #[serde(default)]
    user: Option<User>,
}

pub fn new_nonce() -> String {
    let mut bytes = [0u8; 24];
    rand::thread_rng().fill_bytes(&mut bytes);
    URL_SAFE_NO_PAD.encode(bytes)
}

pub fn deep_link(bot: &str, nonce: &str) -> String {
    format!("https://t.me/{bot}?start={PAYLOAD_PREFIX}{nonce}")
}

fn client() -> Result<reqwest::Client, String> {
    reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(15))
        .user_agent(concat!("MyVibeLab-Desktop/", env!("CARGO_PKG_VERSION")))
        .build()
        .map_err(|e| format!("не поднялся HTTP-клиент: {e}"))
}

/// Один опрос сервера. Токен при успехе сразу уходит в Keychain, наружу его не
/// отдаём — фронту экрана входа он не нужен.
pub async fn poll(app: &AppHandle, base_url: &str, nonce: &str) -> Result<PollOutcome, String> {
    let url = format!("{base_url}{POLL_PATH}?nonce={nonce}");
    let resp = client()?
        .get(&url)
        .send()
        .await
        .map_err(|e| format!("сервер недоступен: {e}"))?;

    let status = resp.status();
    // 404 — бот ещё не получил /start с этим nonce, это нормальное ожидание.
    if status == reqwest::StatusCode::NOT_FOUND || status == reqwest::StatusCode::TOO_MANY_REQUESTS {
        return Ok(PollOutcome::Pending);
    }
    if status == reqwest::StatusCode::GONE {
        return Ok(PollOutcome::Expired);
    }
    if !status.is_success() {
        return Err(format!("сервер ответил {}", status.as_u16()));
    }

    let body: PollBody = resp
        .json()
        .await
        .map_err(|e| format!("не разобрал ответ сервера: {e}"))?;

    match body.status.as_str() {
        "pending" => Ok(PollOutcome::Pending),
        "expired" => Ok(PollOutcome::Expired),
        "ok" => {
            let token = body.token.filter(|t| !t.is_empty()).ok_or("сервер не отдал токен")?;
            let user = body.user.ok_or("сервер не отдал пользователя")?;
            save_session(app, &Session { token, user: user.clone() })?;
            Ok(PollOutcome::Ok { user })
        }
        other => Err(format!("неизвестный статус входа: {other}")),
    }
}

/// Отозвать токен на сервере. Ошибку глотаем: локально из Keychain всё равно чистим.
pub async fn revoke(base_url: &str, token: &str) {
    if base_url.is_empty() {
        return;
    }
    if let Ok(c) = client() {
        let _ = c
            .post(format!("{base_url}{LOGOUT_PATH}"))
            .header("X-Init-Data", token)
            .send()
            .await;
    }
}

// Где лежит сессия. На Apple-платформах — Keychain. На Android его нет, и
// keyring там остаётся без бэкенда: запись молча уходит в никуда, а вход не
// переживает даже переход к окну мини-аппа. Поэтому там пишем в приватный
// каталог приложения — на неперепрошитом устройстве он доступен только нам.
// Это слабее Keychain, но единственная рабочая альтернатива без Android Keystore.

#[cfg(not(target_os = "android"))]
fn entry() -> Result<keyring::Entry, String> {
    keyring::Entry::new(KEYCHAIN_SERVICE, KEYCHAIN_ACCOUNT)
        .map_err(|e| format!("Keychain недоступен: {e}"))
}

#[cfg(not(target_os = "android"))]
pub fn load_session(_app: &AppHandle) -> Option<Session> {
    let raw = entry().ok()?.get_password().ok()?;
    serde_json::from_str(&raw).ok()
}

#[cfg(not(target_os = "android"))]
pub fn save_session(_app: &AppHandle, session: &Session) -> Result<(), String> {
    let raw = serde_json::to_string(session).map_err(|e| e.to_string())?;
    entry()?
        .set_password(&raw)
        .map_err(|e| format!("не записал сессию в Keychain: {e}"))
}

#[cfg(not(target_os = "android"))]
pub fn clear_session(_app: &AppHandle) {
    if let Ok(e) = entry() {
        let _ = e.delete_credential();
    }
}

#[cfg(target_os = "android")]
fn session_path(app: &AppHandle) -> Result<std::path::PathBuf, String> {
    let dir = app
        .path()
        .app_data_dir()
        .map_err(|e| format!("нет каталога данных приложения: {e}"))?;
    std::fs::create_dir_all(&dir).map_err(|e| format!("не создался каталог данных: {e}"))?;
    Ok(dir.join("session.json"))
}

#[cfg(target_os = "android")]
pub fn load_session(app: &AppHandle) -> Option<Session> {
    let raw = std::fs::read_to_string(session_path(app).ok()?).ok()?;
    serde_json::from_str(&raw).ok()
}

#[cfg(target_os = "android")]
pub fn save_session(app: &AppHandle, session: &Session) -> Result<(), String> {
    let raw = serde_json::to_string(session).map_err(|e| e.to_string())?;
    std::fs::write(session_path(app)?, raw).map_err(|e| format!("не записалась сессия: {e}"))
}

#[cfg(target_os = "android")]
pub fn clear_session(app: &AppHandle) {
    if let Ok(p) = session_path(app) {
        let _ = std::fs::remove_file(p);
    }
}
