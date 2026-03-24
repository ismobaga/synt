# Synt — AI Short-Video Generation Platform

> Enter a topic, get a publish-ready short video.

Synt automatically generates **1080×1920 vertical MP4 videos** ready for TikTok, Instagram Reels, and YouTube Shorts from a simple topic or keyword.

## What It Does

Given a topic, Synt:

1. **Generates a structured script** (title, hook, scenes, CTA) via LLM
2. **Searches stock footage/images** for each scene
3. **Synthesizes voiceover** narration via TTS
4. **Creates phrase-based subtitles** with timing
5. **Selects background music** matched to the script's mood
6. **Builds a render manifest** (deterministic JSON timeline)
7. **Renders the final HD video** via FFmpeg

## Architecture

```
[React Frontend (TypeScript + Tailwind)]
         |
         v
[API Gateway (Go)]
         |
         +-----> [PostgreSQL]
         +-----> [Redis Queue]
         +-----> [Object Storage: S3/MinIO]
         |
         +--> [Content Service]   — LLM script generation
         +--> [Media Service]     — Stock media search & prep
         +--> [Voice Service]     — TTS narration
         +--> [Subtitle Service]  — SRT/VTT generation
         +--> [Music Service]     — Licensed track selection
         +--> [Render Service]    — FFmpeg video assembly
         +--> [Moderation]        — Content safety checks
```

### Stack

| Layer       | Technology                                      |
|-------------|--------------------------------------------------|
| Frontend    | React, TypeScript, Tailwind CSS, shadcn/ui       |
| API Gateway | Go (standard library)                            |
| Worker      | Go (background job processor)                   |
| Database    | PostgreSQL                                       |
| Queue       | Redis                                            |
| Storage     | MinIO (S3-compatible)                            |
| Video       | FFmpeg                                           |
| Auth        | JWT (golang-jwt)                                |

## Repository Structure

```
synt/
├── apps/
│   ├── api-gateway/     — HTTP API server (Go)
│   ├── worker/          — Background job worker (Go)
│   └── web/             — React frontend
├── internal/
│   ├── api/             — HTTP handlers & router
│   ├── auth/            — JWT middleware
│   ├── content/         — Script generation service
│   ├── db/              — Database models & queries
│   ├── jobs/            — Job worker & Redis queue
│   ├── media/           — Media search & preparation
│   ├── moderation/      — Content safety
│   ├── music/           — Music selection
│   ├── orchestrator/    — Pipeline coordination
│   ├── render/          — FFmpeg rendering
│   ├── subtitle/        — SRT/VTT generation
│   └── voice/           — TTS synthesis
├── pkg/
│   ├── ffmpeg/          — FFmpeg runner interface
│   ├── llm/             — LLM client interface + stub
│   ├── retry/           — Exponential backoff retry
│   ├── s3util/          — S3-compatible storage client
│   ├── tts/             — TTS client interface + stub
│   └── validator/       — Input validation
├── db/
│   └── migrations/      — PostgreSQL migrations
├── templates/           — Video rendering templates
└── deployments/
    ├── compose/         — Docker Compose
    └── docker/          — Dockerfiles
```

## Quick Start (Docker Compose)

```bash
git clone https://github.com/ismobaga/synt
cd synt
docker compose -f deployments/compose/docker-compose.yml up -d
# Frontend: http://localhost:3000
# API:      http://localhost:8080
```

## Local Development

### Backend

```bash
export DATABASE_URL="postgres://synt:synt@localhost:5432/synt?sslmode=disable"
export REDIS_ADDR="localhost:6379"
psql $DATABASE_URL -f db/migrations/001_initial_schema.sql
go run ./apps/api-gateway   # Terminal 1
go run ./apps/worker        # Terminal 2
```

### Frontend

```bash
cd apps/web
npm install
VITE_API_URL=http://localhost:8080 npm run dev
# Open http://localhost:3000
```

## API Reference

### Create + Generate Example

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "5 AI tools for small businesses",
    "language": "en",
    "platform": "youtube_shorts",
    "duration_sec": 30,
    "tone": "educational",
    "template_id": "fast_caption_v1"
  }'

curl -X POST http://localhost:8080/v1/projects/{id}/generate \
  -d '{"auto_render":true}'

curl http://localhost:8080/v1/projects/{id}/status
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/projects` | Create project |
| `GET` | `/v1/projects` | List projects |
| `GET` | `/v1/projects/:id` | Get project |
| `DELETE` | `/v1/projects/:id` | Delete project |
| `POST` | `/v1/projects/:id/generate` | Start generation |
| `GET` | `/v1/projects/:id/status` | Poll status |
| `POST` | `/v1/projects/:id/retry` | Retry failed |
| `GET` | `/v1/projects/:id/script` | Get script |
| `PUT` | `/v1/projects/:id/script` | Update script |
| `GET` | `/v1/projects/:id/assets` | Get assets |
| `GET` | `/v1/projects/:id/audio` | Get audio |
| `GET` | `/v1/projects/:id/subtitles` | Get subtitles |
| `POST` | `/v1/projects/:id/render/preview` | Render preview |
| `POST` | `/v1/projects/:id/render/final` | Render final HD |
| `GET` | `/v1/templates` | List templates |
| `POST` | `/v1/brand-kits` | Create brand kit |

## Generation Pipeline

```
project:generate
   → script:generate        (LLM → structured JSON)
   → script:validate        (content moderation)
   → media:search           (stock footage per scene)
   → media:prepare          (transcode, crop to 9:16)
   → voice:generate         (TTS narration)
   → subtitle:generate      (SRT with phrase timing)
   → music:select           (licensed track by mood)
   → timeline:build         (JSON render manifest)
   → render:preview         (720×1280 preview)
   → render:final           (1080×1920 HD MP4)
   → render:thumbnail       (JPG thumbnail)
   → project:finalize       (status → done)
```

## Templates

| ID | Name | Style |
|----|------|-------|
| `fast_caption_v1` | Fast Captions | Phrase-based animated captions |
| `minimal_clean_v1` | Minimal Clean | Sentence captions, subtle animations |
| `promo_bold_v1` | Promo Bold | Word-by-word bold, energetic |

## Output Format

- **Resolution**: 1080×1920 (9:16 vertical)
- **Format**: MP4 (H.264 + AAC)
- **Frame Rate**: 30fps
- **Platforms**: TikTok, Instagram Reels, YouTube Shorts
- **Durations**: 15s / 30s / 60s

## Tests

```bash
go test ./...
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://synt:synt@localhost:5432/synt?sslmode=disable` | PostgreSQL DSN |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `ADDR` | `:8080` | API listen address |
| `JWT_SECRET` | — | JWT signing secret |

## Plugging In Real AI Providers

Replace stub clients in `pkg/llm` and `pkg/tts`:

- **LLM**: OpenAI GPT-4, Anthropic Claude, Google Gemini
- **TTS**: Google Cloud TTS, ElevenLabs, Amazon Polly
- **Media**: Pexels API, Pixabay API, Unsplash API
