# Booking Manager API

Backend Go + Gin untuk CRUD data booking (sewa) + ekspor PDF rekap. Implementasi MVP dari `PRD.md`.

## Cara jalan (lokal, SQLite)

```bash
cp .env.example .env
go run ./cmd/api
# atau: make run
```

Server listen `:8080` (atur via `PORT`). Tanpa `DATABASE_URL` dipakai SQLite file `bookings.db`.
Tanpa `API_KEY`, auth dinonaktifkan (dev lokal).

## Konfigurasi

| Env | Default | Keterangan |
|---|---|---|
| `PORT` | `8080` | Port HTTP |
| `DATABASE_URL` | `bookings.db` | Path SQLite, atau `postgres://...` untuk PostgreSQL |
| `API_KEY` | _(kosong)_ | Legacy: bila `ADMIN_API_KEY` kosong, dipakai sebagai kunci admin |
| `ADMIN_API_KEY` | _(kosong)_ | Kunci peran `admin` (akses penuh). Bila kosong, fallback ke `API_KEY` |
| `USER_API_KEY` | _(kosong)_ | Kunci peran `user` (boleh baca + tambah; dilarang PUT/PATCH/DELETE → `403`) |
| `APP_ENV` | `development` | — |

Bila `ADMIN_API_KEY` dan `USER_API_KEY` kosong, auth dinonaktifkan (dev lokal,
peran dianggap admin). Semua endpoint kecuali `/healthz` wajib header
`X-API-Key` atau `Authorization: Bearer <key>`.

## API

Base path `/api/v1`. Format error konsisten:

```json
{ "error": { "code": "VALIDATION_ERROR", "message": "Data tidak valid",
  "details": [{ "field": "durasi", "message": "harus antara 1 dan 1440 menit" }] } }
```

| Method | Endpoint | Deskripsi | Peran |
|---|---|---|---|
| GET | `/me` | Peran hasil autentikasi (`{ data: { role } }`) | admin, user |
| POST | `/bookings` | Buat booking → `201` | admin, user |
| GET | `/bookings?start_date=&end_date=&q=&page=&limit=` | Daftar + `meta{page,limit,total,total_nominal}` | admin, user |
| GET | `/bookings/:id` | Detail, `404` bila tidak ada | admin, user |
| PUT | `/bookings/:id` | Ubah penuh | admin saja |
| PATCH | `/bookings/:id` | Ubah sebagian | admin saja |
| DELETE | `/bookings/:id` | Soft delete → `204` | admin saja |
| GET | `/bookings/export/pdf?cutoff_date=Wajib&start_date=Opsional` | Unduh PDF rekap | admin, user |
| GET | `/healthz` | Health check (publik) | — |

Contoh:

```bash
curl -X POST localhost:8080/api/v1/bookings -H 'Content-Type: application/json' -d '{
  "tanggal": "2026-10-10", "jam": "19:00", "durasi": 120,
  "nama_penyewa": "Budi Santoso", "nominal_pembayaran": 300000
}'

curl 'localhost:8080/api/v1/bookings?q=budi&page=1&limit=20'

curl -OJ 'localhost:8080/api/v1/bookings/export/pdf?cutoff_date=2026-10-31&start_date=2026-10-01'
```

Aturan validasi: `tanggal` `YYYY-MM-DD`; `jam` `HH:MM` 00:00–23:59;
`durasi` 1–1440 menit; `nama_penyewa` 1–120 karakter (di-trim);
`nominal_pembayaran` ≥ 0. Jam selesai melewati tengah malam dibungkus
(tetap milik tanggal mulai).

## Docker

```bash
docker compose up --build   # app + PostgreSQL
```

## Test

```bash
go test ./...
```

Mencakup: validasi service, format Rupiah/tanggal Indonesia, generator PDF
(isi + kosong), integration test endpoint (CRUD, filter/search/pagination,
404, PDF cutoff inklusif, healthz), dan middleware API key.
