//! Адрес мини-аппа Fat Leopard и username бота.
//!
//! Приоритет: переменная окружения (удобно для дева) → config.json в
//! Application Support (правится с экрана входа) → значение, вшитое при сборке
//! через LEO_BASE_URL / LEO_BOT.

use std::fs;
use std::path::PathBuf;

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager};

const DEFAULT_BOT: &str = "FatLeopard_Bot";
// Прод-адрес мини-аппа. Вшит, чтобы приложение работало сразу после установки:
// поле «Адрес сервера» остаётся только для дева и self-hosted сборок.
const DEFAULT_BASE_URL: &str = "https://fat-leopard-main.up.railway.app";

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Settings {
    #[serde(default)]
    pub base_url: String,
    #[serde(default)]
    pub bot_username: String,
}

impl Settings {
    /// Есть ли всё, чтобы вообще начинать логин.
    pub fn is_ready(&self) -> bool {
        !self.base_url.is_empty() && !self.bot_username.is_empty()
    }
}

fn config_path(app: &AppHandle) -> Option<PathBuf> {
    let dir = app.path().app_config_dir().ok()?;
    Some(dir.join("config.json"))
}

fn from_file(app: &AppHandle) -> Settings {
    config_path(app)
        .and_then(|p| fs::read_to_string(p).ok())
        .and_then(|raw| serde_json::from_str::<Settings>(&raw).ok())
        .unwrap_or_default()
}

/// https://host без завершающего слэша; схема добавляется, если её забыли.
pub fn normalize_url(raw: &str) -> String {
    let trimmed = raw.trim().trim_end_matches('/');
    if trimmed.is_empty() {
        return String::new();
    }
    if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        trimmed.to_string()
    } else {
        format!("https://{trimmed}")
    }
}

/// @FatLeopard_Bot и https://t.me/FatLeopard_Bot сводим к FatLeopard_Bot.
pub fn normalize_bot(raw: &str) -> String {
    raw.trim()
        .trim_start_matches("https://")
        .trim_start_matches("http://")
        .trim_start_matches("t.me/")
        .trim_start_matches('@')
        .trim_matches('/')
        .to_string()
}

pub fn load(app: &AppHandle) -> Settings {
    let stored = from_file(app);

    let base_url = std::env::var("LEO_BASE_URL")
        .ok()
        .filter(|v| !v.trim().is_empty())
        .unwrap_or_else(|| {
            if stored.base_url.is_empty() {
                option_env!("LEO_BASE_URL").unwrap_or(DEFAULT_BASE_URL).to_string()
            } else {
                stored.base_url.clone()
            }
        });

    let bot_username = std::env::var("LEO_BOT")
        .ok()
        .filter(|v| !v.trim().is_empty())
        .unwrap_or_else(|| {
            if stored.bot_username.is_empty() {
                option_env!("LEO_BOT").unwrap_or(DEFAULT_BOT).to_string()
            } else {
                stored.bot_username.clone()
            }
        });

    Settings {
        base_url: normalize_url(&base_url),
        bot_username: normalize_bot(&bot_username),
    }
}

pub fn save(app: &AppHandle, url: &str, bot: &str) -> Result<Settings, String> {
    let settings = Settings {
        base_url: normalize_url(url),
        bot_username: normalize_bot(bot),
    };
    let path = config_path(app).ok_or("не нашёл каталог конфигурации")?;
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| format!("не создался каталог конфигурации: {e}"))?;
    }
    let raw = serde_json::to_string_pretty(&settings).map_err(|e| e.to_string())?;
    fs::write(&path, raw).map_err(|e| format!("не записался config.json: {e}"))?;
    Ok(settings)
}
