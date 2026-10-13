# Devaraj Time

**Smarter Attendance. Fairer Workplaces.**

Open-source attendance and workforce management software for organizations, HR teams, department managers, and employees. Record attendance through workplace QR points, manage schedules, and resolve discrepancies through traceable correction requests.

**License:** [MIT](LICENSE)  
**Product showcase:** https://devarajsolutions.site/products/devaraj-time

## Choose how to run it

- **Self-host:** use and modify the MIT-licensed source without buying a service. You operate the infrastructure, credentials, updates, backups, and employee data.
- **Paid setup:** ask Devaraj Solutions to deploy and configure it on your infrastructure, with agreed handover and support scope.
- **Managed hosting:** ask Devaraj Solutions to host and operate it for your team under a separately agreed service arrangement.

The software is provided as-is. Setup, hosting, and support are paid services quoted separately; there are no published prices, response SLAs, or instant hosted signup. See [open-source and service choices](docs/open-source-and-services.md).

**Request a quote:** [email](mailto:pavkhemerak.official@gmail.com), [Telegram @pavkhemerak](https://t.me/pavkhemerak), or phone +85514945699. Share your team size, deployment environment, intended domain, and support needs. Do not send credentials or employee records in the initial inquiry.

## Core capabilities

- QR check-in/out and break events with server timestamps, fresh location evidence, configurable workplace geofences, and duplicate-request protection.
- Employee, workplace, branch, department, and shift administration with authorized access scopes.
- Effective-dated individual/bulk schedules, rotating daily cycles, rest days, and temporary overrides.
- Employee attendance history, correction requests, non-self approval decisions, and retained original events.
- Full-day leave requests, notifications, audit records, and CSV/XLSX attendance-summary exports.
- Administrator provisioning, invitation-only employee passkeys, and SMTP-configured password recovery.
- A responsive React interface with light/dark themes and an informational offline notice.

These describe the implemented core, not a production certification. GPS and photographed QR codes are not proof of physical presence. Read [current limitations](docs/limitations.md) before adoption.

## Local development

Install Docker Engine/Desktop and Docker Compose. Ports 5173, 8080, and 5432 must be available.

Create an **untracked local `.env`** manually. Environment files and templates are intentionally excluded from the repository. Set:

| Setting | Local purpose |
| --- | --- |
| POSTGRES_PASSWORD | A strong private database password. |
| POSTGRES_USER / POSTGRES_DB | Optional database/user names; Compose defaults are documented in the service definition. |
| PUBLIC_URL | The exact local browser origin, including scheme and port. |
| APP_ENV / COOKIE_SECURE | Use development mode and non-secure cookies only for local HTTP. |
| DEMO_PASSWORD | A private password of at least 12 characters if you explicitly use fictional development seed data. |
| DATABASE_URL | Required for native Go scripts; keep it consistent with the local database credentials and URI-encode special password characters. Compose constructs its own internal connection string. |

Start the database, migrate, and run the application:

```sh
docker compose up --build -d db
docker compose run --rm --build migrate
docker compose up --build -d api web
```

For a disposable local demo, explicitly run `docker compose run --rm --build seed` after migrations. The seed is development-only and creates fictional data. Never seed production.

Open http://localhost:5173. Read [the user guide](docs/user-guide.md) and [provisioning/enrollment guide](docs/system-update.md) for account setup. Public registration is disabled. Without demo data, use the documented bootstrap-admin command with private administrator credentials, then provision companies and enroll employees.

Configure real testing workplace coordinates; location verification will reject scans outside the configured area. Do not use production employee records for development.

## Production self-hosting

Follow [deployment and operations](docs/deployment.md). Production needs a stable HTTPS origin, strong credentials, database migrations, appropriate runtime privileges, and a verified backup/restore process. The production override is used together with the base Compose file:

```sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d db
docker compose -f docker-compose.yml -f docker-compose.prod.yml run --rm migrate
docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d api web caddy
```

Configure DOMAIN, ACME_EMAIL, production PUBLIC_URL, APP_ENV, COOKIE_SECURE, and database credentials through your host's secret configuration. SMTP settings are required for recovery email. Passkeys and printed QR links need a stable origin. Review employee-data policies, retention, backups, and the [known security/operating limits](docs/limitations.md) before adoption.

## Stack and layout

Go/Gin and PostgreSQL provide the API and attendance records. React, TypeScript, and Vite provide the application. Docker Compose and Caddy support deployment.

- `backend/cmd/api`: API and operator commands.
- `backend/internal`: attendance policy, authentication, database operations, seed data, and HTTP modules.
- `backend/migrations`: versioned database changes.
- `frontend/src`: workspaces, attendance scanner, records, requests, and management views.
- `frontend/public`: application icons, manifest, and offline notice.
- `docs`: operating guidance, API contract, security, limitations, and recorded verification.
- `scripts`: local development and validation helpers.

The parent company website has its own repository and deployment. No shared database or cross-product authentication is required.

## Checks and contributions

With a disposable local database configured:

```powershell
./scripts/backend.ps1 test
./scripts/backend.ps1 vet
cd frontend
npm ci
npm test
npm run build
npx playwright test
```

Integration tests create isolated schemas; never point tests at a production database. Browser tests need the development application and installed Chrome. Read [verification notes](docs/verification.md) for recorded results and untested scenarios. Follow [CONTRIBUTING.md](CONTRIBUTING.md) for focused bug reports and changes; never publish credentials, precise employee locations, or private records in issues.

## Documentation

[Architecture](docs/architecture.md) · [API](docs/api.md) · [OpenAPI](docs/openapi.json) · [Database](docs/database.md) · [Attendance rules](docs/attendance-rules.md) · [Security](docs/security.md) · [User guide](docs/user-guide.md) · [Limitations](docs/limitations.md)

## License

Original software is licensed under [MIT](LICENSE). Preserve its copyright and permission notice when redistributing. Third-party dependencies retain their own licenses. Paid setup and hosting do not change the open-source license or require users to purchase a service.
