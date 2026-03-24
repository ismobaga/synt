-- Migration 001: initial schema

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT,
  plan TEXT NOT NULL DEFAULT 'free',
  credits INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS projects (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  topic TEXT NOT NULL,
  language TEXT NOT NULL,
  platform TEXT NOT NULL,
  duration_sec INT NOT NULL,
  tone TEXT,
  template_id TEXT,
  status TEXT NOT NULL,
  current_stage TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scripts (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title TEXT,
  hook TEXT,
  cta TEXT,
  language TEXT NOT NULL,
  content_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS assets (
  id UUID PRIMARY KEY,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  source TEXT NOT NULL,
  provider TEXT,
  url TEXT,
  storage_path TEXT,
  mime_type TEXT,
  width INT,
  height INT,
  duration_sec NUMERIC,
  license_info JSONB,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audio_tracks (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  voice_name TEXT,
  language TEXT,
  storage_path TEXT NOT NULL,
  duration_sec NUMERIC,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS subtitles (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  format TEXT NOT NULL,
  storage_path TEXT NOT NULL,
  content JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS renders (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  resolution TEXT NOT NULL,
  fps INT NOT NULL,
  storage_path TEXT,
  thumbnail_path TEXT,
  status TEXT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS jobs (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  payload JSONB,
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,
  last_error TEXT,
  scheduled_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  category TEXT,
  config JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS brand_kits (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id),
  name TEXT NOT NULL,
  logo_path TEXT,
  primary_color TEXT,
  secondary_color TEXT,
  font_family TEXT,
  outro_text TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed templates
INSERT INTO templates (id, name, category, config, created_at)
VALUES
  ('fast_caption_v1', 'Fast Captions', 'caption', '{
    "id": "fast_caption_v1",
    "name": "Fast Captions",
    "video": {"aspect_ratio": "9:16", "fps": 30},
    "captions": {"mode": "phrase", "position": "lower_third", "highlight_current_word": true},
    "overlays": {"hook_position": "top_center", "cta_position": "bottom_center"},
    "transitions": ["quick_fade", "zoom_cut"],
    "max_scenes": 6
  }', NOW()),
  ('minimal_clean_v1', 'Minimal Clean', 'minimal', '{
    "id": "minimal_clean_v1",
    "name": "Minimal Clean",
    "video": {"aspect_ratio": "9:16", "fps": 30},
    "captions": {"mode": "sentence", "position": "bottom", "highlight_current_word": false},
    "overlays": {"hook_position": "center", "cta_position": "bottom_center"},
    "transitions": ["fade", "cut"],
    "max_scenes": 5
  }', NOW()),
  ('promo_bold_v1', 'Promo Bold', 'promo', '{
    "id": "promo_bold_v1",
    "name": "Promo Bold",
    "video": {"aspect_ratio": "9:16", "fps": 30},
    "captions": {"mode": "word", "position": "center", "highlight_current_word": true},
    "overlays": {"hook_position": "top_center", "cta_position": "bottom_center"},
    "transitions": ["zoom_cut", "flash"],
    "max_scenes": 8
  }', NOW())
ON CONFLICT (id) DO NOTHING;
