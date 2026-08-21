#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import mm, cm
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_RIGHT
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak
from reportlab.lib import colors
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.lib.colors import HexColor

# Register fonts (using default fonts that support Cyrillic)
from reportlab.pdfbase.pdfmetrics import registerFontFamily

def create_resume():
    filename = "Ivan_Sishko_Resume_Go_Backend.pdf"
    doc = SimpleDocTemplate(
        filename,
        pagesize=A4,
        rightMargin=20*mm,
        leftMargin=20*mm,
        topMargin=15*mm,
        bottomMargin=15*mm
    )
    
    # Styles
    styles = getSampleStyleSheet()
    
    # Custom styles
    title_style = ParagraphStyle(
        'CustomTitle',
        parent=styles['Heading1'],
        fontSize=20,
        textColor=HexColor('#1a1a1a'),
        spaceAfter=2*mm,
        alignment=TA_LEFT,
        fontName='Helvetica-Bold'
    )
    
    subtitle_style = ParagraphStyle(
        'CustomSubtitle',
        parent=styles['Normal'],
        fontSize=14,
        textColor=HexColor('#555555'),
        spaceAfter=8*mm,
        fontName='Helvetica'
    )
    
    section_header = ParagraphStyle(
        'SectionHeader',
        parent=styles['Heading2'],
        fontSize=13,
        textColor=HexColor('#000000'),
        spaceAfter=3*mm,
        spaceBefore=5*mm,
        fontName='Helvetica-Bold',
        borderWidth=0,
        borderColor=HexColor('#333333'),
        borderPadding=2*mm,
    )
    
    body_style = ParagraphStyle(
        'CustomBody',
        parent=styles['Normal'],
        fontSize=9.5,
        textColor=HexColor('#1a1a1a'),
        spaceAfter=2*mm,
        leading=12,
        fontName='Helvetica'
    )
    
    contact_style = ParagraphStyle(
        'Contact',
        parent=styles['Normal'],
        fontSize=9,
        textColor=HexColor('#333333'),
        spaceAfter=1*mm,
        fontName='Helvetica'
    )
    
    job_title_style = ParagraphStyle(
        'JobTitle',
        parent=styles['Normal'],
        fontSize=11,
        textColor=HexColor('#000000'),
        spaceAfter=1*mm,
        fontName='Helvetica-Bold'
    )
    
    job_period_style = ParagraphStyle(
        'JobPeriod',
        parent=styles['Normal'],
        fontSize=9,
        textColor=HexColor('#666666'),
        spaceAfter=3*mm,
        fontName='Helvetica-Oblique'
    )
    
    # Story
    story = []
    
    # Header
    story.append(Paragraph("СИШКО ИВАН БОРИСОВИЧ", title_style))
    story.append(Paragraph("Senior Go / Backend Developer", subtitle_style))
    
    # Contact Info
    contact_data = [
        ["Москва, Россия", "+7 916 199-54-48"],
        ["ivansishko@gmail.com", "Telegram: @lofitravel"],
        ["github.com/fuserwyn", ""]
    ]
    contact_table = Table(contact_data, colWidths=[85*mm, 85*mm])
    contact_table.setStyle(TableStyle([
        ('FONTNAME', (0, 0), (-1, -1), 'Helvetica'),
        ('FONTSIZE', (0, 0), (-1, -1), 9),
        ('TEXTCOLOR', (0, 0), (-1, -1), HexColor('#333333')),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
    ]))
    story.append(contact_table)
    story.append(Spacer(1, 5*mm))
    
    # О себе
    story.append(Paragraph("О СЕБЕ", section_header))
    story.append(Paragraph(
        "Senior backend-разработчик с <b>5+ лет коммерческого опыта</b> разработки и проектирования "
        "микросервисных систем с нуля. Специализируюсь на <b>Go</b> и распределённых системах высоких "
        "нагрузок (200K+ пользователей).",
        body_style
    ))
    story.append(Paragraph(
        "Имею подтверждённый опыт <b>проектирования и написания сервисов на Go с нуля</b>, включая "
        "разработку Telegram Mini App с AI-агентом, task tracker'ом и системой геймификации. Работал "
        "с полным циклом разработки: от архитектуры до деплоя в production с мониторингом и оркестрацией.",
        body_style
    ))
    story.append(Spacer(1, 3*mm))
    
    # Технические навыки
    story.append(Paragraph("ТЕХНИЧЕСКИЕ НАВЫКИ", section_header))
    
    skills_data = [
        ["<b>Go Stack:</b>", "Go (Echo, Zap), проектирование микросервисов с нуля, REST API, gRPC"],
        ["<b>Infrastructure:</b>", "Prometheus, Kubernetes, HashiCorp Vault, Docker, Railway"],
        ["<b>Базы данных:</b>", "PostgreSQL, Redis, Apache Kafka, Qdrant (векторная БД)"],
        ["<b>Дополнительно:</b>", "Python (FastAPI, async), Git, Linux, Swagger/OpenAPI, Sentry, Kibana"],
    ]
    skills_table = Table(skills_data, colWidths=[35*mm, 135*mm])
    skills_table.setStyle(TableStyle([
        ('FONTNAME', (0, 0), (-1, -1), 'Helvetica'),
        ('FONTSIZE', (0, 0), (-1, -1), 9),
        ('TEXTCOLOR', (0, 0), (-1, -1), HexColor('#1a1a1a')),
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('TOPPADDING', (0, 0), (-1, -1), 1.5*mm),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 1.5*mm),
    ]))
    story.append(skills_table)
    story.append(Spacer(1, 3*mm))
    
    # Опыт работы
    story.append(Paragraph("ОПЫТ РАБОТЫ", section_header))
    
    # Сбер
    story.append(Paragraph("Сбер | Senior Python/Go Developer", job_title_style))
    story.append(Paragraph("Август 2023 — настоящее время", job_period_style))
    
    story.append(Paragraph(
        "Технический лидер распределённой микросервисной платформы с аудиторией <b>200K+ пользователей</b>.",
        body_style
    ))
    
    story.append(Paragraph("<b>Зоны ответственности:</b>", body_style))
    story.append(Paragraph(
        "• Проектирование и разработка <b>микросервисов на Go с нуля</b>: от архитектуры до production<br/>"
        "• Технический лидер для ~15 микросервисов в production<br/>"
        "• Довёл <b>10 микросервисов от концепции до production</b>: полный цикл разработки<br/>"
        "• Проектирование event-driven архитектуры на <b>Apache Kafka</b><br/>"
        "• Настройка мониторинга с <b>Prometheus</b> и алертинга для всех критических сервисов<br/>"
        "• Деплой и управление микросервисами в <b>Kubernetes</b><br/>"
        "• Интеграция <b>HashiCorp Vault</b> для управления секретами и credentials<br/>"
        "• Highload-инженерия: потоковая обработка событий, массовые рассылки",
        body_style
    ))
    
    story.append(Paragraph("<b>Достижения:</b>", body_style))
    story.append(Paragraph(
        "• <b>Снизил latency с 800ms до 120ms</b>, перепроектировав межсервисный обмен на Kafka<br/>"
        "• Спроектировал GDPR-совместимый consent service с 100% соответствием требованиям<br/>"
        "• <b>Ускорил доставку уведомлений на 60%</b> через асинхронную обработку<br/>"
        "• Интегрировал <b>AI-агентов (Qwen)</b> для автоматизации бизнес-процессов<br/>"
        "• <b>Сократил production-инциденты на 35%</b> через интеграционное тестирование<br/>"
        "• Настроил full observability stack: Prometheus + Grafana + Zap logging",
        body_style
    ))
    
    story.append(Paragraph(
        "<b>Стек:</b> Go (Echo, Zap), Python (FastAPI), Apache Kafka, PostgreSQL, Redis, "
        "Kubernetes, Prometheus, HashiCorp Vault, Docker, Swagger/OpenAPI",
        body_style
    ))
    story.append(Spacer(1, 3*mm))
    
    # KITEsoft
    story.append(Paragraph("KITEsoft | Разработчик Python/Go", job_title_style))
    story.append(Paragraph("Октябрь 2022 — Июль 2023", job_period_style))
    
    story.append(Paragraph(
        "• Проектировал и реализовывал <b>микросервисы на Go с нуля</b>, отвечая за архитектуру<br/>"
        "• Интеграция Kafka и Redis Queue для асинхронной обработки<br/>"
        "• Настройка загрузки данных в Amazon S3<br/>"
        "• Код-ревью и менторинг команды",
        body_style
    ))
    
    story.append(Paragraph(
        "<b>Стек:</b> Go, Python (FastAPI), Kafka, Redis Queue, Amazon S3, Docker, Ansible, PostgreSQL",
        body_style
    ))
    story.append(Spacer(1, 4*mm))
    
    # Ключевые проекты
    story.append(Paragraph("КЛЮЧЕВЫЕ ПРОЕКТЫ НА GO", section_header))
    
    # Fat Leopard
    story.append(Paragraph("Fat Leopard — Telegram Mini App с AI-тренером", job_title_style))
    story.append(Paragraph("<i>Роль: Tech Lead, Go Backend Developer</i>", job_period_style))
    
    story.append(Paragraph(
        "Спроектировал и разработал <b>с нуля полноценное приложение</b> для трекинга тренировок "
        "с AI-агентом и системой задач.",
        body_style
    ))
    
    story.append(Paragraph("<b>Архитектура и реализация:</b>", body_style))
    story.append(Paragraph(
        "• <b>Go-бот</b> как основной backend: обработка команд, бизнес-логика, интеграция с Mini App<br/>"
        "• <b>AI-агент на базе LLM (Deepseek)</b> с векторным хранилищем <b>Qdrant</b><br/>"
        "  - Генерация персонализированных тренировочных планов<br/>"
        "  - Анализ прогресса пользователя<br/>"
        "  - Ответы на вопросы о тренировках в реальном времени<br/>"
        "• <b>Task Tracker / Система задач:</b><br/>"
        "  - Автоматическая генерация тренировочных задач AI-агентом<br/>"
        "  - Трекинг выполнения упражнений<br/>"
        "  - Система напоминаний и уведомлений<br/>"
        "  - Прогресс-трекинг с визуализацией",
        body_style
    ))
    
    story.append(Paragraph("<b>Технические особенности:</b>", body_style))
    story.append(Paragraph(
        "• <b>Go (Echo framework, Zap logging)</b> для REST API и Telegram Bot API<br/>"
        "• Event-driven архитектура для асинхронной обработки<br/>"
        "• <b>PostgreSQL</b> для хранения данных пользователей, тренировок и задач<br/>"
        "• Система геймификации: XP/MET по стандартам WHO/ACSM<br/>"
        "• Интеграция платежей: Telegram Stars, ЮKassa<br/>"
        "• <b>Мониторинг:</b> Prometheus метрики, Zap structured logging<br/>"
        "• <b>Деплой:</b> Docker, Railway с автоматическим CI/CD",
        body_style
    ))
    
    story.append(Paragraph(
        "<b>Стек:</b> Go (Echo, Zap), Python, TypeScript/Vite, PostgreSQL, Qdrant, "
        "Prometheus, Docker, Railway",
        body_style
    ))
    story.append(Spacer(1, 3*mm))
    
    # Другие проекты
    story.append(Paragraph("Автономный торговый AI-агент для Bybit", job_title_style))
    story.append(Paragraph(
        "• <b>Go</b> для исполнения ордеров с low latency и риск-менеджментом<br/>"
        "• Python (FastAPI) для ML-модели и API<br/>"
        "• <b>Kafka</b> для оркестрации компонентов<br/>"
        "• Полная автономность: zero human intervention",
        body_style
    ))
    story.append(Paragraph(
        "<b>Стек:</b> Go, Python (FastAPI), Kafka, LightGBM, Docker, Railway",
        body_style
    ))
    story.append(Spacer(1, 3*mm))
    
    story.append(Paragraph("Telegram-бот на Go (Школа 42)", job_title_style))
    story.append(Paragraph(
        "• OCR для распознавания текста с изображений<br/>"
        "• Интеграция с AI API для генерации контента<br/>"
        "• Асинхронная обработка запросов",
        body_style
    ))
    story.append(Spacer(1, 4*mm))
    
    # Образование
    story.append(Paragraph("ОБРАЗОВАНИЕ", section_header))
    
    story.append(Paragraph(
        "<b>Ecole 42, Париж</b> (платформа INTRA) — 2021–2026<br/>"
        "Диплом RNCP6 (аналог бакалавриата) — разработка ПО: системное программирование, "
        "алгоритмы, низкоуровневая разработка",
        body_style
    ))
    story.append(Spacer(1, 2*mm))
    
    story.append(Paragraph(
        "<b>МГТУ им. Н.Э. Баумана</b><br/>"
        "Высшее образование — Энергомашиностроение, Плазменные технологии (2015)<br/>"
        "Аспирантура — Электроракетные двигатели (2019)",
        body_style
    ))
    story.append(Spacer(1, 3*mm))
    
    # Языки
    story.append(Paragraph("ЯЗЫКИ", section_header))
    story.append(Paragraph("Русский — родной | Английский — C1 (продвинутый)", body_style))
    
    # Build PDF
    doc.build(story)
    print(f"PDF created: {filename}")

if __name__ == "__main__":
    create_resume()
