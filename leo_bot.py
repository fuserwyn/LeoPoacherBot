import asyncio
import logging
import os
from datetime import datetime, timedelta

import aiosqlite
from aiogram import Bot, Dispatcher, types
from aiogram.enums import ChatMemberStatus
from dotenv import load_dotenv

load_dotenv()

API_TOKEN = os.getenv("API_TOKEN")

bot = Bot(token=API_TOKEN)
dp = Dispatcher()

# Словарь для отслеживания запланированных удалений пользователей
scheduled_removals = {}

DB_NAME = "training.db"


async def init_db():
    async with aiosqlite.connect(DB_NAME) as db:
        # Таблица для отчетов
        await db.execute('''
            CREATE TABLE IF NOT EXISTS training_log (
                user_id INTEGER PRIMARY KEY,
                last_report TEXT
            )
        ''')
        # Таблица для отслеживания последних сообщений
        await db.execute('''
            CREATE TABLE IF NOT EXISTS message_log (
                user_id INTEGER,
                chat_id INTEGER,
                last_message TEXT,
                has_training_done BOOLEAN DEFAULT FALSE,
                PRIMARY KEY (user_id, chat_id)
            )
        ''')
        await db.commit()


@dp.chat_member()
async def handle_chat_member_updated(event: types.ChatMemberUpdated):
    """Обрабатывает добавление новых участников в чат"""
    if event.new_chat_member.status == ChatMemberStatus.MEMBER:
        user_id = event.new_chat_member.user.id
        chat_id = event.chat.id
        username = event.new_chat_member.user.username or event.new_chat_member.user.first_name
        
        logging.info(f"Новый участник добавлен: {user_id} (@{username})")
        
        # Отправляем персональное приветствие новому участнику
        try:
            welcome_message = f"👋 **Добро пожаловать, @{username}!**\n\n🤖 **Я Fat Leopard - ваш тренер!**\n\n⏰ **Таймер уже запущен!**\n• У вас есть **7 дней** чтобы отправить `#training_done`\n• Через **6 дней** вы получите предупреждение\n• Через **7 дней** без отчета - удаление из чата\n\n💪 **Отправьте `#training_done` прямо сейчас!**"
            
            await bot.send_message(chat_id, welcome_message, parse_mode="Markdown")
            logging.info(f"Отправлено приветствие новому участнику {user_id} (@{username})")
        except Exception as e:
            logging.error(f"Ошибка при отправке приветствия пользователю {user_id}: {e}")
        
        # Планируем удаление через 7 дней для нового участника
        logging.info(f"⏰ ЗАПУСК ТАЙМЕРА для нового участника {user_id} (@{username})")
        warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60))  # 6 дней = 6 * 24 * 60 * 60 секунд
        removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60))  # 7 дней = 7 * 24 * 60 * 60 секунд
        scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
        logging.info(f"✅ Таймер запущен для нового участника {user_id} (@{username}) - предупреждение через 6 дней, удаление через 7 дней")


