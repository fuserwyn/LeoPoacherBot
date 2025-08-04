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
OWNER_ID = int(os.getenv("OWNER_ID", "0"))

bot = Bot(token=API_TOKEN)
dp = Dispatcher()

# Словарь для отслеживания запланированных удалений пользователей
scheduled_removals = {}

DB_NAME = "training.db"


async def init_db():
    """Инициализирует базу данных"""
    async with aiosqlite.connect(DB_NAME) as db:
        # Создаем таблицу для логирования сообщений
        await db.execute('''
            CREATE TABLE IF NOT EXISTS message_log (
                user_id INTEGER,
                chat_id INTEGER,
                last_message TEXT,
                has_training_done BOOLEAN DEFAULT FALSE,
                has_sick_leave BOOLEAN DEFAULT FALSE,
                has_healthy BOOLEAN DEFAULT FALSE,
                timer_start_time TEXT,
                sick_leave_start_time TEXT,
                PRIMARY KEY (user_id, chat_id)
            )
        ''')
        
        # Создаем таблицу для отчетов о тренировках
        await db.execute('''
            CREATE TABLE IF NOT EXISTS training_log (
                user_id INTEGER PRIMARY KEY,
                last_report TEXT
            )
        ''')
        
        # Безопасная миграция для добавления поля has_sick_leave
        try:
            # Проверяем, существует ли поле has_sick_leave
            await db.execute('''
                SELECT has_sick_leave FROM message_log LIMIT 1
            ''')
            logging.info("Поле has_sick_leave уже существует в базе данных")
        except aiosqlite.OperationalError:
            # Поле не существует, добавляем его
            logging.info("Добавляем поле has_sick_leave в существующую базу данных")
            await db.execute('''
                ALTER TABLE message_log ADD COLUMN has_sick_leave BOOLEAN DEFAULT FALSE
            ''')
            logging.info("Поле has_sick_leave успешно добавлено")
        
        # Безопасная миграция для добавления поля has_healthy
        try:
            # Проверяем, существует ли поле has_healthy
            await db.execute('''
                SELECT has_healthy FROM message_log LIMIT 1
            ''')
            logging.info("Поле has_healthy уже существует в базе данных")
        except aiosqlite.OperationalError:
            # Поле не существует, добавляем его
            logging.info("Добавляем поле has_healthy в существующую базу данных")
            await db.execute('''
                ALTER TABLE message_log ADD COLUMN has_healthy BOOLEAN DEFAULT FALSE
            ''')
            logging.info("Поле has_healthy успешно добавлено")
        
        # Безопасная миграция для добавления поля timer_start_time
        try:
            # Проверяем, существует ли поле timer_start_time
            await db.execute('''
                SELECT timer_start_time FROM message_log LIMIT 1
            ''')
            logging.info("Поле timer_start_time уже существует в базе данных")
        except aiosqlite.OperationalError:
            # Поле не существует, добавляем его
            logging.info("Добавляем поле timer_start_time в существующую базу данных")
            await db.execute('''
                ALTER TABLE message_log ADD COLUMN timer_start_time TEXT
            ''')
            logging.info("Поле timer_start_time успешно добавлено")
        
        # Безопасная миграция для добавления поля sick_leave_start_time
        try:
            # Проверяем, существует ли поле sick_leave_start_time
            await db.execute('''
                SELECT sick_leave_start_time FROM message_log LIMIT 1
            ''')
            logging.info("Поле sick_leave_start_time уже существует в базе данных")
        except aiosqlite.OperationalError:
            # Поле не существует, добавляем его
            logging.info("Добавляем поле sick_leave_start_time в существующую базу данных")
            await db.execute('''
                ALTER TABLE message_log ADD COLUMN sick_leave_start_time TEXT
            ''')
            logging.info("Поле sick_leave_start_time успешно добавлено")
        
        await db.commit()
        logging.info("База данных инициализирована")


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
            welcome_message = f"👋 Добро пожаловать, @{username}!**\n\n🤖 Я Fat Leopard - местный тренер!\n\n⏰ Таймер уже запущен!\n• У тебя есть 7 дней чтобы отправить `#training_done`\n• Через 6 дней ты получишь предупреждение\n• Через 7 дней без отчета - удаление из чата\n\n💪 **Отправьте `#training_done` прямо сейчас!"
            
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
🤖 **LeoPoacherBot — Инструкция и правила**

