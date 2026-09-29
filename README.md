# Сокращатель ссылок (Go)

[![hexlet-check](https://github.com/P-Lisaveta/go-from-scratch-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/P-Lisaveta/go-from-scratch-project-278/actions)

[![ci](https://github.com/P-Lisaveta/go-from-scratch-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/P-Lisaveta/go-from-scratch-project-278/actions/workflows/ci.yml)

Спроектируйте приложение для удобных ссылок

Учебный проект Хекслета: https://ru.hexlet.io/programs/go-from-scratch


## Стек

- Go
- Gin
- PostgreSQL
- goose
- Docker
- Render
- Bugsink (Sentry SDK)

## Деплой

Приложение развернуто на Render: <https://go-from-scratch-project-278.onrender.com>.

Проверка работоспособности: <https://go-from-scratch-project-278.onrender.com/ping>.

## Переменные окружения

| Переменная | Назначение |
| --- | --- |
| `PORT` | Порт HTTP-сервера, по умолчанию `8080` |
| `DATABASE_URL` | Строка подключения к PostgreSQL |
| `SENTRY_DSN` | DSN проекта Bugsink/Sentry |
| `SENTRY_ENVIRONMENT` | Необязательное окружение событий, по умолчанию `production` |
| `SENTRY_RELEASE` | Необязательный идентификатор релиза |

## Установка

<!-- Опишите установку: клонирование, зависимости, переменные окружения -->

```bash
git clone https://github.com/P-Lisaveta/go-from-scratch-project-278.git
cd go-from-scratch-project-278
```

## Локальный запуск

```bash
go run .
```

Приложение будет доступно на `http://localhost:8080`. Миграции применяются
при запуске контейнера через `bin/run.sh`.

## Проверка мониторинга ошибок

После запуска приложения с заданным `SENTRY_DSN` откройте:

```text
/debug/sentry
```

Приложение отправит тестовое событие в Bugsink. Проверить статус сервиса можно
по маршруту `/ping`.

## Сборка и запуск в Docker

```bash
docker build -t go-from-scratch-project-278 .
docker run --rm -p 8080:8080 \
  -e PORT=8080 \
  -e DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=disable" \
  -e SENTRY_DSN="https://public-key@bugsink-host/project-id" \
  go-from-scratch-project-278
```

## Render

1. Создайте Web Service из репозитория.
2. Выберите Language — Docker.
3. Выберите Instance Type — Free.
4. Добавьте переменные окружения `PORT=8080`, `DATABASE_URL` и `SENTRY_DSN`.
5. После сборки проверьте маршрут `/ping` по HTTPS.

## Использование

<!-- Добавьте примеры запуска и запись asciinema — именно это смотрит работодатель -->

---

<details>
<summary>Автоматические тесты Хекслета</summary>

Тесты запускаются на каждый коммит. За запуск отвечает файл `.github/workflows/hexlet-check.yml` — не удаляйте и не переименовывайте ни его, ни репозиторий.

</details>

## О Хекслете

[Хекслет](https://ru.hexlet.io/) — школа программирования: авторские программы обучения с практикой, поддержкой наставников и реальными проектами, которые остаются в резюме. Этот репозиторий — один из таких проектов.
