# otel-gitverse-demo

Мини-стек, который показывает цепочку:

`Go backend -> OpenTelemetry SDK -> Grafana Alloy -> Tempo -> Grafana`

В примере два Go-сервиса:
- `gitverse-api` — принимает `POST /repos`
- `hook-receiver` — получает внутренний webhook от `gitverse-api`

Это полезно, потому что один запрос создает **один trace**, внутри которого видно:
- входящий HTTP span на `gitverse-api`
- дочерние spans `auth.check`, `db.insert_repo`, `git.init_repo`
- клиентский span `hook.deliver`
- серверный span на `hook-receiver`
- дочерние spans `hook.validate` и `hook.store_event`

## Что показывает пример

- как инициализировать OTel SDK в Go
- как создавать spans вручную
- как передавать `traceparent` между сервисами
- как отправлять trace-данные в Alloy по OTLP/HTTP
- как открыть конкретный trace в Grafana Tempo

## Быстрый старт

```bash
docker compose up --build
```

После старта:
- API: `http://localhost:8080`
- Grafana: `http://localhost:3000`
- Tempo API: `http://localhost:3200`
- Alloy OTLP HTTP: `http://localhost:4318`

## Сгенерировать trace

```bash
curl -X POST http://localhost:8080/repos \
  -H 'Content-Type: application/json' \
  -d '{"owner":"alice","name":"demo-repo"}'
```

Пример ответа:

```json
{
  "status": "created",
  "repo_id": "alice-demo-repo-1742910000000",
  "owner": "alice",
  "name": "demo-repo",
  "trace_id": "4a66e6f4f83f0e53d8e1c9cfaf0e1234"
}
```

## Где смотреть trace

1. Открой Grafana: `http://localhost:3000`
2. Перейди в **Explore**
3. Выбери datasource **Tempo**
4. Вставь `trace_id` из ответа API
5. Открой trace и посмотри waterfall

Ты увидишь, что один запрос к `POST /repos` проходит через два сервиса и несколько span-ов.

## Что важно в коде

### 1. Инициализация OpenTelemetry

Файл: `internal/telemetry/telemetry.go`

Там:
- создается exporter
- поднимается `TracerProvider`
- ставится `BatchSpanProcessor`
- задается `service.name`
- включается propagator `TraceContext`

### 2. Root span на входящем HTTP-запросе

Файл: `internal/telemetry/telemetry.go`

Middleware:
- читает входящий `traceparent`, если он есть
- стартует server span
- пишет `X-Trace-ID` в ответ

### 3. Child spans в бизнес-логике

Файл: `cmd/api/main.go`

В `handleCreateRepo` вручную создаются span-ы:
- `auth.check`
- `db.insert_repo`
- `git.init_repo`
- `hook.deliver`

### 4. Передача `traceparent` во второй сервис

Файл: `cmd/api/main.go`

Перед HTTP-вызовом в `hook-receiver` выполняется:

```go
otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
```

### 5. Извлечение контекста во втором сервисе

Файл: `internal/telemetry/telemetry.go`

Middleware делает:

```go
otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
```

За счет этого span в `hook-receiver` попадает в тот же trace.

## Можно ли запустить без Alloy/Tempo/Grafana

Да. Если переменная `OTEL_EXPORTER_OTLP_ENDPOINT` не задана, приложение переключается на `stdout` exporter и печатает traces в консоль.

Например:

```bash
go mod tidy
go run ./cmd/api
```

## Следующий шаг

Этот стек специально минимальный. Он показывает **trace + spans + propagation**.

Если захочешь второй шаг, можно добавить:
- Prometheus
- metrics-generator в Tempo
- service graph в Grafana
- Loki для связки `trace -> logs`