👋 **Для новичков:**
• После добавления в чат бот автоматически запускает для вас таймер.
• Ваша задача — регулярно тренироваться и отправлять отчёты с тегом `#training_done`.
• Если не отправить отчёт — станете "Толстым Леопардом" и будете удалены из чата!

• `/help` — Показать это сообщение

💪 **Отчёты о тренировке:**
• `#training_done` — Отправить отчёт о тренировке (можно в тексте или подписи к фото/видео)
• `#sick_leave` — Взять больничный (таймер приостанавливается)
• `#healthy` — Сообщить о выздоровлении (таймер возобновляется с места остановки)

📜 **Правила:**
• Отчётом считается любое сообщение с тегом `#training_done` (текст, фото, видео, аудио).
• Если заболели или уехали — отправьте `#sick_leave`, чтобы приостановить таймер.
• После выздоровления — отправьте `#healthy`, чтобы продолжить участие с того места где прервали.
• Через 6 дней без отчёта — предупреждение.
• Через 7 дней без отчёта — удаление из чата.

⏰ **Как работает бот:**
• При добавлении бота в чат автоматически запускаются таймеры для всех участников.
• Каждый отчёт с `#training_done` перезапускает таймер на 7 дней.
• Больничный приостанавливает таймер, выздоровление возобновляет с места остановки.
• Если время истекло во время больничного — запускается новый таймер.
"""
        await msg.reply(help_text, parse_mode="Markdown")
        return

    # Обработка команды /db
    if msg.text and msg.text.startswith("/db"):
        chat_id = msg.chat.id
        
        # Проверяем, является ли отправитель администратором
        try:
            chat_member = await bot.get_chat_member(chat_id, msg.from_user.id)
            if chat_member.status not in [ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.CREATOR]:
                await msg.reply("❌ Только администраторы могут использовать эту команду!")
                return
        except Exception as e:
            logging.warning(f"Ошибка при проверке прав администратора: {e}")
            return

        try:
            # Получаем данные из базы данных
            async with aiosqlite.connect(DB_NAME) as db:
                # Получаем всех пользователей в этом чате
                async with db.execute('''
                    SELECT user_id, last_message, has_training_done, has_sick_leave, has_healthy, timer_start_time, sick_leave_start_time
                    FROM message_log 
                    WHERE chat_id = ?
                    ORDER BY last_message DESC
                ''', (chat_id,)) as cursor:
                    users = await cursor.fetchall()
                
                # Получаем отчеты о тренировках
                async with db.execute('''
                    SELECT user_id, last_report
                    FROM training_log
                    ORDER BY last_report DESC
                ''') as cursor:
                    training_reports = await cursor.fetchall()
            
            if not users:
                await msg.reply("📊 **База данных пуста**\n\nВ этом чате пока нет пользователей.")
                return
            
            # Формируем отчет
            db_report = f"📊 **База данных чата**\n\n"
            db_report += f"👥 **Пользователей в чате:** {len(users)}\n"
            db_report += f"📝 **Отчетов о тренировках:** {len(training_reports)}\n\n"
            
            # Показываем последние 10 пользователей
            db_report += f"🕐 **Последние активности:**\n"
            for i, (user_id, last_message, has_training_done, has_sick_leave, has_healthy, timer_start_time, sick_leave_start_time) in enumerate(users[:10], 1):
                status_emoji = "✅" if has_training_done else "⏰" if has_sick_leave else "💪" if has_healthy else "📝"
                timer_info = ""
                if timer_start_time:
                    timer_info = " ⏰"
                if sick_leave_start_time:
                    timer_info += " 🏥"
                db_report += f"{i}. ID: {user_id} {status_emoji}{timer_info}\n"
            
            if len(users) > 10:
                db_report += f"\n... и еще {len(users) - 10} пользователей"
            
            await msg.reply(db_report, parse_mode="Markdown")
            logging.info(f"Администратор {msg.from_user.id} просмотрел базу данных чата {chat_id}")
            
        except Exception as e:
            logging.error(f"Ошибка при просмотре базы данных: {e}")
            await msg.reply("❌ Ошибка при просмотре базы данных")
        return

    # Обработка команды /start_timer
    if msg.text and msg.text.startswith("/start_timer"):
        chat_id = msg.chat.id
        
        # Проверяем, является ли отправитель администратором
        try:
            chat_member = await bot.get_chat_member(chat_id, msg.from_user.id)
            if chat_member.status not in [ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.CREATOR]:
                await msg.reply("❌ Только администраторы могут использовать эту команду!")
                return
        except Exception as e:
            logging.warning(f"Ошибка при проверке прав администратора: {e}")
            return

        try:
            await msg.reply("🐆 **Fat Leopard активирован!**\n\n⏳ Запускаю таймеры для всех участников...", parse_mode="Markdown")
            
            # Получаем всех пользователей из базы данных
            async with aiosqlite.connect(DB_NAME) as db:
                async with db.execute('''
                    SELECT DISTINCT user_id FROM message_log 
                    WHERE chat_id = ?
                    ORDER BY last_message DESC
                ''', (chat_id,)) as cursor:
                    db_users = await cursor.fetchall()
            
            # if not db_users:
            #     await msg.reply("⚠️ В базе данных нет пользователей для этого чата")
            #     return
            
            current_time = datetime.utcnow().isoformat()
            started_timers = 0
            failed_users = []
            
            for (user_id,) in db_users:
                try:
                    # Проверяем, что пользователь все еще в чате
                    chat_member = await bot.get_chat_member(chat_id, user_id)
                    if chat_member.status in [ChatMemberStatus.MEMBER, ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.CREATOR]:
                        username = chat_member.user.username or chat_member.user.first_name
                        
                        # Обновляем время в базе данных
                        async with aiosqlite.connect(DB_NAME) as db:
                            await db.execute('''
                                UPDATE message_log 
                                SET last_message = ?, has_training_done = FALSE, has_sick_leave = FALSE, has_healthy = FALSE, timer_start_time = ?, sick_leave_start_time = NULL
                                WHERE user_id = ? AND chat_id = ?
                            ''', (current_time, current_time, user_id, chat_id))
                            await db.commit()
                        
                        # Отменяем существующие таймеры
                        if user_id in scheduled_removals:
                            cancel_user_removal(user_id)
                        
                        # Запускаем новые таймеры
                        warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60))
                        removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60))
                        scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
                        
                        started_timers += 1
                        logging.info(f"⏰ Запущен таймер для: {user_id} (@{username})")
                        
                    else:
                        failed_users.append(f"Пользователь {user_id} (не в чате)")
                        logging.warning(f"Пользователь {user_id} не в чате")
                        
                except Exception as e:
                    failed_users.append(f"Пользователь {user_id} (ошибка: {e})")
                    logging.error(f"Ошибка при обработке пользователя {user_id}: {e}")
            
            # Отправляем итоговый отчет
            result_message = f"🐆 **Fat Leopard активирован!**\n"
            if failed_users:
                result_message += f"❌ **Ошибки:** {len(failed_users)}\n"
            result_message += f"\n⏱️ **Время:** 7 дней\n"
            result_message += f"💪 **Действие:** Отправьте `#training_done`\n\n"
            result_message += f"🦁 **Вы ведь не хотите стать как Fat Leopard?**\n"
            result_message += f"Тогда тренируйтесь и отправляйте отчеты!"
            
            await msg.reply(result_message, parse_mode="Markdown")
            logging.info(f"Fat Leopard запустил таймеры: {started_timers} успешно, {len(failed_users)} ошибок")
            
        except Exception as e:
            logging.error(f"Ошибка при запуске таймеров: {e}")
            await msg.reply("❌ Ошибка при запуске таймеров")
        return

    # Обработка команды /leopard_say
    if msg.text and msg.text.startswith("/leopard_say"):
        if msg.from_user.id != OWNER_ID:
            await msg.reply("❌ Только владелец может использовать эту команду!")
            return
        text = msg.text[len("/leopard_say"):].strip()
        if not text:
            await msg.reply("⚠️ Укажите текст для отправки.")
            return
        
        # Если команда отправлена в личку боту, запрашиваем ID чата
        if msg.chat.type == "private":
            await msg.reply("📝 Отправьте ID чата, куда нужно отправить сообщение.\n\nЧтобы получить ID чата:\n1. Добавьте бота в нужный чат\n2. Отправьте в чат команду /chat_id\n3. Скопируйте полученный ID")
            return
        
        # Отправляем сообщение в чат
        await bot.send_message(msg.chat.id, f"🦁 {text}")
        # Попытка удалить команду из чата (если есть права)
        try:
            await bot.delete_message(msg.chat.id, msg.message_id)
        except Exception as e:
            logging.warning(f"Не удалось удалить команду /leopard_say: {e}")
        return

    # Обработка команды /chat_id для получения ID чата
    if msg.text and msg.text.startswith("/chat_id"):
        if msg.from_user.id != OWNER_ID:
            return
        chat_id = msg.chat.id
        chat_title = msg.chat.title or "Личный чат"
        await msg.reply(f"📊 **Информация о чате:**\n\n🆔 **ID чата:** `{chat_id}`\n📝 **Название:** {chat_title}\n\n💡 Скопируйте ID чата для использования в личке с ботом")
        return

    # Обработка команды для отправки сообщения в конкретный чат (из лички)
    if msg.text and msg.text.startswith("/send_to_chat"):
        if msg.from_user.id != OWNER_ID:
            await msg.reply("❌ Только владелец может использовать эту команду!")
            return
        if msg.chat.type != "private":
            await msg.reply("⚠️ Эта команда работает только в личке с ботом.")
            return
        
        # Парсим команду: /send_to_chat CHAT_ID текст сообщения
        parts = msg.text.split(" ", 2)
        if len(parts) < 3:
            await msg.reply("⚠️ Использование: /send_to_chat CHAT_ID текст сообщения")
            return
        
        try:
            target_chat_id = int(parts[1])
            message_text = parts[2]
            
            # Отправляем сообщение в указанный чат
            await bot.send_message(target_chat_id, f"🦁 {message_text}")
            await msg.reply(f"✅ Сообщение отправлено в чат {target_chat_id}")
            
        except ValueError:
            await msg.reply("❌ Неверный формат ID чата. Используйте: /send_to_chat CHAT_ID текст")
        except Exception as e:
            await msg.reply(f"❌ Ошибка отправки: {e}")
        return

    # Обработка обычных сообщений (только если это не команда)
    if not msg.text or not msg.text.startswith("/"):
        chat_id = msg.chat.id
        user_id = msg.from_user.id
        current_time = datetime.utcnow()
        
        # Проверяем #training_done в тексте сообщения или подписи к медиа
        message_text = msg.text or msg.caption or ""
        has_training_done = "#training_done" in message_text.lower()
        has_sick_leave = "#sick_leave" in message_text.lower()
        has_healthy = "#healthy" in message_text.lower()
        
        logging.info(f"Получено сообщение от {user_id}: '{message_text}' (has_training_done: {has_training_done}, has_sick_leave: {has_sick_leave}, has_healthy: {has_healthy})")

        # Автоматически сохраняем информацию о сообщении
        try:
            async with aiosqlite.connect(DB_NAME) as db:
                # Получаем текущие данные пользователя
                async with db.execute('''
                    SELECT timer_start_time, sick_leave_start_time FROM message_log 
                    WHERE user_id = ? AND chat_id = ?
                ''', (user_id, chat_id)) as cursor:
                    row = await cursor.fetchone()
                    current_timer_start = row[0] if row else None
                    current_sick_leave_start = row[1] if row else None
                
                # Определяем время начала таймера
                timer_start_time = current_timer_start
                if has_training_done and not current_timer_start:
                    # Если это первый #training_done, устанавливаем время начала таймера
                    timer_start_time = current_time.isoformat()
                elif has_training_done and current_timer_start:
                    # Если это повторный #training_done, обновляем время начала таймера
                    timer_start_time = current_time.isoformat()
                
                # Определяем время начала больничного
                sick_leave_start_time = current_sick_leave_start
                if has_sick_leave and not current_sick_leave_start:
                    # Если это первый #sick_leave, устанавливаем время начала больничного
                    sick_leave_start_time = current_time.isoformat()
                elif has_healthy and current_sick_leave_start:
                    # Если это #healthy, очищаем время больничного
                    sick_leave_start_time = None
                
                # Сохраняем информацию о сообщении
                await db.execute('''
                    INSERT OR REPLACE INTO message_log (user_id, chat_id, last_message, has_training_done, has_sick_leave, has_healthy, timer_start_time, sick_leave_start_time)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                ''', (user_id, chat_id, current_time.isoformat(), has_training_done, has_sick_leave, has_healthy, timer_start_time, sick_leave_start_time))
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
                await msg.reply("✅ Отчёт принят 💪\n\n⏰ Таймер перезапускается на 7 дней\n\n🎯 Продолжай тренироваться и не забывай отправлять #training_done !")
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
        
        elif has_sick_leave:
            # Если это больничный, приостанавливаем таймер
            username = msg.from_user.username or msg.from_user.first_name
            logging.info(f"🏥 БОЛЬНИЧНЫЙ: Пользователь {user_id} (@{username}) запросил больничный")
            
            # Отменяем существующие таймеры
            if user_id in scheduled_removals:
                cancel_user_removal(user_id)
                logging.info(f"Таймеры для пользователя {user_id} приостановлены на время больничного")
            
            try:
                await msg.reply("🏥 **Больничный принят!** 🤒\n\n⏸️ Таймер приостановлен на время болезни\n\n💪 Выздоравливай и возвращайся к тренировкам!\n\n📝 Когда поправишься, отправь `#healthy` для возобновления таймера с места остановки")
                logging.info(f"Ответ 'Больничный принят' отправлен пользователю {user_id}")
            except Exception as e:
                logging.error(f"Ошибка при отправке ответа о больничном пользователю {user_id}: {e}")
        
        elif has_healthy:
            # Если это выздоровление, возобновляем таймер с того места где прервали
            username = msg.from_user.username or msg.from_user.first_name
            logging.info(f"💪 ВЫЗДОРОВЛЕНИЕ: Пользователь {user_id} (@{username}) выздоровел и возобновляет таймер")
            
            # Получаем данные о времени таймера и больничного
            async with aiosqlite.connect(DB_NAME) as db:
                async with db.execute('''
                    SELECT timer_start_time, sick_leave_start_time FROM message_log 
                    WHERE user_id = ? AND chat_id = ? 
                    ORDER BY last_message DESC LIMIT 1
                ''', (user_id, chat_id)) as cursor:
                    row = await cursor.fetchone()
                    timer_start_time = row[0] if row else None
                    sick_leave_start_time = row[1] if row else None
            
            if timer_start_time and sick_leave_start_time:
                # Рассчитываем оставшееся время
                timer_start = datetime.fromisoformat(timer_start_time)
                sick_leave_start = datetime.fromisoformat(sick_leave_start_time)
                current_time_dt = current_time
                
                # Время, которое прошло до больничного
                time_before_sick_leave = (sick_leave_start - timer_start).total_seconds()
                
                # Полное время таймера (7 дней)
                full_timer_duration = 7 * 24 * 60 * 60  # 7 дней в секундах
                full_warning_duration = 6 * 24 * 60 * 60  # 6 дней в секундах
                
                # Оставшееся время после больничного
                remaining_removal_time = max(0, full_timer_duration - time_before_sick_leave)
                remaining_warning_time = max(0, full_warning_duration - time_before_sick_leave)
                
                logging.info(f"РАСЧЕТ ВРЕМЕНИ: Таймер начался {timer_start}, больничный начался {sick_leave_start}, прошло {time_before_sick_leave} секунд, осталось {remaining_removal_time} секунд")
                
                if remaining_removal_time > 0:
                    # Возобновляем таймер с оставшимся временем
                    warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, remaining_warning_time, timer_start_time))
                    removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, remaining_removal_time, timer_start_time))
                    scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
                    
                    # Конвертируем в дни для отображения
                    remaining_days = remaining_removal_time / (24 * 60 * 60)
                    remaining_warning_days = remaining_warning_time / (24 * 60 * 60)
                    
                    try:
                        await msg.reply(f"💪 **Выздоровление принято!** 🎉\n\n⏰ Таймер возобновлён с места остановки!\n\n• {remaining_warning_days:.1f} дней до предупреждения\n• {remaining_days:.1f} дней до удаления\n\n💪 Добро пожаловать обратно к тренировкам!")
                        logging.info(f"Ответ 'Выздоровление принято' отправлен пользователю {user_id}")
                    except Exception as e:
                        logging.error(f"Ошибка при отправке ответа о выздоровлении пользователю {user_id}: {e}")
                    
                    logging.info(f"Таймер возобновлен для пользователя {user_id} после больничного с оставшимся временем {remaining_removal_time} секунд")
                else:
                    # Время истекло, запускаем новый таймер
                    warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60, current_time.isoformat()))
                    removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60, current_time.isoformat()))
                    scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
                    
                    try:
                        await msg.reply("💪 **Выздоровление принято!** 🎉\n\n⏰ Время на таймере истекло во время больничного, поэтому запущен новый таймер на 7 дней!\n\n• 6 дней до предупреждения\n• 7 дней до удаления\n\n💪 Добро пожаловать обратно к тренировкам!")
                        logging.info(f"Ответ 'Выздоровление принято' отправлен пользователю {user_id}")
                    except Exception as e:
                        logging.error(f"Ошибка при отправке ответа о выздоровлении пользователю {user_id}: {e}")
                    
                    logging.info(f"Новый таймер запущен для пользователя {user_id} после истечения времени во время больничного")
            else:
                # Если нет данных о времени, запускаем обычный таймер
                warning_task = asyncio.create_task(schedule_user_warning(user_id, chat_id, username, 6 * 24 * 60 * 60, current_time.isoformat()))
                removal_task = asyncio.create_task(schedule_user_removal(user_id, chat_id, 7 * 24 * 60 * 60, current_time.isoformat()))
                scheduled_removals[user_id] = {"warning": warning_task, "removal": removal_task}
                
                try:
                    await msg.reply("💪 **Таймер запущен!** 🎯\n\n⏰ Не удалось найти данные о предыдущем таймере, поэтому запущен новый таймер на 7 дней!\n\n• 6 дней до предупреждения\n• 7 дней до удаления\n\n💪 Тренируйся и отправляй `#training_done`!")
                    logging.info(f"Ответ 'Таймер запущен' отправлен пользователю {user_id}")
                except Exception as e:
                    logging.error(f"Ошибка при отправке ответа о запуске таймера пользователю {user_id}: {e}")
                
                logging.info(f"Новый таймер запущен для пользователя {user_id}")
        
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
