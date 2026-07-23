# Deployment Guide

Backend is self-hosted on the same DigitalOcean droplet as the `take` project
(4GB/2vCPU, $24/mo). All services run as Docker containers, built directly on
the droplet from a git clone — local dev is Apple Silicon, and cross-compiling
`linux/amd64` images with buildx from there was unreliable, so there's no
local build/push/GHCR step for this project.

The iOS app (`daily`) is distributed to a friend via TestFlight external
testing.

## Architecture

The droplet already runs `take`'s Caddy container bound to ports 80/443.
Widget doesn't get its own Caddy — it joins a shared Docker network so
`take`'s Caddy can reverse-proxy to it too, on its own subdomain.

```
Internet → take's Caddy (TLS + reverse proxy, ports 80/443)
              ├── take.yourdomain.com   → take frontend/api
              └── widget.tessera.love   → widget-api :8080

widget-api ──┐
              ├── widget's Postgres :5432
widget-worker ┘
              └── widget's Redis    :6379
```

`widget`'s Postgres/Redis/worker are private to widget's own compose
project. Only `api` is bridged onto the shared network so Caddy can reach
it — the two projects' databases never talk to each other.

## Prerequisites

### Local machine
- Xcode with a valid Apple Developer Program membership on team `723N3TY9M9`
  (this is the team already configured for the `daily` and `storyExtension`
  targets — see `frontend/daily/daily.xcodeproj`)
- Git access to push to the repo the droplet will clone from

### Droplet
- Docker + Docker Compose plugin, git (see one-time setup below)

### DNS (one-time)
Add an `A` record: `widget.tessera.love` → the droplet's IP.

### Droplet (one-time setup)

If you haven't already, create a Docker network shared between the two
projects, and add widget's site block to `take`'s Caddyfile.

```bash
# On the droplet
apt-get update && apt-get install -y docker.io docker-compose-plugin git   # if not already installed
docker network create web
```

#### Deploy key for the widget repo

This repo is private, so the droplet needs its own read-only deploy key —
don't reuse `take`'s key, since a leak of one would then expose both repos.

```bash
# On the droplet
ssh-keygen -t ed25519 -C "widget-deploy@droplet" -f ~/.ssh/widget_deploy -N ""
cat ~/.ssh/widget_deploy.pub
```

On GitHub: repo → **Settings** → **Deploy keys** → **Add deploy key** → paste
the public key. Leave **Allow write access** unchecked — the droplet only
ever pulls.

Since the droplet already has `take`'s deploy key at the default
`~/.ssh/id_ed25519` (or similar), add a host alias so git picks the right
key per repo — append to `~/.ssh/config`:

```
Host github.com-widget
    HostName github.com
    User git
    IdentityFile ~/.ssh/widget_deploy
    IdentitiesOnly yes
```

Then clone using that alias (note `github.com-widget`, not `github.com`):

```bash
git clone git@github.com-widget:<your-github-username>/widget.git /opt/widget
```

Because the remote URL itself encodes the alias, subsequent `git pull` in
`/opt/widget` will keep using this key automatically — no extra config
needed per pull.

Attach `take`'s Caddy container to the shared network — edit
`/opt/take/docker-compose.production.yml` and add the `web` network to the
`caddy` service and to the top-level `networks:` block:

```yaml
  caddy:
    # ...unchanged...
    networks:
      - default
      - web

networks:
  web:
    external: true
```

Add a site block to `/opt/take/Caddyfile`:

```
widget.tessera.love {
    reverse_proxy widget-api:8080
}
```

Then restart take's Caddy so it picks up both changes:

```bash
cd /opt/take
docker compose -f docker-compose.production.yml --env-file .env.production up -d caddy
```

## Environment variables

Create `/opt/widget/.env.production` on the droplet. Never commit this file.