@dp.message()
async def handle_message(msg: types.Message):
    # Обработка команды /help
    if msg.text and msg.text.startswith("/help"):
        help_text = """
🤖 **LeoPoacherBot - Команды:**

💪 **Отчеты о тренировке:**
• `#training_done` - Отправить отчет о тренировке

⏰ **Как работает бот:**
• При добавлении бота в чат автоматически запускаются таймеры для всех участников
• При получении `#training_done` таймер перезапускается на 7 дней
• Через 6 дней без `#training_done` - предупреждение
• Через 7 дней без `#training_done` - удаление из чата

🔧 **Требования:**
• Бот должен быть администратором чата для полного функционала
"""
        await msg.reply(help_text, parse_mode="Markdown")
        return

    # Обработка обычных сообщений
    chat_id = msg.chat.id
    user_id = msg.from_user.id
    current_time = datetime.utcnow()
    has_training_done = msg.text and "#training_done" in msg.text.lower()
    
    logging.info(f"Получено сообщение от {user_id}: '{msg.text}' (has_training_done: {has_training_done})")

    # Автоматически сохраняем информацию о сообщении
    try:
        async with aiosqlite.connect(DB_NAME) as db:
            # Сохраняем информацию о сообщении
            await db.execute('''
                INSERT OR REPLACE INTO message_log (user_id, chat_id, last_message, has_training_done)
                VALUES (?, ?, ?, ?)
            ''', (user_id, chat_id, current_time.isoformat(), has_training_done))
            await db.commit()
            logging.info(f"Сообщение пользователя {user_id} в чате {chat_id} сохранено в БД")
    except Exception as e:
        logging.error(f"Ошибка при сохранении сообщения: {e}")

    # Автоматически запускаем таймер для нового пользователя
    if user_id not in scheduled_removals:
        username = msg.from_user.username or msg.from_user.first_name
        logging.info(f"🆕 НОВЫЙ ПОЛЬЗОВАТЕЛЬ: Запускаем таймер для пользователя {user_id} (@{username}) при первом сообщении")
        warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60))  # 6 дней
        removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60))  # 7 дней
        scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
        logging.info(f"⏰ Таймер запущен для нового пользователя {user_id} (@{username})")
        
        # Отправляем приветствие новому пользователю
        try:
            welcome_message = f"👋 **Привет, @{username}!**\n\n🤖 **Я Fat Leopard - ваш тренер!**\n\n⏰ **Таймер запущен!**\n• У вас есть **7 дней** чтобы отправить `#training_done`\n• Через **6 дней** вы получите предупреждение\n• Через **7 дней** без отчета - удаление из чата\n\n💪 **Отправьте `#training_done` прямо сейчас!**"
            await bot.send_message(chat_id, welcome_message, parse_mode="Markdown")
            logging.info(f"Отправлено приветствие новому пользователю {user_id} (@{username})")
        except Exception as e:
            logging.error(f"Ошибка при отправке приветствия пользователю {user_id}: {e}")

    if has_training_done:
        # Если это отчет о тренировке, сохраняем в training_log
        async with aiosqlite.connect(DB_NAME) as db:
            await db.execute('''
                INSERT OR REPLACE INTO training_log (user_id, last_report)
                VALUES (?, ?)
            ''', (user_id, current_time.isoformat()))
            await db.commit()
        logging.info(f"Отчет о тренировке сохранен для пользователя {user_id}")
        try:
            await msg.reply("✅ **Отчёт принят!** 💪\n\n⏰ Таймер перезапущен на 7 дней\n\n🎯 Продолжай тренироваться и не забывай отправлять `#training_done`!")
            logging.info(f"Ответ 'Отчёт принят' отправлен пользователю {user_id}")
        except Exception as e:
            logging.error(f"Ошибка при отправке ответа пользователю {user_id}: {e}")
        
        # Запускаем новый персональный таймер на 7 дней после #training_done
        username = msg.from_user.username or msg.from_user.first_name
        timer_start_time = current_time.isoformat()
        logging.info(f"ЗАПУСК НОВОГО ТАЙМЕРА: После #training_done планируем удаление пользователя {user_id} (@{username}) через 7 дней")
        warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60, timer_start_time))  # 6 дней
        removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60, timer_start_time))  # 7 дней
        scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
        logging.info(f"Новый персональный таймер запущен для пользователя {user_id} - предупреждение через 6 дней, удаление через 7 дней")
    else:
        # Игнорируем обычные сообщения - они не влияют на таймер
        logging.info(f"ОБЫЧНОЕ СООБЩЕНИЕ: Игнорируем сообщение от пользователя {user_id} - таймер не перезапускается")


def cancel_user_removal(user_id: int):
    """Отменяет запланированное удаление пользователя"""
    if user_id in scheduled_removals:
        tasks = scheduled_removals[user_id]
        if isinstance(tasks, dict):
            # Новая структура с предупреждением и удалением
            tasks["warning"].cancel()
            tasks["removal"].cancel()
        else:
            # Старая структура (для обратной совместимости)
            tasks.cancel()
        del scheduled_removals[user_id]
        logging.info(f"Отменено удаление пользователя {user_id}")


