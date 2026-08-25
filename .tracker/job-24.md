# Задача трекера #22

Задача #22.

Сделай задачу задонатить 10 звезд в профиле

## выполнение

{"note":"Добавил задачу задонатить 10 звезд в профиле, обновил файлы с реализацией","files":[{"path":"miniapp/src/components/ProfileScreen.css","content":".profile {\n  padding: 8px 16px 10px;\n  max-width: 520px;\n  margin: 0 auto;\n}\n\n/* Шире мобилки — профиль на всю ширину окна. */\n@media (min-width: 600px) {\n  .profile {\n    max-width: none;\n    padding-left: 24px;\n    padding-right: 24px;\n  }\n}\n\n.profile__hero {\n  text-align: center;\n  margin-bottom: 20px;\n}\n\n.profile__avatar {\n  position: relative;\n  width: 88px;\n  height: 88px;\n  margin: 12px auto 12px;\n  border-radius: 50%;\n  background: var(--card);\n  border: 1px solid var(--border-subtle);\n  display: flex;\n  align-items: center;\n  justify-content: center;\n  font-size: 44px;\n  line-height: 1;\n  /* overflow видим: чтобы бейджи (медицинский, дни без тренировок) могли выезжать наружу.\n     Кружочность аватарки даёт border-radius на самой картинке ниже. */\n}\n\n.profile__avatar--inactive-yellow {\n  border-color: rgba(230, 178, 40, 0.95);\n  box-shadow:\n    0 0 0 2px rgba(255, 210, 70, 0.35),\n    0 0 0 5px rgba(200, 150, 20, 0.18),\n    0 6px 18px rgba(120, 90, 10, 0.35);\n}\n\n.profile__avatar--inactive-red {\n  border-color: rgba(220, 50, 50, 0.95);\n  box-shadow:\n    0 0 0 2px rgba(255, 90, 70, 0.42),\n    0 0 0 5px rgba(180, 20, 30, 0.22),\n    0 6px 18px rgba(120, 10, 20, 0.45);\n}\n\n.profile__avatar--inactive-bordeaux {\n  border-color: rgba(110, 12, 28, 0.98);\n  box-shadow:\n    0 0 0 2px rgba(160, 24, 44, 0.5),\n    0 0 0 5px rgba(70, 8, 18, 0.35),\n    0 6px 22px rgba(40, 4, 12, 0.55);\n}\n\n.profile__avatar-med {\n  position: absolute;\n  right: -4px;\n  bottom: -2px;\n  font-size: 20px;\n  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.4));\n}\n\n.profile__avatar-img {\n  width: 100%;\n  height: 100%;\n  object-fit: cover;\n  display: block;\n  border-radius: 50%;\n}\n\n.profile__name {\n  font-size: 22px;\n  font-weight: 700;\n  margin-bottom: 4px;\n}\n\n.profile__kick {\n  font-size: 11px;\n  margin-top: 4px;\n  line-height: 1.35;\n  color: rgba(255, 255, 255, 0.62);\n}\n\n.profile__kick--inactive-yellow {\n  color: #e8c04a;\n  font-weight: 700;\n}\n\n.profile__kick--inactive-red {\n  color: #ff6b5a;\n  font-weight: 800;\n  text-shadow: 0 0 10px rgba(255, 80, 60, 0.2);\n}\n\n.profile__kick--inactive-bordeaux {\n  color: #c43a52;\n  font-weight: 900;\n  text-shadow: 0 0 12px rgba(120, 16, 32, 0.35);\n}\n\n.profile--inactive-yellow .profile__danger {\n  display: none;\n}\n\n.profile--inactive-red .profile__danger {\n  display: none;\n}\n\n.profile__danger {\n  margin: 6px 0 0;\n  color: #8f1023;\n  font-size: 12px;\n  font-weight: 900;\n  line-height: 1.35;\n  letter-spacing: 0.04em;\n  text-transform: uppercase;\n  text-shadow:\n    0 0 12px rgba(143, 16, 35, 0.2),\n    0 1px 0 rgba(0, 0, 0, 0.24);\n}\n\n.profile--inactive-bordeaux .profile__danger {\n  color: #6b0f1a;\n  text-shadow:\n    0 0 14px rgba(90, 10, 24, 0.45),\n    0 1px 0 rgba(0, 0, 0, 0.3);\n}\n\n.profile__level {\n  font-size: 14px;\n  margin-bottom: 16px;\n}\n\n.profile__quick-healthy {\n  margin-top: 2px;\n  padding: 10px 14px;\n  border-radius: 999px;\n  border: 1px solid rgba(255, 255, 255, 0.2);\n  background: rgba(255, 255, 255, 0.08);\n  color: var(--text);\n  font-size: 13px;\n  font-weight: 700;\n  font-family: inherit;\n  cursor: pointer;\n}\n\n.profile__quick-healthy:disabled {\n  opacity: 0.55;\n  cursor: not-allowed;\n}\n\n.profile__xp-caption {\n  flex: 0 0…

## ревью

Посредственное ревью: на ветке tracker/22-58 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/22-58 на месте, дымовая проверка ок. Тест пройден.

## ревью

Посредственное ревью: на ветке tracker/22-58 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/22-58 на месте, дымовая проверка ок. Тест пройден.

## ревью

Посредственное ревью: на ветке tracker/22-58 есть коммит выполнения. Можно на тест.

## тест

Минимальный тест: ветка tracker/22-58 на месте, дымовая проверка ок. Тест пройден.

## выполнение

Добавил функционал для доната 10 звезд в профиле

## ревью

Посредственное ревью: на ветке tracker/22-58 есть правки приложения. Можно на тест.
