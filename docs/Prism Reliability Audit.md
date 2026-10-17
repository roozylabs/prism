roozylabs/prism · main @ 57c4067 · diperbarui 16 Oct 2026

# Audit robustness dan reliability Prism

Audit menyeluruh atas jalur request gateway, operasional dan deploy, serta semua fitur dari backend sampai UI. Setiap temuan menyebut lokasi di kode, skenario gagalnya, dan perbaikannya. Perbaikan dikerjakan bertahap dalam batch PR kecil; kolom Status diperbarui setiap batch masuk.

52 temuanGo API · Next.js · Postgres · Redisdibaca dari kode, temuan utama diverifikasi manual

[Rencana perbaikan](#plan) [Keamanan dan akses](#sec) [Jalur request gateway](#gw) [Operasional dan deploy](#ops) [Fitur rusak atau tiruan](#feat) [Validasi dan penanganan error](#val) [Cakupan tes](#test) [Matriks fitur](#matrix) [Yang sudah solid](#solid)

**1**kritis

**17**tinggi

**28**sedang

**6**rendah

**52 / 52**sudah diperbaiki

**Dua hal paling mendesak: captcha login yang bisa dilewati, dan gateway yang bisa macet saat provider bermasalah.**

Login dan signup menerima token uji Cloudflare di produksi tanpa rate limit (S1). Di gateway, failover stream membocorkan slot concurrency (G1), retry tidak dibatasi (G3), dan body request tidak dibatasi (G4), jadi satu provider yang lambat atau satu request besar bisa menahan seluruh layanan. Job async juga hilang di setiap deploy (O1).

## Rencana perbaikan

Satu PR per batch, diurutkan dari risiko terbesar. Batch tidak bergantung satu sama lain kecuali yang menyentuh file yang sama, sehingga urutannya bisa diubah.

| Batch | Isi | Temuan | Status |
| --- | --- | --- | --- |
| 1 | **fix(auth): captcha dan brute force**Token uji Turnstile hanya di development, rate limit login dan signup, validasi signup, timeout ke Cloudflare. | S1 | Selesai |
| 2 | **fix(gateway): failover dan retry**Lepas slot saat failover, pakai MAX_RETRIES dan deadline, klasifikasi 401/403, batas body, ReadTimeout. | G1 G2 G3 G4 G5 | Selesai |
| 3 | **fix(gateway): admission**Agent dari middleware, agent tak dikenal ditolak, budget harian untuk semua budget, deadline kuota. | S3 G7 G8 | Selesai |
| 4 | **fix(queue): job async tahan restart**BLMOVE dan re-queue, shutdown berurutan, async tanpa stream, post-processor terbatas dan aman. | G6 O1 O7 | Selesai |
| 5 | **fix(ops): health, log, config, Redis**/ready untuk healthcheck, log tanpa token, timeout Redis, config gagal cepat, migration dirty, trusted proxies. | S2 S8 O2 O8 O9 O10 O11 | Selesai |
| 6 | **fix(deploy): rollback dan backup**Tag sha-, tag sebelumnya disimpan, gagal saat pull atau nginx gagal, skrip backup tunggal. | O3 O5 | Selesai |
| 7 | **feat(retention): pembersihan tabel**Retensi per tabel dengan hapus bertahap, sesi kedaluwarsa, worker jalan saat start. | O4 O6 | Selesai |
| 8 | **fix(app): fitur rusak di UI**Tampilkan gateway key, paginasi, Settings nyata, billing, bentuk respons di client. | S9 U1 U2 U3 U6 U10 | Selesai |
| 9 | **fix(api): akses dan validasi**Registry dan template per pemilik, guard izin, gateway key di route admin, validasi, 409, policy default. | S4 S5 S6 S7 U9 V1 V2 | Selesai |
| 10 | **fix(gateway): rate limit, idempotency, cache key**Rate limiter satu skrip Lua dengan `rate_limit` per key dan header `X-RateLimit-*`, lock idempotency bertoken dengan perpanjangan dan sidik jari request (422 bila key dipakai ulang), invalidate cache saat key dihapus plus cache negatif. | G12 G13 G16 | Selesai |
| 11 | **fix(gateway): usage Anthropic, model gratis, stream panjang**Usage stream Anthropic dari `message.usage` dan `usage`, event error diteruskan atau failover, stream kosong dianggap gagal; keepalive `: ping`, deadline dan idle timeout per percobaan, header SSE baru ditulis saat byte pertama; model gratis dihargai 0 dengan cache harga. | G9 G14 G15 | Selesai |
| 12 | **perf(gateway): efek samping terbatas, statement_timeout, active streams per user**Tulis DB/Redis pasca-request lewat pool 16 worker dengan deadline 10 detik dan antrean terbatas, `statement_timeout` pada pool koneksi, hash active streams per user dengan HSET+EXPIRE satu pipeline dan publish di luar jalur request. | G10 G11 | Selesai |
| 13 | **ops(observability): metrik, correlation id, log JSON, container dan CI**Exporter Prometheus selalu aktif dengan `/metrics` di balik `METRICS_TOKEN`, correlation middleware dipasang dan request id dipakai handler serta payload error, semua log JSON via `slog`, rule alert di `deploy/otel`; image API non-root di Alpine 3.22, batas CPU/memori dan `GOMEMLIMIT` di compose, action CI di-pin ke SHA, ambang coverage, typecheck app. | O12 O13 | Selesai |
| 14 | **feat(members): undangan bisa diterima, audit log admin ditulis**Token undangan di-hash dan tidak pernah dikembalikan; endpoint preview, terima, daftar, cabut; email via SMTP bila diset, selain itu tautan diserahkan sekali ke pengundang; halaman `/accept-invite` di dashboard. Audit admin ditulis untuk anggota, key, credential, policy, settings, budget; ekspor CSV lewat `encoding/csv` dengan guard formula, filter tanggal, dan paging tanpa batas 1000. | U4 U5 | Selesai |
| 15 | **fix(routing): routing rule dipakai, pemilih policy sandbox berfungsi**Router membaca `routing_rules` yang aktif (pola case-insensitive dengan `*` dan `?`, prioritas terkecil menang) untuk model eksplisit maupun kandidat `prism-auto`; key yang terikat provider tetap menang; validasi pola, prioritas, dan kepemilikan provider. Sandbox membaca `X-Routing-Policy` (id atau nama policy milik pemilik key, 404 bila tidak ada/nonaktif) di jalur sync dan async; `X-Prism-Routing-Policy` berisi nama policy yang dipakai, bukan nama model; selector dashboard memakai id policy nyata dan membaca `X-Prism-Model`. | U7 U8 | Selesai |
| 16 | **test: suite tes dashboard, tes handler dan repository**Vitest + jsdom + Testing Library di `apps/app` (`pnpm test`, dijalankan CI sebelum build): alur gateway key (dialog buat key, secret sekali tampil, client key berhalaman), paginasi (`DataTable` mode client dan server, `fetchAllPages`), auth (schema login/signup, `useTurnstile`, redirect middleware, header auth/tenant dan 401 di client axios), parse error API. Tes handler berbasis DB untuk budget, gateway key, routing policy, logs, dashboard, agent, model, credential, onboarding; tes repository untuk billing, budget, routing policy, request log, audit trail, gateway key, MCP registry. | T1 T2 | Selesai |
| — | **Backlog**Tidak ada temuan tersisa setelah batch 16. | — | — |

## Keamanan dan akses

Celah yang membuat orang lain bisa masuk, membaca, atau mengubah sesuatu yang bukan miliknya.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Kritis | S1**Captcha login dan signup bisa dilewati di produksi**handlers/auth.go:54 · main.go:271 | Token uji Cloudflare (`XXXX.DUMMY.TOKEN.XXXX`) selalu diterima, tanpa melihat `APP_ENV`. `/auth/login` dan `/auth/signup` tidak punya rate limit, signup tidak memvalidasi email atau panjang password, dan panggilan ke Cloudflare tidak punya timeout.**Perbaikan:** Token uji hanya di development, rate limit per IP dan per email, validasi signup, client HTTP dengan timeout, token tidak di-log. | Selesai[#143](https://github.com/roozylabs/prism/pull/143) |
| Tinggi | S2**Token sesi dan gateway key tercatat di log**middleware/auth.go:35 · main.go:254 | Auth menerima `?token=` untuk SSE, dan logger bawaan `gin.Default()` mencetak query string lengkap. Token sesi 7 hari, `gw_sk_`, dan kode OAuth masuk stdout.**Perbaikan:** `gin.New()` dengan logger yang menyamarkan `token` dan `code`, ditambah `gin.Recovery()`. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Tinggi | S3**Aturan model agent bisa dilewati lewat header**handlers/gateway.go:201 · orchestrator.go:81 · agent_governance.go:30 | Admission membaca nama agent dari header `X-Prism-Agent-Name`, bukan agent yang sudah di-resolve middleware. Nama yang tidak ditemukan dianggap diizinkan, jadi `allowed_models` tidak pernah ditegakkan.**Perbaikan:** Admission memakai agent hasil middleware; agent tidak ditemukan berarti ditolak. | Selesai[#145](https://github.com/roozylabs/prism/pull/145) |
| Sedang | S4**Registry MCP terbaca lintas tenant**repository/mcp_registry.go:48 · handlers/mcp_registry.go:58 | `GET /mcp/registry/:slug` mengembalikan entri privat user lain. `visibility` tidak divalidasi, jadi siapa pun bisa menerbitkan entri publik ke katalog semua tenant; `server_url` tidak dicek terhadap SSRF.**Perbaikan:** Batasi ke publik atau milik sendiri, validasi `visibility`, pakai `ValidateOutboundURL`. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Sedang | S5**Agent template bisa dibaca dan disalin lintas user**repository/agent_template.go:54 | `FindByID` tidak memfilter pemilik; instantiate menyalin template privat user lain.**Perbaikan:** Batasi ke preset atau milik sendiri. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Sedang | S6**Route tanpa izin dan gateway key diterima di dashboard**main.go:349-413 · middleware/auth.go:44 | Routing policy dan rule, log, analitik, simulate, dan acknowledge alert tidak punya guard izin. `AuthMiddleware` menerima `gw_sk_`, jadi viewer atau key inferensi yang bocor bisa membaca log dan mengubah routing.**Perbaikan:** Tambah guard (`logs:read`, `finops:read`, izin routing) dan tolak principal gateway key di route admin. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Sedang | S7**Adapter Paperclip mengubah agent tanpa izin**handlers/paperclip_handler.go:32 | Gateway key apa pun bisa membuat atau mengubah agent beserta `AllowedModels`-nya tanpa `agent:create`.**Perbaikan:** Wajibkan izin dan kepemilikan agent. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Sedang | S8**API terbuka langsung dan IP klien bisa dipalsukan**docker-compose.yml:34,85 · main.go:515 | Port API terbuka di `0.0.0.0` melewati nginx dan TLS; tanpa trusted proxies, `X-Forwarded-For` bisa dipalsukan untuk audit log dan Turnstile.**Perbaikan:** Bind ke `127.0.0.1` dan `SetTrustedProxies`. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Rendah | S9**Proxy Next.js membocorkan URL backend saat 502**apps/app/app/api/\[\[...path\]\]/route.ts | Blok catch mengembalikan `targetUrl` dan detail error ke browser.**Perbaikan:** Kembalikan pesan generik. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |

## Jalur request gateway

Dari middleware sampai adapter provider: timeout, failover, streaming, admission, dan pencatatan biaya.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Tinggi | G1**Failover stream membocorkan slot concurrency**proxy/engine.go:920, 1028 | `defer Body.Close(); release()` dipasang di dalam loop, lalu jalur failover sebelum token pertama langsung `continue`. Body dan slot tertahan sampai seluruh request selesai; dengan batas 2 slot, request lain ke provider yang sama menunggu tanpa batas.**Perbaikan:** Tutup body dan lepas slot sebelum `continue`; defer hanya setelah streaming dimulai. | Selesai[#144](https://github.com/roozylabs/prism/pull/144) |
| Tinggi | G2**Satu 401/403 mematikan credential permanen, atau justru dipulihkan**proxy/engine.go:519, 866, 1399 | Di stream, 401/403 tanpa kata quota menandai credential `invalid`, dan tidak ada jalur pemulihan; satu 403 untuk satu model menghilangkan key itu bagi semua user. Di non-stream, `syncCredentialHealth` menimpa `invalid` menjadi `healthy`, jadi key yang benar-benar dicabut terus dicoba.**Perbaikan:** Bedakan 401 dan 403 (403 jadi cooldown), sync tidak menimpa `invalid`, tambah pengecekan ulang berkala. | Selesai[#144](https://github.com/roozylabs/prism/pull/144) |
| Tinggi | G3**`MAX_RETRIES` tidak pernah dipakai**proxy/engine.go:122 · router.go:207, 525 | Satu request mencoba semua credential semua kandidat model, masing-masing sampai 5 menit menunggu header. Saat upstream bermasalah, beban ke provider berlipat sebanyak jumlah credential.**Perbaikan:** Batasi percobaan ke `maxRetries+1`, deadline total per request, lewati credential lain dari provider yang timeout. | Selesai[#144](https://github.com/roozylabs/prism/pull/144) |
| Tinggi | G4**Body request tanpa batas dan server tanpa ReadTimeout**handlers/gateway.go:165, 456 · main.go:515 | `io.ReadAll` tanpa batas, dan body di-parse dua sampai tiga kali. Satu POST ratusan MB dari pemegang key bisa menghabiskan memori; upload lambat menahan goroutine.**Perbaikan:** `http.MaxBytesReader` di route gateway, `ReadTimeout` dan `IdleTimeout` di server. | Selesai[#144](https://github.com/roozylabs/prism/pull/144) |
| Sedang | G10**Goroutine efek samping tanpa batas di jalur panas**proxy/engine.go:207, 268, 1149 · post_processor.go:107 | Tiap request memicu beberapa goroutine tulis DB dengan `context.Background()` tanpa timeout; pool 25 koneksi tanpa `statement_timeout`. Saat DB lambat, goroutine menumpuk dan latensi runtuh.**Perbaikan:** Lewatkan efek samping ke worker pool terbatas dengan timeout; set `statement_timeout`. | Selesai(batch 12, belum di-PR) |
| Sedang | G11**Redis sekuensial dan hash global di setiap request**proxy/engine.go:359 · redis/cooldown.go:198 | Sekitar 10 panggilan Redis berurutan per request dengan timeout bawaan 3 detik, dan `HGETALL` atas hash semua tenant dua kali per request.**Perbaikan:** Hash per user, publish async, timeout Redis pendek. | Selesai(batch 12, belum di-PR) |
| Sedang | G12**Rate limiter tidak atomik dan menghitung request yang ditolak**middleware/ratelimit.go:27-35 | Request ditambahkan sebelum dicek, jadi klien yang terus mencoba tetap terblokir; member bisa bentrok di milidetik yang sama; batas per key yang tersimpan diabaikan; fail-open saat Redis error.**Perbaikan:** Skrip Lua cek-lalu-tambah dengan member unik, pakai `rate_limit` per key. | Selesai[#153](https://github.com/roozylabs/prism/pull/153) |
| Sedang | G13**Idempotency bisa menjalankan request dua kali**proxy/idempotency.go:20, 78, 85 | Lock 120 detik lebih pendek dari durasi request, release tanpa cek kepemilikan, key yang sama dengan body berbeda memutar ulang respons lama, dan envelope di atas \~3,7 MB terpotong lalu diabaikan.**Perbaikan:** Lock milik token dengan perpanjangan, hash body di key, potong body bukan envelope. | Selesai[#153](https://github.com/roozylabs/prism/pull/153) |
| Sedang | G5**Respons yang gagal di-parse dicoba ulang ke credential lain**proxy/engine.go:571 | Upstream sudah sukses dan sudah menagih, lalu request dikirim lagi ke credential lain: biaya ganda.**Perbaikan:** Kembalikan 502, jangan failover. | Selesai[#144](https://github.com/roozylabs/prism/pull/144) |
| Sedang | G6**Async dengan `stream:true` selalu panic**proxy/engine.go:735, 794 | Worker memanggil `ProxyStream` dengan context gin `nil`; setiap job seperti ini gagal dengan "panic in worker".**Perbaikan:** Tolak atau paksa `stream=false` saat job dibuat. | Selesai[#146](https://github.com/roozylabs/prism/pull/146) |
| Sedang | G7**Budget harian tidak pernah ditegakkan**proxy/budget_manager.go:39, 50, 65 | Status hanya dari belanja bulanan, `DailyLimit` dihitung lalu diabaikan, dan hanya budget pertama yang dicek. `AdmissionDowngrade` tidak pernah diterapkan.**Perbaikan:** Cek harian dan bulanan untuk semua budget aktif. | Selesai[#145](https://github.com/roozylabs/prism/pull/145) |
| Sedang | G8**Query kuota yang macet menahan semua request satu org**proxy/quota_cache.go:61 | Evaluasi bersama berjalan dengan `WithoutCancel` tanpa deadline, dan pemanggil tidak bisa keluar dari `group.Do`.**Perbaikan:** Deadline pada evaluasi dan `DoChan` dengan select pada context pemanggil. | Selesai[#145](https://github.com/roozylabs/prism/pull/145) |
| Sedang | G9**Usage stream Anthropic salah dan error di stream ditelan**proxy/anthropic.go:176 · engine.go:977, 1014 | Usage dibaca dari field yang salah, jadi biaya stream Anthropic selalu estimasi. Event `error` dibuang; stream kosong dicatat sukses 200.**Perbaikan:** Baca `message.usage`; teruskan event error; nol chunk berarti gagal. | Selesai[#154](https://github.com/roozylabs/prism/pull/154) |
| Rendah | G14**Model gratis ditagih tarif heuristik**service/orchestrator.go:164 | Biaya tepat 0 dianggap "belum ada harga", jadi model gratis dikenai $0,001–0,002 per 1K token.**Perbaikan:** Bedakan model gratis dari model tanpa harga; cache pricing. | Selesai[#154](https://github.com/roozylabs/prism/pull/154) |
| Rendah | G15**Stream panjang terputus**proxy/engine.go:124, 925 | Tidak ada keepalive SSE, jadi load balancer dengan idle 60 detik memutus jeda reasoning panjang; client timeout 15 menit membunuh stream yang lebih lama; error sebelum token pertama dikirim dengan content-type stream.**Perbaikan:** Komentar `: ping` berkala, deadline per percobaan, reset header sebelum error. | Selesai[#154](https://github.com/roozylabs/prism/pull/154) |
| Rendah | G16**Gateway key yang dicabut masih berlaku 30 detik**proxy/gateway_key_cache.go:40 | `Invalidate` tidak pernah dipanggil, dan lookup key tidak valid tidak di-cache serta berjalan sebelum rate limit.**Perbaikan:** Invalidate saat delete; cache negatif singkat. | Selesai[#153](https://github.com/roozylabs/prism/pull/153) |

## Operasional dan deploy

Hal-hal di luar request: startup, worker, health check, Redis, backup, rollback, dan pertumbuhan data.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Tinggi | O1**Job async hilang atau macet setiap deploy**queue/job_queue.go:183, 206 · main.go:179 | `BLPOP` mengeluarkan job sebelum diproses dan `jobQueue.Stop()` tidak pernah dipanggil. Saat SIGTERM, job yang berjalan ditinggal; status ditulis dengan context yang sudah dibatalkan, jadi job tetap "processing" sampai 24 jam lalu hilang.**Perbaikan:** `BLMOVE` ke daftar processing per instance, re-queue saat start, hentikan antrean sebelum post-processor, context terpisah untuk tulis status. | Selesai[#146](https://github.com/roozylabs/prism/pull/146) |
| Tinggi | O2**Health check selalu 200**handlers/health.go:28 · docker-compose.yml:47 · api-ci-cd.yml:215 | `/health` mengembalikan 200 walau DB atau Redis mati, dan dipakai healthcheck Docker serta gate deploy. `/ready` yang benar-benar mengecek tidak dipakai.**Perbaikan:** Healthcheck dan gate ke `/ready` dengan timeout ping 2 detik. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Tinggi | O3**Tidak ada jalur rollback**api-ci-cd.yml:200, 235, 246 · docs/operations/rollback.md | Deploy memakai `:latest`, pull gagal ditutupi `\|\| true`, image lama di-prune, dan warna lama tetap dihentikan walau switch nginx gagal. Dokumen rollback menyalakan biru dan hijau bersamaan.**Perbaikan:** Deploy tag `sha-`, simpan tag sebelumnya, gagal bila pull atau nginx gagal, hentikan warna lama setelah switch terkonfirmasi. | Selesai[#148](https://github.com/roozylabs/prism/pull/148) |
| Tinggi | O4**Tabel besar tumbuh tanpa batas**workers/retention.go:62 · repository/session.go:85 | Hanya `request_payloads` dan `tool_invocations` yang dibersihkan. `routing_decisions` menyimpan `prompt_preview`, jadi teks prompt bertahan melewati purge payload 30 hari. `session.DeleteExpired` tidak pernah dipanggil.**Perbaikan:** Retensi per tabel yang bisa diatur, hapus bertahap 10 ribu baris; partisi `request_logs` per bulan nanti. | Selesai[#149](https://github.com/roozylabs/prism/pull/149) |
| Tinggi | O5**Backup tidak otomatis dan dokumennya tidak konsisten**docs/operations/backup-recovery.md · docs/INFRASTRUCTURE.md:482 | Runbook berlabel "VERIFIED" menjanjikan dump per jam ke S3, padahal skripnya hanya ke disk VPS yang sama. Tanpa `pipefail`, pg_dump gagal menghasilkan .gz kosong yang tampak valid, dan nama DB-nya salah.**Perbaikan:** Satu skrip `set -euo pipefail` dengan salinan di luar host, alert saat gagal, uji restore berkala. | Selesai[#148](https://github.com/roozylabs/prism/pull/148) |
| Sedang | O10**Konfigurasi salah jatuh ke default tanpa peringatan**config/config.go:41, 76 · main.go:57 | `APP_ENV` kosong menjadi development (validasi kunci jadi peringatan, Swagger aktif); angka env yang salah dipakai default; `JWT_SECRET` dan `HASH_KEY` tidak dipakai tapi default-nya "secret".**Perbaikan:** Default production, nilai salah berarti gagal start, hapus secret mati, wajibkan password DB dan Redis. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Sedang | O11**Instance baru mereset credential yang sedang dibatasi**main.go:94 | Setiap start menjalankan `UPDATE credentials ... 'rate_limited' → 'active'`, membatalkan state yang baru ditetapkan instance yang masih hidup saat deploy.**Perbaikan:** Hapus; serahkan ke worker credential recovery. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Sedang | O12**Observability praktis mati**telemetry/otel.go:25 · main.go:254 · middleware/correlation.go:39 | `OTEL_ENABLED` tidak diset sehingga metrik no-op, `/metrics` publik, `CorrelationMiddleware` tidak dipasang, log tidak terstruktur, tidak ada alert.**Perbaikan:** Aktifkan metrik internal, pasang correlation ID, log JSON dengan `request_id`. | Selesai(batch 13, belum di-PR) |
| Sedang | O6**Worker per jam bisa tidak pernah jalan**workers/worker.go:39 | Ticker baru berbunyi setelah satu interval penuh dan diulang setiap deploy; dengan lebih dari satu deploy per jam, retensi dan flush latensi tidak pernah jalan.**Perbaikan:** Jalankan sekali saat start dengan jitter. | Selesai[#149](https://github.com/roozylabs/prism/pull/149) |
| Sedang | O7**Post-processor bisa membuat server crash**service/post_processor.go:107, 115, 211 | Saat buffer penuh, satu goroutine per tugas tanpa batas; `processTask` tanpa `recover`; `Stop()` menutup channel saat job masih enqueue, jadi panic saat shutdown.**Perbaikan:** Overflow terbatas, recover dengan log, hentikan produsen dulu. | Selesai[#146](https://github.com/roozylabs/prism/pull/146) |
| Sedang | O8**Redis mati membuat semua request lambat**database/redis.go:11 | Timeout bawaan 5 detik dial dan 3 detik baca dengan 3 retry, dikali belasan panggilan per request. Rate limit, idempotency, dan cooldown fail-open tanpa dokumentasi; Redis tanpa `maxmemory`.**Perbaikan:** Timeout 200–500 ms, `MaxRetries=1`, dokumentasikan fail-open/closed, set `maxmemory`. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Sedang | O9**Migration dirty dipulihkan diam-diam**database/migrate.go:23 | Index `CONCURRENTLY` yang gagal meninggalkan index INVALID, lalu auto-recovery memaksa versi dan `IF NOT EXISTS` melewatinya. Query diam-diam tidak memakai index.**Perbaikan:** Di produksi gagal saat dirty, atau hapus index invalid sebelum mengulang. | Selesai[#147](https://github.com/roozylabs/prism/pull/147) |
| Rendah | O13**Container dan pipeline belum dikeraskan**Dockerfile · .github/workflows/\* | Tanpa batas memori/CPU dan `GOMEMLIMIT`, image `alpine:3.19` sudah EOL dan berjalan sebagai root, action SSH tidak di-pin, coverage tanpa ambang, workflow app tanpa tes.**Perbaikan:** Resource limits, image baru non-root, pin action ke SHA, ambang coverage. | Selesai(batch 13, belum di-PR) |

## Fitur rusak atau tiruan

Fitur yang terlihat selesai di UI tapi tidak bekerja dari ujung ke ujung.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Tinggi | U1**Billing hanya tiruan dan UI-nya tidak cocok**repository/billing.go:20-96 · lib/api/billing.ts:19 | Plan di-hardcode, langganan selalu "Pro, active", upgrade tidak menyimpan apa pun, invoice dan agregat harian tidak pernah ditulis. Frontend memanggil path yang salah (upgrade selalu 404) dan membaca field yang tidak ada, jadi semua plan tampil $0.**Perbaikan:** Sembunyikan di balik flag, atau sambungkan penyimpanan dan samakan field. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |
| Tinggi | U2**Gateway key baru tidak pernah ditampilkan**handlers/gateway_key.go:37 · lib/api/gateway-keys.ts:18 | Backend mengirim field `key`, frontend membaca `rawKey` lalu hanya menampilkan toast. Hanya hash yang disimpan, jadi key yang dibuat lewat UI tidak bisa dipakai.**Perbaikan:** Tampilkan key sekali dengan tombol salin. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |
| Tinggi | U3**Daftar terpotong di 10 baris**handlers/model.go:49 · credential.go:80 · logs/page.tsx:17 · gateway-keys/page.tsx:20 | Backend default `limit=10` dan frontend tidak pernah mengirim halaman: model, credential, log, dan gateway key ke-11 tidak terlihat dan tidak bisa dicabut.**Perbaikan:** Paginasi server-side dengan satu bentuk envelope. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |
| Tinggi | U4**Undangan anggota tidak bisa diterima**handlers/organization_members.go:54 · repository/rbac.go:481 | Undangan dibuat tapi tidak ada endpoint terima dan tidak ada email, sementara token undangan ikut dikembalikan di respons. Org dengan banyak anggota tidak bisa dibentuk.**Perbaikan:** Endpoint terima + email; jangan kembalikan token. | Selesai(batch 14, belum di-PR) |
| Tinggi | U5**Audit log admin selalu kosong**repository/audit_trail.go:22 · handlers/audit_trail.go:167 | `CreateAuditLog` tidak punya pemanggil. Ekspor CSV tidak meng-escape tanda kutip, rentan formula injection, dan terpotong di 1000 baris.**Perbaikan:** Tulis audit pada perubahan anggota, key, credential, policy, setting; pakai `encoding/csv`. | Selesai(batch 14, belum di-PR) |
| Tinggi | U6**Halaman Settings tidak menyimpan apa pun**app/(dashboard)/settings/page.tsx:67 | Tombol simpan menunggu `setTimeout` lalu menampilkan sukses; region, MFA, dan session timeout hanya ada di UI. Pengaturan yang benar ada di `/settings/organization`.**Perbaikan:** Arahkan ke halaman pengaturan org yang nyata dan hapus field palsu. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |
| Sedang | U7**Routing rule tidak dipakai siapa pun**main.go:382 | CRUD lengkap, tapi router dan frontend tidak membaca `routing_rules`.**Perbaikan:** Hapus fiturnya atau sambungkan ke router. | Selesai(batch 15, belum di-PR) |
| Sedang | U8**Pemilih policy di sandbox tidak berpengaruh**useSandboxMutation.ts:23 · engine.go:237 | Frontend mengirim `X-Routing-Policy` yang tidak pernah dibaca, dan membaca header `X-Prism-Selected-Model` yang tidak pernah diset.**Perbaikan:** Baca header di engine atau hapus pemilihnya. | Selesai(batch 15, belum di-PR) |
| Sedang | U9**Routing policy default bisa hilang**repository/routing_policy.go:25, 156 | `SetDefault` menghapus default lama sebelum mengecek id baru milik user; id asing membuat user tanpa default tapi tetap 200. Policy nonaktif hilang dari daftar dan tidak bisa diaktifkan lagi.**Perbaikan:** Cek kepemilikan dulu dalam transaksi; tampilkan policy nonaktif. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Rendah | U10**Bentuk respons tidak konsisten di frontend**lib/api/client.ts · lib/api/\*.ts | Beberapa endpoint mengembalikan `{object,data}` tapi diketik sebagai array; tiap halaman membongkar sendiri. 401 tidak mengarahkan ke login, dan React Query mengulang 401/403/404 tiga kali.**Perbaikan:** Normalisasi di client, redirect saat 401, tanpa retry untuk 4xx. | Selesai[#150](https://github.com/roozylabs/prism/pull/150) |

## Validasi dan penanganan error

Input yang seharusnya ditolak dengan jelas, tapi diterima atau berakhir 500.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Sedang | V1**Angka negatif dan enum sembarang diterima**handlers/budget.go:44 · quota.go:91 · governance_policy.go:22 | Budget dan kuota menerima nilai negatif, ambang warning bisa melebihi critical, `effect` governance seperti "Deny " membuat policy yang tidak pernah cocok.**Perbaikan:** Validasi rentang dan enum. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |
| Sedang | V2**Kesalahan user dijawab 500**repository/\* · handlers/dashboard.go:26 | Nama duplikat (23505) jadi 500 bukan 409, tanggal tidak valid langsung ke SQL, `days` tanpa batas, Delete id asing tetap 200.**Perbaikan:** Petakan 23505 ke 409, validasi tanggal, cek `RowsAffected`. | Selesai[#151](https://github.com/roozylabs/prism/pull/151) |

## Cakupan tes

Area yang belum punya tes sehingga regresi di atas tidak tertangkap CI.

| Tingkat | Temuan | Dampak dan perbaikan | Status |
| --- | --- | --- | --- |
| Sedang | T1**Frontend tanpa tes sama sekali**apps/app/package.json | Tidak ada tooling tes; CI hanya lint dan typecheck. Bug seperti U2 dan U3 lolos tanpa terdeteksi.**Perbaikan:** Mulai dengan tes komponen untuk alur key, paginasi, dan auth. | Selesai(batch 16, belum di-PR) |
| Sedang | T2**Banyak handler dan repository tanpa tes**internal/handlers · internal/repository | Handler agent, budget, credential, dashboard, finops, gateway key, logs, model, onboarding, policy, resource, tool, serta login/signup tanpa tes. Repository billing, budget, routing policy, request log, audit, gateway key, dan registry tanpa tes DB.**Perbaikan:** Tambah tes bersama setiap batch perbaikan di area tersebut. | Selesai(batch 16, belum di-PR; finops, resource, tool dan login/signup handler masih tanpa tes khusus) |

## Matriks fitur

Status setiap fitur dari route sampai halaman. "Tersambung" berarti route, handler, repository, dan tabelnya ada dan dipakai.

| Fitur | Backend | Frontend | Tes | Catatan |
| --- | --- | --- | --- | --- |
| **Auth dan OAuth** | Tersambung | Tersambung | OAuth ya, login/signup tidak | Captcha bisa dilewati (S1) |
| **Onboarding** | Tersambung | Tersambung | Tidak ada | Provider dan key awal diabaikan |
| **Anggota dan RBAC** | Undangan buntu | Tersambung | Repository dan security | Undangan tidak bisa diterima (U4) |
| **Provider** | Tersambung | Tersambung | Ada | Baik |
| **Credential** | Tersambung | Terpotong 10 baris | Hanya Google OAuth | U3; 401/403 mematikan key (G2) |
| **Model** | Tersambung | Terpotong 10 baris | Tidak ada | U3 |
| **Routing policy** | Tanpa guard izin | Tersambung | Engine ya, handler tidak | S6, U9 |
| **Routing rule** | Tersambung ke router | Tidak ada | Tidak ada | U7 selesai (batch 15); UI belum ada |
| **Playground dan sandbox** | Tersambung | Tersambung | Sandbox ya | U8 selesai (batch 15) |
| **Gateway key** | Tersambung | Key tidak ditampilkan | Role dan sandbox | U2, U3, G16 |
| **Budget** | Harian tidak ditegakkan | Tersambung | Manager saja | G7, V1 |
| **FinOps dan anomali** | Tersambung | Belum dipakai | Forecast dan anomali | Backend saja |
| **Kuota** | Tersambung | Tersambung (edit) | Ada | Baik setelah #141 |
| **Billing** | Tiruan | Field dan path tidak cocok | Minim | Rusak (U1) |
| **Agent** | Tersambung | Tersambung | Proxy saja | Aturan model bisa dilewati (S3) |
| **Agent template** | Lintas user | Belum dipakai | Bind saja | S5 |
| **Tool dan resource** | Tersambung | Tersambung | Proxy saja | Baik |
| **MCP server** | Tersambung | Tersambung | Sebagian | Baik |
| **Registry MCP** | Lintas tenant | Path salah, hanya daftar | Bind saja | S4 |
| **Governance** | Tersambung | Tersambung | Proxy | Enum tidak divalidasi (V1) |
| **Audit trail AI** | Tersambung | Tersambung | Recorder | Baik |
| **Audit log admin** | Tidak pernah ditulis | Tersambung | Tidak ada | Selalu kosong (U5) |
| **Settings** | Per org | /settings palsu | Ada | U6 |
| **Log, analitik, dashboard** | Tanpa guard izin | Log terpotong | Tidak ada | U3, S6 |
| **Job async** | Hilang saat deploy | Tanpa fallback polling | Tidak ada | O1, G6 |

## Yang sudah solid

Bagian yang sudah dirancang dengan benar dan tidak perlu disentuh.

- Pembatalan oleh klien diteruskan ke upstream dan tidak menghukum credential.
- Backoff berjitter dengan batas, menghormati pembatalan.
- Failover sebelum token pertama; event error eksplisit bila stream putus di tengah.
- Cooldown dari `Retry-After`, circuit breaker per credential, cek cooldown dalam satu `MGET`.
- Transport HTTP dengan timeout dial, TLS, dan header; buffer scanner dibatasi 20 MB.
- RBAC dan pengecekan agent fail-closed saat DB error; role dari keanggotaan.
- Pencatatan request dan metering async dengan worker terbatas.
- Blue/green aman dari pemrosesan ganda: anomali dan alert dengan `ON CONFLICT`, recovery credential bersyarat.
- Setiap `BeginTx` punya `Rollback`; list endpoint membatasi limit.
- Key Redis punya TTL; cache kuota dan gateway key terbatas; SSE berlangganan ulang setelah Redis putus.
- CI API menjalankan `-race` dengan Postgres dan Redis sungguhan; tes rantai migration menolak regresi skema.
- `ENCRYPTION_KEY` wajib dan divalidasi, tersedia alat re-encrypt.

Audit ini membaca kode di `main` tanpa menjalankan produksi, karena VPS sedang mati. Temuan yang paling berat (S1, S3, G1, G3, U2, O2) sudah diverifikasi langsung di kode. Perbaikan skema sebelumnya (#130–#142) ada di laporan skema terpisah dan tidak diulang di sini.