```env
# Postgres
DATABASE_URL=postgres://widget:CHANGE_ME@db:5432/widgetdb
POSTGRES_PASSWORD=CHANGE_ME   # must match the password in DATABASE_URL above

# Redis
REDIS_URL=redis://redis:6379

# Auth
JWT_SECRET=CHANGE_ME

# Spotify (required by worker)
SPOTIFY_CLIENT_ID=
SPOTIFY_CLIENT_SECRET=

# TMDB (required by worker)
TMDB_ACCESS_TOKEN=

# Instagram (optional; fetcher currently disabled in worker)
INSTAGRAM_APP_ACCESS_TOKEN=

# APNs push (worker treats these as optional — omit all four to run without push)
APNS_KEY_PATH=/app/apns_key.p8
APNS_KEY_ID=
APNS_TEAM_ID=
APNS_TOPIC=skapur.sharedaily
APNS_PRODUCTION=true

# Host path to your .p8 key file, used by docker-compose.production.yml's
# volume mount for the worker container. Keep the key itself out of git.
APNS_KEY_HOST_PATH=/opt/widget/apns_key.p8
```

Copy your APNs `.p8` key (downloaded from Apple Developer → Certificates,
Identifiers & Profiles → Keys) to `/opt/widget/apns_key.p8` on the droplet.

## Deploy to droplet

Everything is built on the droplet from the git clone made during one-time
setup — no local build/push step.

```bash
# SSH in
ssh root@<droplet-ip>

cd /opt/widget
git pull

# Build the api/worker/migrate images from source
docker compose -f docker-compose.production.yml --env-file .env.production build

# Apply any pending schema migrations (safe to run on every deploy, including
# the first — see "Database migrations" below)
docker compose -f docker-compose.production.yml --env-file .env.production up -d db
docker compose -f docker-compose.production.yml --env-file .env.production \
  --profile tools run --rm migrate

# Start everything
docker compose -f docker-compose.production.yml --env-file .env.production up -d

# Check everything is running
docker compose -f docker-compose.production.yml ps
```

Confirm `widget-api` joined the shared network and Caddy can reach it:

```bash
curl -I https://widget.tessera.love/healthz
```

## Updating a single service

```bash
# On the droplet, after a git pull
docker compose -f docker-compose.production.yml --env-file .env.production build api
docker compose -f docker-compose.production.yml --env-file .env.production up -d api
```

If the pulled changes include a new file under `backend/sql/migrations/`, run
the migration step from "Deploy to droplet" above (rebuild + run `migrate`)
before restarting the service that depends on the new schema.

## Database migrations

