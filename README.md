# cards-api

Emisión de tarjetas y autorización de compras contra Cardnet, nuestro processor. Cada débito
autorizado se registra en el ledger. Alcance PCI: ver [`docs/PCI.md`](docs/PCI.md) antes de tocar
nada de lo que toca `card_token`.

## Qué expone

Todo bajo `/v1`, JWT de Keycloak salvo donde se aclara:

- `POST /v1/cards` — `{account_id}`. Emite con Cardnet (`POST /v1/cards/issue`) y persiste.
- `GET /v1/cards/{id}` — nunca devuelve `card_token`. Ver `docs/PCI.md`.
- `GET /v1/accounts/{account_id}/cards` — tarjetas de una cuenta.
- `POST /v1/cards/{id}/block` — bloquea.
- `POST /v1/authorizations` — requiere rol `service`. `{card_id, amount, currency, merchant_name}`.
  Cardnet autoriza; si aprueba, débito en ledger-core con `reference: card-auth:<auth_id>`. Se
  persiste la autorización apruebe o no. Diagrama del flujo completo en `docs/PCI.md`.

`GET /health` y `GET /metrics` sin auth.

## Stack

Go 1.22, `chi`, `pgx/v5`, `golang-migrate` (migraciones en `migrations/`, aplicadas al arrancar —
ver `internal/store/migrate.go`), `cauri-go-kit` (`log`, `auth`, `httpx`, `metrics`),
`prometheus/client_golang` vía el kit.

## Correr local

```
docker compose up
```

Levanta un Postgres vacío y cards-api apuntando a él. cards-api corre sus propias migraciones al
arrancar (`migrations/000001_init.up.sql`), así que `/health` en verde ya implica que `cards`
tiene las tablas. Las variables de `CARDNET_URL`, `CARDNET_API_KEY`, `KEYCLOAK_CLIENT_SECRET` en
`docker-compose.yml` son dummies — no hay Cardnet real corriendo en local, solo el mock de
`infra-terraform` en el cluster del lab.

## Tests

```
go test ./...
```

Los handlers se testean contra `store`, `processor` (Cardnet) y `ledger` (ledger-core) fakes, sin
red ni Postgres de por medio (`internal/http/*_test.go`). Cubre: autorización aprobada registra
un débito con la referencia correcta; declinada no toca el ledger pero sí queda persistida;
`GET /v1/cards/{id}` no expone `card_token` bajo ninguna forma.

## Deploy

Helm, vía Actions (`deploy.yml`): push a `main` sube a staging con el `sha` corto de la imagen; un
tag `v*` sube lo mismo a prod. Chart en `deploy/chart`, valores por entorno en
`deploy/values-staging.yaml` (Cardnet sandbox) y `deploy/values-prod.yaml` (Cardnet live).

```
helm upgrade --install cards-api ./deploy/chart -n staging -f deploy/values-staging.yaml
```

`ci.yml` corre `go vet`, `go test`, `govulncheck` y `helm lint` en cada PR.

## Variables de entorno

`SERVICE_NAME`, `ENV`, `PORT`, `LOG_LEVEL`, `DATABASE_URL` (base `cards`), `KEYCLOAK_ISSUER`,
`KEYCLOAK_CLIENT_ID` (`cards-api`), `KEYCLOAK_CLIENT_SECRET`, `LEDGER_CORE_URL`, `CARDNET_URL`,
`CARDNET_API_KEY`. Todas requeridas salvo las que tienen default en `cmd/cards-api/run.go`.

— Diego
