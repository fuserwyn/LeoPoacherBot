//! MyVibeLab для macOS.
//!
//! Два окна: локальный экран входа (src/index.html) и окно с мини-аппом,
//! который отдаёт сам бот. Вся работа с сетью и Keychain живёт здесь, в Rust —
//! окно с удалённой страницей намеренно не получает доступа к командам (см.
//! capabilities/default.json: они выданы только окну login).

mod auth;
mod settings;

use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindowBuilder};
use tauri_plugin_opener::OpenerExt;

const LOGIN_LABEL: &str = "login";
const MAIN_LABEL: &str = "main";

fn desktop_tg_platform() -> &'static str {
    if cfg!(target_os = "windows") || cfg!(target_os = "linux") {
        "tdesktop"
    } else {
        "macos"
    }
}

#[derive(serde::Serialize)]
struct BeginResponse {
    nonce: String,
    link: String,
}

#[derive(serde::Serialize)]
struct SettingsView {
    base_url: String,
    bot_username: String,
    ready: bool,
}

#[tauri::command]
fn settings_get(app: AppHandle) -> SettingsView {
    let s = settings::load(&app);
    SettingsView {
        ready: s.is_ready(),
        base_url: s.base_url,
        bot_username: s.bot_username,
    }
}

#[tauri::command]
fn settings_set(app: AppHandle, url: String, bot: String) -> Result<SettingsView, String> {
    let s = settings::save(&app, &url, &bot)?;
    Ok(SettingsView {
        ready: s.is_ready(),
        base_url: s.base_url,
        bot_username: s.bot_username,
    })
}

/// Кто залогинен прямо сейчас (по Keychain), или None.
#[tauri::command]
fn session_status(app: AppHandle) -> Option<auth::User> {
    auth::load_session(&app).map(|s| s.user)
}

/// Начать вход: сгенерить nonce и открыть бота в системном Telegram.
#[tauri::command]
fn auth_begin(app: AppHandle) -> Result<BeginResponse, String> {
    let s = settings::load(&app);
    if !s.is_ready() {
        return Err("не задан адрес сервера или username бота".into());
    }
    let nonce = auth::new_nonce();
    let link = auth::deep_link(&s.bot_username, &nonce);
    open_externally(&app, &link)?;
    Ok(BeginResponse { nonce, link })
}

/// Ещё раз открыть ссылку подтверждения (если юзер закрыл Telegram).
#[tauri::command]
fn auth_open_link(app: AppHandle, link: String) -> Result<(), String> {
    open_externally(&app, &link)
}

#[tauri::command]
async fn auth_poll(app: AppHandle, nonce: String) -> Result<auth::PollOutcome, String> {
    let base = settings::load(&app).base_url;
    if base.is_empty() {
        return Err("не задан адрес сервера".into());
    }
    auth::poll(&app, &base, &nonce).await
}

#[tauri::command]
async fn auth_logout(app: AppHandle) -> Result<(), String> {
    let base = settings::load(&app).base_url;
    if let Some(session) = auth::load_session(&app) {
        auth::revoke(&base, &session.token).await;
    }
    auth::clear_session(&app);
    Ok(())
}

/// Открыть мини-апп и закрыть экран входа.
#[tauri::command]
fn open_app(app: AppHandle) -> Result<(), String> {
    open_main_window(&app)?;
    if let Some(login) = app.get_webview_window(LOGIN_LABEL) {
        let _ = login.close();
    }
    Ok(())
}

fn open_externally(app: &AppHandle, url: &str) -> Result<(), String> {
    app.opener()
        .open_url(url, None::<&str>)
        .map_err(|e| format!("не открылась ссылка: {e}"))
}

fn open_login_window(app: &AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window(LOGIN_LABEL) {
        let _ = win.set_focus();
        return Ok(());
    }
    let builder = WebviewWindowBuilder::new(app, LOGIN_LABEL, WebviewUrl::App("index.html".into()))
        .title("MyVibeLab");
    // Размеры и центрирование — только для десктопа. На телефоне вебвью должен
    // занять экран сам: заданный inner_size там применяется буквально, и окно
    // шире дисплея обрезает вёрстку.
    #[cfg(desktop)]
    let builder = builder
        .inner_size(460.0, 660.0)
        .min_inner_size(420.0, 480.0)
        .center();
    builder
        .build()
        .map_err(|e| format!("не открылось окно входа: {e}"))?;
    Ok(())
}

fn open_main_window(app: &AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window(MAIN_LABEL) {
        let _ = win.set_focus();
        return Ok(());
    }

    let session = auth::load_session(app).ok_or("нет сохранённой сессии")?;
    let base = settings::load(app).base_url;
    let url = tauri::Url::parse(&base).map_err(|e| format!("плохой адрес сервера: {e}"))?;
    let allowed_host = url.host_str().unwrap_or_default().to_string();

    let script = include_str!("telegram_shim.js")
        .replace(
            "__MVL_INIT_DATA__",
            &serde_json::to_string(&session.token).unwrap_or_else(|_| "\"\"".into()),
        )
        .replace(
            "__MVL_USER__",
            &serde_json::to_string(&session.user).unwrap_or_else(|_| "null".into()),
        )
        .replace(
            "__MVL_PLATFORM__",
            &serde_json::to_string(desktop_tg_platform()).unwrap_or_else(|_| "\"macos\"".into()),
        );

    let handle = app.clone();
    let builder = WebviewWindowBuilder::new(app, MAIN_LABEL, WebviewUrl::External(url))
        .title("MyVibeLab");
    #[cfg(desktop)]
    let builder = builder
        .inner_size(1180.0, 820.0)
        .min_inner_size(480.0, 600.0)
        .center();
    builder
        .initialization_script(script.as_str())
        .on_navigation(move |target| {
            let scheme = target.scheme();
            // tauri://, about:, data: — внутренние переходы вебвью.
            if scheme != "http" && scheme != "https" {
                return true;
            }
            let same_host = target
                .host_str()
                .map(|h| h.eq_ignore_ascii_case(&allowed_host))
                .unwrap_or(false);
            if same_host {
                return true;
            }
            // Всё чужое (t.me, github.com, ссылки на оплату) уходит в систему,
            // а переход отменяем — мини-апп остаётся на месте.
            let _ = handle.opener().open_url(target.as_str(), None::<&str>);
            false
        })
        .build()
        .map_err(|e| format!("не открылось окно приложения: {e}"))?;
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            settings_get,
            settings_set,
            session_status,
            auth_begin,
            auth_open_link,
            auth_poll,
            auth_logout,
            open_app,
        ])
        .setup(|app| {
            let handle = app.handle().clone();
            let s = settings::load(&handle);
            // Сессия есть и сервер известен — сразу мини-апп, иначе экран входа.
            let straight_to_app = s.is_ready() && auth::load_session(&handle).is_some();
            let opened = if straight_to_app {
                open_main_window(&handle).is_ok()
            } else {
                false
            };
            if !opened {
                open_login_window(&handle)?;
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