Schema changes are tracked with [goose](https://github.com/pressly/goose) as
plain SQL files in `backend/sql/migrations/`, applied via a one-off
`migrate` container built from the same `backend/Dockerfile` (see the
`migrate` target). It's given the `tools` profile so it never starts with
plain `up -d`; it only runs when explicitly invoked with `--profile tools run`.

Rerunning `migrate` is always safe — goose tracks applied versions in a
`goose_db_version` table and skips anything already applied, so it can be run
on every deploy (including ones with no new migrations) without side effects.

To add a new migration:

```bash
# Locally, from backend/
go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 \
  -dir sql/migrations create <name> sql
```

This creates `sql/migrations/000N_<name>.sql` with `-- +goose Up` / `-- +goose
Down` sections to fill in. `sqlc generate` reads this directory directly (see
`sqlc.yaml`), so schema and queries stay in sync automatically — there's no
separate `schema.sql` to hand-update.

To apply migrations against the droplet:

```bash
# On the droplet, after a git pull
docker compose -f docker-compose.production.yml --env-file .env.production build migrate
docker compose -f docker-compose.production.yml --env-file .env.production \
  --profile tools run --rm migrate
```

To check the database's current migration version without applying anything:

```bash
docker compose -f docker-compose.production.yml --env-file .env.production \
  --profile tools run --rm migrate status
```

## Logs

```bash
docker compose -f docker-compose.production.yml logs -f api
docker compose -f docker-compose.production.yml logs -f worker
```

## Backups

```bash
# Manual backup (run on droplet)
docker exec widget-db pg_dump -U widget widgetdb | gzip > /backups/widgetdb-$(date +%Y%m%d).sql.gz
```

```bash
crontab -e
# Add:
0 4 * * * docker exec widget-db pg_dump -U widget widgetdb | gzip > /backups/widgetdb-$(date +\%Y\%m\%d).sql.gz
```

---

# TestFlight

## 1. Register the app in App Store Connect

1. Go to [appstoreconnect.apple.com](https://appstoreconnect.apple.com) → **My Apps** → **+** → **New App**.
2. Platform: iOS. Name: `daily` (or whatever you want testers to see).
3. Bundle ID: select `skapur.sharedaily` — if it's not in the dropdown yet,
   go to [developer.apple.com/account/resources/identifiers](https://developer.apple.com/account/resources/identifiers)
   first and register it under team `723N3TY9M9`, with **App Groups** and
   **Push Notifications** capabilities enabled. Automatic signing in Xcode
   can also create/update this for you the first time you build (see below).
4. The widget extension's bundle ID (`skapur.sharedaily.story`) does not
   need its own App Store Connect record — it ships inside the main app's
   archive.
5. SKU: anything unique (e.g. `daily-widget-2026`).

## 2. Confirm signing in Xcode

- Both already-edited files matter here:
  - `frontend/daily/daily.entitlements` — `aps-environment` is now `production`
    (required for push to work in a TestFlight/App Store build; the
    `development` value only works for local Xcode-run builds).
  - `daily.xcodeproj` Release config — `API_BASE_URL` is now
    `https://widget.tessera.love`.
- Open `frontend/daily/daily.xcodeproj` in Xcode, select the `daily` target
  → **Signing & Capabilities**. Confirm **Automatically manage signing** is
  checked and **Team** is set to the one matching `723N3TY9M9`. Do the same
  for the `storyExtension` target.
- If Xcode flags a signing issue, let it auto-fix — that regenerates
  provisioning profiles with the App Group + Push capabilities.

## 3. Archive and upload

1. In Xcode, set the run destination to **Any iOS Device (arm64)**.
2. **Product → Archive**. This builds the `daily` app and bundles the
   `storyExtension` widget into it.
3. When the Organizer window opens, select the archive → **Distribute App**
   → **App Store Connect** → **Upload**.
4. Use automatic signing when prompted. When asked about export compliance,
   answer that the app only uses standard HTTPS encryption (no custom
   cryptography) — this qualifies for the standard exemption.
5. Wait for processing in App Store Connect (**TestFlight** tab on your app)
   — usually 5–15 minutes, sometimes longer for a first build.

## 4. Invite your friend

Use **External Testing** rather than Internal Testing — internal testers
have to be added as users on your Apple Developer account, which is more
access than a friend needs.

1. In App Store Connect → your app → **TestFlight** tab → **External
   Testing** → create a group (e.g. "Friends").
2. Add the build once processing finishes.
3. **Test Information**: fill in what testers see (short description,
   feedback email). Required before the first external build can be
   reviewed.
4. Submit for **Beta App Review** — this is a lightweight review (usually
   under 24–48h) required only for a group's first build; later builds to
   the same group skip it unless you make significant changes.
5. Once approved, add your friend by email under the group's **Testers**
   tab. They'll get an email with a TestFlight invite link — they install
   the free **TestFlight** app from the App Store, then open your link.

## 5. Shipping updates

Bump `CURRENT_PROJECT_VERSION` (build number) in Xcode for each new
archive — `MARKETING_VERSION` doesn't need to change for TestFlight-only
updates. Archive and upload again; it'll appear as a new build in the same
external group without needing a new review (unless the changes are
significant enough for Apple to require one).
