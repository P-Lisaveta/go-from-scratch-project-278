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
- Node.js
- Caddy
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
| `BASE_URL` | Публичный базовый URL приложения; из него формируется `short_url` |
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

Нужен Node.js 20 или новее. Установите frontend-зависимости:

```bash
npm install
```

Затем задайте `DATABASE_URL` и запустите API и UI одной командой:

```bash
npm start
```

Интерфейс будет доступен на `http://localhost:5173`, API — на
`http://localhost:8080`. В development API разрешает CORS-запросы с
`http://localhost:5173`. Миграции применяются при запуске контейнера через
`bin/run.sh`.

## API коротких ссылок

| Метод | Маршрут | Результат |
| --- | --- | --- |
| `GET` | `/api/links` | Список ссылок |
| `POST` | `/api/links` | Создание ссылки; `short_name` необязателен |
| `GET` | `/api/links/:id` | Одна ссылка |
| `PUT` | `/api/links/:id` | Обновление ссылки |
| `DELETE` | `/api/links/:id` | Удаление ссылки |

Для пагинации списка передайте включительный диапазон индексов в параметре
`range`, например `GET /api/links?range=[0,9]` вернёт первые десять ссылок.
Ответ содержит заголовок `Content-Range`, например `links 0-9/42`.

Для генерации кода SQL после изменения миграций или запросов выполните:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
```

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
docker run --rm -p 8080:80 \
  -e PORT=80 \
  -e BASE_URL="http://localhost:8080" \
  -e DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=disable" \
  -e SENTRY_DSN="https://public-key@bugsink-host/project-id" \
  go-from-scratch-project-278
```

## Render

1. Создайте Web Service из репозитория.
2. Выберите Language — Docker.
3. Выберите Instance Type — Free.
4. Добавьте переменные окружения `PORT=8080`, `DATABASE_URL`, `BASE_URL` и `SENTRY_DSN`.
   Значение `BASE_URL` — публичный URL Web Service, например
   `https://go-from-scratch-project-278.onrender.com`.
5. После сборки Caddy раздаёт интерфейс на корневом маршруте и проксирует
   `/api/*` и `/ping` к API. Проверьте маршрут `/ping` по HTTPS.

## Использование

<!-- Добавьте примеры запуска и запись asciinema — именно это смотрит работодатель -->

---

<details>
<summary>Автоматические тесты Хекслета</summary>

Тесты запускаются на каждый коммит. За запуск отвечает файл `.github/workflows/hexlet-check.yml` — не удаляйте и не переименовывайте ни его, ни репозиторий.

</details>

## О Хекслете

[Хекслет](https://ru.hexlet.io/) — школа программирования: авторские программы обучения с практикой, поддержкой наставников и реальными проектами, которые остаются в резюме. Этот репозиторий — один из таких проектов.
