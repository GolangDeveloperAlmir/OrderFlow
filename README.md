# OrderFlow

OrderFlow is an order management service written in Go. It exposes a REST API
over HTTPS with PostgreSQL persistence, Redis-backed authentication sessions
and generated Swagger documentation. A simple React/TypeScript frontend is
provided under `frontend/`.

## Running

Set the following environment variables:

```
export DATABASE_URL=postgres://user:pass@localhost:5432/orderflow?sslmode=disable
export REDIS_ADDR=localhost:6379
```

Run the server with TLS enabled:

```bash
go run .
```

The server listens on `https://localhost:8443`.

### API

* `POST /login` – authenticate and receive a session cookie stored in Redis.
* `POST /orders` – create an order.
* `GET /orders` – list all stored orders.

Swagger docs are generated into the `docs/` directory and served at
`/swagger/index.html`.

## Testing

Run unit tests and other helpers via the provided `Makefile`:

```bash
make test   # run Go unit tests
make swag   # regenerate swagger documentation
make docker # start services with docker-compose
```