async def schedule_user_warning(user_id: int, chat_id: int, username: str, delay_seconds: int, timer_start_time: str = None):
    """Планирует отправку предупреждения пользователю"""
    logging.info(f"ФУНКЦИЯ ПРЕДУПРЕЖДЕНИЯ: Начинаем отсчет {delay_seconds} секунд для пользователя {user_id}")
    try:
        await asyncio.sleep(delay_seconds)
        logging.info(f"ФУНКЦИЯ ПРЕДУПРЕЖДЕНИЯ: Отсчет завершен для пользователя {user_id}, проверяем #training_done")
        
        # Проверяем, написал ли пользователь #training_done за это время
        async with aiosqlite.connect(DB_NAME) as db:
            if timer_start_time:
                # Проверяем только те #training_done, которые были написаны после запуска таймера
                async with db.execute('''
                    SELECT has_training_done FROM message_log 
                    WHERE user_id = ? AND has_training_done = 1 AND last_message > ?
                    ORDER BY last_message DESC LIMIT 1
                ''', (user_id, timer_start_time)) as cursor:
                    row = await cursor.fetchone()
                    if row and row[0]:
                        # Пользователь написал #training_done после запуска таймера, не отправляем предупреждение
                        logging.info(f"Пользователь {user_id} написал #training_done после запуска таймера, предупреждение отменено")
                        return
            else:
                # Старая логика для обратной совместимости
                async with db.execute('''
                    SELECT has_training_done FROM message_log WHERE user_id = ? AND has_training_done = 1
                    ORDER BY last_message DESC LIMIT 1
                ''', (user_id,)) as cursor:
                    row = await cursor.fetchone()
                    if row and row[0]:
                        # Пользователь написал #training_done, не отправляем предупреждение
                        logging.info(f"Пользователь {user_id} написал #training_done, предупреждение отменено")
                        return
        
        # Отправляем предупреждение
        try:
            mention = f"@{username}" if username.startswith("@") else f"@{username}"
            warning_message = f"{mention} ⚠️ **ПРЕДУПРЕЖДЕНИЕ!**\n\n⏰ У тебя остался ровно 1 день!\n\n💪 Отправь `#training_done` прямо сейчас, иначе будешь удален из чата!\n\n💀 **Время истекает!**"
            await bot.send_message(chat_id, warning_message, parse_mode="Markdown")
            logging.info(f"Отправлено предупреждение пользователю {user_id}")
        except Exception as e:
            logging.warning(f"Ошибка при отправке предупреждения {user_id}: {e}")
    except asyncio.CancelledError:
        # Задача была отменена
        logging.info(f"Предупреждение пользователю {user_id} отменено")
    finally:
        # Очищаем задачу из словаря
        if user_id in scheduled_removals and isinstance(scheduled_removals[user_id], dict):
            if "warning" in scheduled_removals[user_id]:
                del scheduled_removals[user_id]["warning"]


async def schedule_user_removal(user_id: int, chat_id: int, delay_seconds: int, timer_start_time: str = None):
    """Планирует удаление пользователя через указанное время, если он не написал #training_done"""
    try:
        await asyncio.sleep(delay_seconds)
        
        # Проверяем, написал ли пользователь #training_done за это время
        async with aiosqlite.connect(DB_NAME) as db:
            if timer_start_time:
                # Проверяем только те #training_done, которые были написаны после запуска таймера
                async with db.execute('''
                    SELECT has_training_done FROM message_log 
                    WHERE user_id = ? AND has_training_done = 1 AND last_message > ?
                    ORDER BY last_message DESC LIMIT 1
                ''', (user_id, timer_start_time)) as cursor:
                    row = await cursor.fetchone()
                    if row and row[0]:
                        # Пользователь написал #training_done после запуска таймера, не удаляем
                        logging.info(f"Пользователь {user_id} написал #training_done после запуска таймера, удаление отменено")
                        return
            else:
                # Старая логика для обратной совместимости
                async with db.execute('''
                    SELECT has_training_done FROM message_log WHERE user_id = ? AND has_training_done = 1
                    ORDER BY last_message DESC LIMIT 1
                ''', (user_id,)) as cursor:
                    row = await cursor.fetchone()
                    if row and row[0]:
                        # Пользователь написал #training_done, не удаляем
                        logging.info(f"Пользователь {user_id} написал #training_done, удаление отменено")
                        return
        
        # Если пользователь не написал #training_done, удаляем его
        try:
            await bot.ban_chat_member(chat_id, user_id)
            await bot.unban_chat_member(chat_id, user_id)  # чтобы могли вернуться
            
            # Отправляем сообщение об удалении
            removal_message = f"❌ **Пользователь удален из чата!**\n\n⏰ Время истекло - не было отправлено `#training_done`\n\n🔄 Можешь вернуться в чат, но помни про правила!"
            await bot.send_message(chat_id, removal_message, parse_mode="Markdown")
            
            logging.info(f"Удалён пользователь {user_id} за отсутствие #training_done")
        except Exception as e:
            logging.warning(f"Ошибка при удалении {user_id}: {e}")
    except asyncio.CancelledError:
        # Задача была отменена
        logging.info(f"Удаление пользователя {user_id} отменено")
    finally:
        # Очищаем задачу из словаря
        if user_id in scheduled_removals:
            if isinstance(scheduled_removals[user_id], dict):
                if "removal" in scheduled_removals[user_id]:
                    del scheduled_removals[user_id]["removal"]
            else:
                del scheduled_removals[user_id]


async def main():
    logging.info("Инициализация базы данных...")
    await init_db()
    logging.info("Старт бота...")
    await dp.start_polling(bot)


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')
    asyncio.run(main())
