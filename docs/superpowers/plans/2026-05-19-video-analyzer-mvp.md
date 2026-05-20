# Video Analyzer MVP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy a single-user video analyzer to Railway today: user uploads MP4 directly to GCS via signed URL, backend (Go) orchestrates Google Video Intelligence + Claude analysis, frontend (React) polls and displays structured insights.

**Architecture:** 3 Railway services in one project — `web` (React/Vite served by Caddy), `api` (Go binary + Node sidecar running modified `analyze-video.js`), and `postgres` plugin. Video file never touches the Go process (direct browser→GCS upload). Job state lives in a single `analyses` table; goroutine + polling, no Redis/queue.

**Tech Stack:**
- Backend: Go 1.22, chi v5 (HTTP), pgx v5 (Postgres), cloud.google.com/go/storage (GCS), net/http (Claude), slog (logging)
- Sidecar: Node 20 running `analyze-video.js` (modified for `inputUri`)
- Frontend: Vite + React 18 + TypeScript + Tailwind v3
- Infra: Railway services + Postgres plugin + GCS bucket
- Tests: Go testing + table-driven; Postgres real via local docker-compose; integration tests with tiny fixture video

**Reference docs (read before starting any task):**
- `docs/superpowers/specs/2026-05-19-video-analyzer-mvp-design.md` — full spec
- `content-strategy.md` — Claude system prompt
- `video-analyzer-spec.md` — original product spec (context only, not the source of truth for implementation)

**Prerequisite (BLOCKING):** The Anthropic API key pasted in chat during brainstorm is compromised. Rotate at https://console.anthropic.com/settings/keys BEFORE Task 33.

**Tooling assumed available:** `git`, `go 1.22+`, `node 20+`, `npm`, `docker` + `docker compose`, `gcloud` CLI, `jq`, `psql`, `curl`. On macOS, `base64 -w0` does not exist — use `base64` (no flag) instead; output is the same single-line base64 the env var needs.

**Module name choice:** This plan uses `github.com/leoklaser/video-analyzer/api` as the Go module path. If you prefer a different name, change it in Task 2 step 1 and then run `find /workspace/api -name "*.go" -exec sed -i 's|github.com/leoklaser/video-analyzer/api|<your-new-path>|g' {} +` before Task 7.

---

## Phase 0: Repo bootstrap

### Task 1: Initialize repo, .gitignore, .env.example, move tools/

**Files:**
- Create: `/workspace/.gitignore`
- Create: `/workspace/.env.example`
- Create: `/workspace/README.md`
- Move: `/workspace/tools/` → `/workspace/api/tools/` (analyze-video.js + package.json + package-lock.json + node_modules ignored)
- Delete: `/workspace/.DS_Store`, `/workspace/tools/.DS_Store`

- [ ] **Step 1: git init**

```bash
cd /workspace && git init -b main
```

Expected: `Initialized empty Git repository in /workspace/.git/`

- [ ] **Step 2: Write .gitignore**

```gitignore
# Secrets
*.json
!package.json
!package-lock.json
!tsconfig*.json
!vite.config.json
.env
.env.local

# OS
.DS_Store
Thumbs.db

# Node
node_modules/
dist/
.vite/

# Go
*.exe
*.test
*.out
/api/server
/api/coverage.*

# Logs
*.log

# Temp
/tmp/
testdata/*.mp4
!testdata/*-keep.mp4
```

Note: `*.json` is broad on purpose — service account JSONs at the repo root will not be accidentally committed.

- [ ] **Step 3: Write .env.example (template at root)**

```bash
# === api service ===
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/video_analyzer
PORT=8080
ALLOWED_ORIGINS=http://localhost:5173

# GCP — service account JSON encoded as base64 (cat key.json | base64 -w0)
GOOGLE_APPLICATION_CREDENTIALS_JSON=
GCS_BUCKET=video-analyzer-tmp
GCP_PROJECT_ID=gen-lang-client-0123498892

# Anthropic
ANTHROPIC_API_KEY=
ANTHROPIC_MODEL=claude-sonnet-4-6

# === web service (build-time only) ===
VITE_API_URL=http://localhost:8080
```

- [ ] **Step 4: Write minimal README.md**

```markdown
# Video Analyzer

SaaS de análise de vídeos pra criadores. Backend Go + frontend React + Postgres + GCS + Claude.

## Quick start (dev)

```bash
# Postgres local
docker compose up -d postgres

# Backend
cd api && cp ../.env.example ./.env && go run ./cmd/server

# Frontend
cd web && npm install && npm run dev
```

Specs: `docs/superpowers/specs/`. Plan: `docs/superpowers/plans/`.
```

- [ ] **Step 5: Move tools/ into api/tools/, drop .DS_Store and node_modules**

```bash
cd /workspace
mkdir -p api
mv tools api/tools
rm -f api/tools/.DS_Store .DS_Store
rm -rf api/tools/node_modules api/tools/outputs/*.json
```

- [ ] **Step 6: Verify and commit**

```bash
ls /workspace/api/tools/
# expected: analyze-video.js  package.json  package-lock.json  README.md  outputs/ (dir empty)

git add .
git status
# verify no .json secret files are staged

git commit -m "chore: initialize monorepo, move tools/ into api/"
```

---

### Task 2: Bootstrap Go module + dependencies

**Files:**
- Create: `/workspace/api/go.mod`
- Create: `/workspace/api/cmd/server/main.go` (stub)

- [ ] **Step 1: Initialize Go module**

```bash
cd /workspace/api
go mod init github.com/leoklaser/video-analyzer/api
```

Note: replace `leoklaser` with your GitHub username if different. Module path doesn't need to match GitHub for local builds.

- [ ] **Step 2: Add dependencies**

```bash
cd /workspace/api
go get github.com/go-chi/chi/v5
go get github.com/go-chi/chi/v5/middleware
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/jackc/pgx/v5
go get github.com/google/uuid
go get cloud.google.com/go/storage
go get google.golang.org/api/option
```

- [ ] **Step 3: Write minimal main.go that builds**

```go
package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("video-analyzer api boot stub")
}
```

- [ ] **Step 4: Verify it builds**

```bash
cd /workspace/api && go build ./...
```

Expected: no errors, no output.

- [ ] **Step 5: Commit**

```bash
git add api/go.mod api/go.sum api/cmd/server/main.go
git commit -m "chore(api): bootstrap go module + deps"
```

---

### Task 3: Bootstrap Vite + React + TypeScript + Tailwind

**Files:**
- Create: full Vite scaffold under `/workspace/web/`

- [ ] **Step 1: Create Vite project**

```bash
cd /workspace
npm create vite@latest web -- --template react-ts
cd web
npm install
```

- [ ] **Step 2: Add Tailwind v3 + PostCSS**

```bash
cd /workspace/web
npm install -D tailwindcss@^3 postcss autoprefixer
npx tailwindcss init -p
```

This creates `tailwind.config.js` and `postcss.config.js`.

- [ ] **Step 3: Configure Tailwind content paths**

Overwrite `/workspace/web/tailwind.config.js`:

```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {},
  },
  plugins: [],
};
```

- [ ] **Step 4: Add Tailwind directives to index.css**

Overwrite `/workspace/web/src/index.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

html, body, #root { height: 100%; }
body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; }
```

- [ ] **Step 5: Replace App.tsx with a "hello tailwind" smoke check**

Overwrite `/workspace/web/src/App.tsx`:

```tsx
export default function App() {
  return (
    <div className="h-full flex items-center justify-center bg-zinc-900 text-zinc-100">
      <h1 className="text-2xl font-semibold">video-analyzer · web boot ok</h1>
    </div>
  );
}
```

- [ ] **Step 6: Run dev server and verify**

```bash
cd /workspace/web && npm run dev
```

Expected: server on http://localhost:5173 with the dark page above. Stop with Ctrl+C.

- [ ] **Step 7: Commit**

```bash
git add web/
git commit -m "chore(web): bootstrap vite + react + tailwind"
```

---

## Phase 1: Local infra

### Task 4: docker-compose for local Postgres

**Files:**
- Create: `/workspace/docker-compose.yml`

- [ ] **Step 1: Write docker-compose.yml**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: video_analyzer
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

- [ ] **Step 2: Start and verify**

```bash
cd /workspace
docker compose up -d postgres
docker compose ps
# expect: postgres healthy
docker compose exec postgres psql -U postgres -d video_analyzer -c "SELECT version();"
```

Expected: prints Postgres version string.

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "chore: add local postgres via docker compose"
```

---

### Task 5: GCS bucket + service account permissions (manual via gcloud)

This task is mostly executed in the user's terminal (gcloud commands), not by an agent. If executing as an agent and `gcloud` is not authenticated, ask the user to run these manually.

- [ ] **Step 1: Set project context and bucket name**

```bash
export GCP_PROJECT_ID=gen-lang-client-0123498892   # or your project ID
export GCS_BUCKET=video-analyzer-tmp
export SA_EMAIL=$(jq -r .client_email /workspace/gen-lang-client-*.json)
echo "Project: $GCP_PROJECT_ID  Bucket: $GCS_BUCKET  SA: $SA_EMAIL"
```

- [ ] **Step 2: Create bucket**

```bash
gcloud storage buckets create gs://$GCS_BUCKET \
  --project=$GCP_PROJECT_ID \
  --location=us-central1 \
  --uniform-bucket-level-access
```

If `gcloud` is not installed, create the bucket in the GCP Console: Storage → Create bucket → name `video-analyzer-tmp`, location `us-central1`, uniform bucket-level access.

- [ ] **Step 3: Set CORS (allow PUT from frontend domain)**

```bash
cat > /tmp/cors.json <<'EOF'
[
  {
    "origin": ["http://localhost:5173", "https://*.up.railway.app"],
    "method": ["PUT", "GET"],
    "responseHeader": ["Content-Type", "x-goog-content-length-range"],
    "maxAgeSeconds": 3600
  }
]
EOF

gcloud storage buckets update gs://$GCS_BUCKET --cors-file=/tmp/cors.json
```

- [ ] **Step 4: Lifecycle rule (auto-delete after 24h)**

```bash
cat > /tmp/lifecycle.json <<'EOF'
{
  "lifecycle": {
    "rule": [
      { "action": { "type": "Delete" },
        "condition": { "age": 1 } }
    ]
  }
}
EOF

gcloud storage buckets update gs://$GCS_BUCKET --lifecycle-file=/tmp/lifecycle.json
```

- [ ] **Step 5: Grant SA permissions**

```bash
# Object admin on the bucket
gcloud storage buckets add-iam-policy-binding gs://$GCS_BUCKET \
  --member="serviceAccount:$SA_EMAIL" \
  --role="roles/storage.objectAdmin"

# Token creator on the SA itself (required for signed URL v4 without local key)
gcloud iam service-accounts add-iam-policy-binding $SA_EMAIL \
  --project=$GCP_PROJECT_ID \
  --member="serviceAccount:$SA_EMAIL" \
  --role="roles/iam.serviceAccountTokenCreator"
```

- [ ] **Step 6: Enable Video Intelligence API**

```bash
gcloud services enable videointelligence.googleapis.com --project=$GCP_PROJECT_ID
```

- [ ] **Step 7: Smoke test: upload a tiny file via gsutil/gcloud and delete**

```bash
echo "hello" > /tmp/hello.txt
gcloud storage cp /tmp/hello.txt gs://$GCS_BUCKET/hello.txt
gcloud storage rm gs://$GCS_BUCKET/hello.txt
```

Expected: both succeed without permission errors.

- [ ] **Step 8: Encode SA JSON as base64 for env var (used in Task 8 and Railway deploy)**

```bash
base64 -w0 /workspace/gen-lang-client-*.json > /tmp/sa.b64
echo "Length: $(wc -c < /tmp/sa.b64) bytes"
# Copy contents into .env or Railway secret later; do NOT commit
```

No commit — this task is infra setup.

---

### Task 6: Adjust analyze-video.js to use inputUri + --json-stdout

**Files:**
- Modify: `/workspace/api/tools/analyze-video.js`

- [ ] **Step 1: Read current state of the file**

```bash
head -40 /workspace/api/tools/analyze-video.js
```

Expected: matches the file we inspected during brainstorm.

- [ ] **Step 2: Add `--uri` and `--json-stdout` flags; switch from `inputContent` to `inputUri`**

Replace the entire file with this:

```javascript
// analyze-video.js — Google Video Intelligence wrapper
// Modes:
//   CLI (legacy):    node analyze-video.js ./video.mp4 [--name NAME]
//   Backend invoke:  node analyze-video.js --uri gs://bucket/key.mp4 --json-stdout

const videoIntelligence = require('@google-cloud/video-intelligence');
const fs = require('fs');
const path = require('path');

const client = new videoIntelligence.VideoIntelligenceServiceClient();

function parseArgs() {
  const args = process.argv.slice(2);
  const get = (flag) => {
    const i = args.indexOf(flag);
    return i !== -1 ? args[i + 1] : null;
  };
  const has = (flag) => args.includes(flag);

  return {
    filePath: args.find((a) => !a.startsWith('--') && !args[args.indexOf(a) - 1]?.startsWith('--')) || null,
    uri: get('--uri'),
    name: get('--name'),
    jsonStdout: has('--json-stdout'),
  };
}

function classifyRhythm(avgDuration) {
  if (avgDuration < 2) return 'fast';
  if (avgDuration < 4) return 'medium';
  return 'slow';
}

function parseTimestamp(timeOffset) {
  if (!timeOffset) return 0;
  const seconds = parseInt(timeOffset.seconds || 0);
  const nanos = parseInt(timeOffset.nanos || 0);
  return seconds + nanos / 1e9;
}

function processLabels(annotationResults) {
  const videoLabels = (annotationResults.segmentLabelAnnotations || [])
    .map((l) => l.entity.description)
    .slice(0, 15);

  const shotLabels = (annotationResults.shotLabelAnnotations || []).map((label) => ({
    label: label.entity.description,
    category: label.categoryEntities?.map((c) => c.description) || [],
    segments: (label.segments || []).map((seg) => ({
      start: `${parseTimestamp(seg.segment.startTimeOffset).toFixed(1)}s`,
      end: `${parseTimestamp(seg.segment.endTimeOffset).toFixed(1)}s`,
      confidence: Math.round(seg.confidence * 100) / 100,
    })),
  }));

  const frameLabels = {};
  (annotationResults.frameLabelAnnotations || []).forEach((label) => {
    (label.frames || []).forEach((frame) => {
      const time = Math.round(parseTimestamp(frame.timeOffset));
      const key = `${time}s`;
      if (!frameLabels[key]) frameLabels[key] = [];
      if (!frameLabels[key].includes(label.entity.description)) {
        frameLabels[key].push(label.entity.description);
      }
    });
  });

  return { videoLabels, shotLabels, frameLabels };
}

function processShotChanges(annotationResults) {
  const shots = annotationResults.shotAnnotations || [];
  if (shots.length === 0) {
    return { total: 0, averageDuration: 0, timestamps: [], rhythm: 'unknown' };
  }
  const durations = shots.map((s) => parseTimestamp(s.endTimeOffset) - parseTimestamp(s.startTimeOffset));
  const total = durations.reduce((a, b) => a + b, 0);
  const avg = total / durations.length;
  return {
    total: shots.length,
    averageDuration: Math.round(avg * 100) / 100,
    timestamps: shots.map((s) => `${parseTimestamp(s.startTimeOffset).toFixed(1)}s`),
    durations: durations.map((d) => `${d.toFixed(2)}s`),
    rhythm: classifyRhythm(avg),
  };
}

function buildContentInsights(shotChanges, labels, duration) {
  const cutsPerSecond = duration > 0 ? shotChanges.total / duration : 0;
  const firstShotLabels = labels.shotLabels
    .filter((l) => l.segments.some((s) => s.start === '0.0s'))
    .map((l) => l.label)
    .slice(0, 5);

  const firstFrameLabels = [];
  for (const [time, labs] of Object.entries(labels.frameLabels)) {
    if (parseFloat(time) <= 3) {
      labs.forEach((l) => {
        if (!firstFrameLabels.includes(l)) firstFrameLabels.push(l);
      });
    }
  }

  return {
    cutsPerSecond: Math.round(cutsPerSecond * 100) / 100,
    rhythm: shotChanges.rhythm,
    totalShots: shotChanges.total,
    averageShotDuration: `${shotChanges.averageDuration}s`,
    firstShotLabels,
    firstFrameLabels: firstFrameLabels.slice(0, 10),
    dominantLabels: labels.videoLabels.slice(0, 5),
  };
}

function buildRequest({ uri, filePath }) {
  const features = ['LABEL_DETECTION', 'SHOT_CHANGE_DETECTION'];
  const videoContext = {
    labelDetectionConfig: { labelDetectionMode: 'SHOT_AND_FRAME_MODE', model: 'builtin/latest' },
    shotChangeDetectionConfig: { model: 'builtin/latest' },
  };
  if (uri) {
    return { inputUri: uri, features, videoContext };
  }
  // legacy local path mode
  const inputContent = fs.readFileSync(filePath).toString('base64');
  return { inputContent, features, videoContext };
}

async function analyze() {
  const { filePath, uri, name, jsonStdout } = parseArgs();

  if (!uri && !filePath) {
    console.error('Usage: node analyze-video.js (<path> [--name NAME] | --uri gs://... [--json-stdout])');
    process.exit(1);
  }
  if (filePath && !fs.existsSync(filePath)) {
    console.error(`File not found: ${filePath}`);
    process.exit(1);
  }

  const source = uri ? `(uri) ${uri}` : `(file) ${filePath}`;
  if (!jsonStdout) console.log(`Analyzing ${source} ...`);

  const request = buildRequest({ uri, filePath });

  try {
    const [operation] = await client.annotateVideo(request);
    const [operationResult] = await operation.promise();
    const annotationResults = operationResult.annotationResults[0];

    const shotChanges = processShotChanges(annotationResults);
    const labels = processLabels(annotationResults);

    const lastShot = (annotationResults.shotAnnotations || []).slice(-1)[0];
    const totalDuration = lastShot
      ? parseTimestamp(lastShot.endTimeOffset)
      : 0;

    const insights = buildContentInsights(shotChanges, labels, totalDuration);

    const output = {
      metadata: {
        source: uri || path.basename(filePath || ''),
        duration: `${totalDuration.toFixed(1)}s`,
        analyzedAt: new Date().toISOString(),
        features: ['LABEL_DETECTION', 'SHOT_CHANGE_DETECTION'],
      },
      shotChanges,
      labels: {
        videoLevel: labels.videoLabels,
        byShot: labels.shotLabels.slice(0, 30),
        byFrame: labels.frameLabels,
      },
      contentInsights: insights,
    };

    if (jsonStdout) {
      process.stdout.write(JSON.stringify(output));
      return;
    }

    // Legacy CLI mode: save to outputs/
    const outputName = name || path.basename(filePath, path.extname(filePath));
    const date = new Date().toISOString().split('T')[0];
    const outputPath = path.join(__dirname, 'outputs', `${date}-${outputName}.json`);
    fs.mkdirSync(path.dirname(outputPath), { recursive: true });
    fs.writeFileSync(outputPath, JSON.stringify(output, null, 2));
    console.log(`\nSaved: ${outputPath}`);
  } catch (error) {
    if (error.code === 7) {
      console.error('ERROR: Billing not enabled or quota exceeded.');
    } else if (error.code === 3) {
      console.error('ERROR: Unsupported video format or file too large.');
    } else {
      console.error('API error:', error.message, '(code', error.code, ')');
    }
    process.exit(1);
  }
}

analyze();
```

- [ ] **Step 3: Install deps inside api/tools/**

```bash
cd /workspace/api/tools && npm install
```

- [ ] **Step 4: Smoke test with --uri flag (requires real GCS object)**

If you have a small video already in the bucket (or copy one): `gcloud storage cp some-tiny.mp4 gs://$GCS_BUCKET/test.mp4`

```bash
cd /workspace/api/tools
export GOOGLE_APPLICATION_CREDENTIALS=/workspace/gen-lang-client-*.json
node analyze-video.js --uri gs://$GCS_BUCKET/test.mp4 --json-stdout | jq .metadata
```

Expected: prints `{ source, duration, analyzedAt, features }`. If skipped (no test video), continue — integration test in Task 23 covers this.

- [ ] **Step 5: Commit**

```bash
git add api/tools/analyze-video.js
git commit -m "feat(analyzer): support inputUri + json-stdout mode"
```

---

## Phase 2: Backend foundations

### Task 7: Config loader (env vars) — TDD

**Files:**
- Create: `/workspace/api/internal/config/config.go`
- Create: `/workspace/api/internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

`/workspace/api/internal/config/config_test.go`:

```go
package config

import (
	"testing"
)

func TestLoad_AllRequiredSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "9000")
	t.Setenv("ALLOWED_ORIGINS", "http://a,http://b")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", "Zm9v") // "foo" b64
	t.Setenv("GCS_BUCKET", "buck")
	t.Setenv("GCP_PROJECT_ID", "proj")
	t.Setenv("ANTHROPIC_API_KEY", "sk-x")
	t.Setenv("ANTHROPIC_MODEL", "claude-sonnet-4-6")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port: got %q want 9000", cfg.Port)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins: got %v want 2", cfg.AllowedOrigins)
	}
	if cfg.GCSBucket != "buck" {
		t.Errorf("GCSBucket: got %q", cfg.GCSBucket)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// Only set a couple, leave the rest empty
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PORT", "")
	t.Setenv("ALLOWED_ORIGINS", "http://a")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", "Zm9v")
	t.Setenv("GCS_BUCKET", "buck")
	t.Setenv("GCP_PROJECT_ID", "proj")
	t.Setenv("ANTHROPIC_API_KEY", "sk-x")
	t.Setenv("ANTHROPIC_MODEL", "claude-sonnet-4-6")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default Port: got %q want 8080", cfg.Port)
	}
}
```

- [ ] **Step 2: Run test, verify failure**

```bash
cd /workspace/api && go test ./internal/config/...
```

Expected: build failure — `config.Load` undefined.

- [ ] **Step 3: Implement config.go**

`/workspace/api/internal/config/config.go`:

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL                    string
	Port                           string
	AllowedOrigins                 []string
	GoogleAppCredsJSON             string // base64-encoded
	GCSBucket                      string
	GCPProjectID                   string
	AnthropicAPIKey                string
	AnthropicModel                 string
}

func Load() (*Config, error) {
	required := []struct {
		key string
		dst *string
	}{
		{"DATABASE_URL", new(string)},
		{"ALLOWED_ORIGINS", new(string)},
		{"GOOGLE_APPLICATION_CREDENTIALS_JSON", new(string)},
		{"GCS_BUCKET", new(string)},
		{"GCP_PROJECT_ID", new(string)},
		{"ANTHROPIC_API_KEY", new(string)},
		{"ANTHROPIC_MODEL", new(string)},
	}
	var missing []string
	for _, r := range required {
		v := os.Getenv(r.key)
		if v == "" {
			missing = append(missing, r.key)
		}
		*r.dst = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Port:               firstNonEmpty(os.Getenv("PORT"), "8080"),
		AllowedOrigins:     splitAndTrim(os.Getenv("ALLOWED_ORIGINS"), ","),
		GoogleAppCredsJSON: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"),
		GCSBucket:          os.Getenv("GCS_BUCKET"),
		GCPProjectID:       os.Getenv("GCP_PROJECT_ID"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:     os.Getenv("ANTHROPIC_MODEL"),
	}
	if len(cfg.AllowedOrigins) == 0 {
		return nil, errors.New("ALLOWED_ORIGINS must have at least one origin")
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

- [ ] **Step 4: Run test, verify pass**

```bash
cd /workspace/api && go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/config/
git commit -m "feat(config): env var loader with required-vars validation"
```

---

### Task 8: GCP credentials bootstrap (decode b64 → /tmp/gcp-sa.json)

**Files:**
- Create: `/workspace/api/internal/gcpauth/gcpauth.go`
- Create: `/workspace/api/internal/gcpauth/gcpauth_test.go`

- [ ] **Step 1: Write failing test**

`/workspace/api/internal/gcpauth/gcpauth_test.go`:

```go
package gcpauth

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestWriteCredsFile(t *testing.T) {
	want := []byte(`{"type":"service_account"}`)
	b64 := base64.StdEncoding.EncodeToString(want)

	tmp := t.TempDir() + "/sa.json"
	if err := WriteCredsFile(b64, tmp); err != nil {
		t.Fatalf("WriteCredsFile: %v", err)
	}
	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("contents mismatch: got %q want %q", got, want)
	}
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != tmp {
		t.Errorf("GOOGLE_APPLICATION_CREDENTIALS not set to %s", tmp)
	}
}

func TestWriteCredsFile_BadBase64(t *testing.T) {
	if err := WriteCredsFile("not-base64!!!", t.TempDir()+"/sa.json"); err == nil {
		t.Fatal("expected error on invalid base64")
	}
}
```

- [ ] **Step 2: Run, verify fail**

```bash
cd /workspace/api && go test ./internal/gcpauth/...
```

Expected: build failure.

- [ ] **Step 3: Implement**

`/workspace/api/internal/gcpauth/gcpauth.go`:

```go
// Package gcpauth decodes the base64-encoded service account JSON from the
// GOOGLE_APPLICATION_CREDENTIALS_JSON env var and writes it to disk so the
// GCP SDKs can pick it up via GOOGLE_APPLICATION_CREDENTIALS.
package gcpauth

import (
	"encoding/base64"
	"fmt"
	"os"
)

// WriteCredsFile decodes b64 into outPath and sets
// GOOGLE_APPLICATION_CREDENTIALS=outPath in the current process env.
func WriteCredsFile(b64, outPath string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("decode credentials base64: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("write credentials file: %w", err)
	}
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", outPath); err != nil {
		return fmt.Errorf("setenv GOOGLE_APPLICATION_CREDENTIALS: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run, verify pass**

```bash
cd /workspace/api && go test ./internal/gcpauth/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/gcpauth/
git commit -m "feat(gcpauth): decode credentials b64 to disk for SDK pickup"
```

---

### Task 9: DB pool + init.sql + analyses CRUD (TDD with real Postgres)

**Files:**
- Create: `/workspace/api/internal/db/init.sql`
- Create: `/workspace/api/internal/db/db.go`
- Create: `/workspace/api/internal/db/analyses.go`
- Create: `/workspace/api/internal/db/analyses_test.go`
- Create: `/workspace/api/internal/db/testdb_test.go` (test helper)
- Create: `/workspace/api/internal/models/analysis.go`

- [ ] **Step 1: Write the schema file**

`/workspace/api/internal/db/init.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS analyses (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  status           TEXT NOT NULL DEFAULT 'processing'
                     CHECK (status IN ('processing', 'done', 'error')),
  mode             TEXT NOT NULL
                     CHECK (mode IN ('pre_post', 'reference', 'post_mortem')),

  gcs_uri          TEXT NOT NULL,
  original_name    TEXT,

  business_context JSONB NOT NULL,
  metrics_input    JSONB,

  gvi_result       JSONB,
  claude_result    JSONB,

  progress_msg     TEXT,
  error_msg        TEXT,

  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_analyses_created_at
  ON analyses (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_analyses_status_updated
  ON analyses (status, updated_at)
  WHERE status = 'processing';
```

- [ ] **Step 2: Write Analysis model**

`/workspace/api/internal/models/analysis.go`:

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusError      Status = "error"
)

type Mode string

const (
	ModePrePost    Mode = "pre_post"
	ModeReference  Mode = "reference"
	ModePostMortem Mode = "post_mortem"
)

type BusinessContext struct {
	BrandName       string   `json:"brand_name"`
	Description     string   `json:"description"`
	TargetAudience  string   `json:"target_audience"`
	Platforms       []string `json:"platforms"`
	MainPain        string   `json:"main_pain"`
	ContentHistory  string   `json:"content_history"`
}

type Metrics struct {
	Views            *int     `json:"views,omitempty"`
	AvgWatchTime     *float64 `json:"avg_watch_time,omitempty"`
	CompletionRate   *float64 `json:"completion_rate,omitempty"`
	FollowersGained  *int     `json:"followers_gained,omitempty"`
}

type Analysis struct {
	ID              uuid.UUID        `json:"id"`
	Status          Status           `json:"status"`
	Mode            Mode             `json:"mode"`
	GCSURI          string           `json:"gcs_uri"`
	OriginalName    string           `json:"original_name,omitempty"`
	BusinessContext BusinessContext  `json:"business_context"`
	MetricsInput    *Metrics         `json:"metrics_input,omitempty"`
	GVIResult       json.RawMessage  `json:"gvi_result,omitempty"`
	ClaudeResult    json.RawMessage  `json:"claude_result,omitempty"`
	ProgressMsg     string           `json:"progress_msg,omitempty"`
	ErrorMsg        string           `json:"error_msg,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
}
```

- [ ] **Step 3: Write db.go (pool + Init from embed)**

`/workspace/api/internal/db/db.go`:

```go
package db

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed init.sql
var initSQL string

type DB struct {
	*pgxpool.Pool
}

func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Init(ctx context.Context) error {
	_, err := d.Exec(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("apply init.sql: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Write test helper that opens a real DB connection**

`/workspace/api/internal/db/testdb_test.go`:

```go
package db

import (
	"context"
	"os"
	"testing"
)

// testDB returns a DB connected to TEST_DATABASE_URL (or DATABASE_URL).
// Skips the test if neither is set so the suite is safe in offline envs.
func testDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("set TEST_DATABASE_URL or DATABASE_URL to run db tests")
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() {
		// truncate for isolation between tests
		_, _ = db.Exec(ctx, "TRUNCATE analyses")
		db.Close()
	})
	return db
}
```

- [ ] **Step 5: Write failing CRUD tests**

`/workspace/api/internal/db/analyses_test.go`:

```go
package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func TestInsertAndGet(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	a := &models.Analysis{
		Status:       models.StatusProcessing,
		Mode:         models.ModePrePost,
		GCSURI:       "gs://video-analyzer-tmp/test.mp4",
		OriginalName: "test.mp4",
		BusinessContext: models.BusinessContext{
			BrandName:      "X",
			Description:    "Y",
			TargetAudience: "Z",
			Platforms:      []string{"tiktok"},
			MainPain:       "P",
			ContentHistory: "H",
		},
	}
	if err := Insert(ctx, d, a); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if a.ID.String() == "" {
		t.Fatal("ID not populated")
	}

	got, err := Get(ctx, d, a.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GCSURI != a.GCSURI {
		t.Errorf("GCSURI: got %q want %q", got.GCSURI, a.GCSURI)
	}
	if got.BusinessContext.BrandName != "X" {
		t.Errorf("BusinessContext.BrandName: got %q", got.BusinessContext.BrandName)
	}
}

func TestUpdateProgress(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	if err := Insert(ctx, d, a); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := UpdateProgress(ctx, d, a.ID, "Step 2..."); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.ProgressMsg != "Step 2..." {
		t.Errorf("ProgressMsg: %q", got.ProgressMsg)
	}
}

func TestUpdateError(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	if err := SetError(ctx, d, a.ID, "boom"); err != nil {
		t.Fatalf("SetError: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.Status != models.StatusError {
		t.Errorf("Status: got %q want error", got.Status)
	}
	if got.ErrorMsg != "boom" {
		t.Errorf("ErrorMsg: %q", got.ErrorMsg)
	}
}

func TestMarkDone(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	gvi := json.RawMessage(`{"x":1}`)
	claude := json.RawMessage(`{"verdict":"ok"}`)
	if err := SetGVI(ctx, d, a.ID, gvi); err != nil {
		t.Fatalf("SetGVI: %v", err)
	}
	if err := MarkDone(ctx, d, a.ID, claude); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	got, _ := Get(ctx, d, a.ID)
	if got.Status != models.StatusDone {
		t.Errorf("Status: %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt not set")
	}
}

func TestList(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = Insert(ctx, d, minimalAnalysis())
	}
	list, err := List(ctx, d, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len: %d want 3", len(list))
	}
}

func TestStaleProcessing(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	a := minimalAnalysis()
	_ = Insert(ctx, d, a)
	// Force updated_at into the past
	_, _ = d.Exec(ctx,
		"UPDATE analyses SET updated_at = now() - interval '10 minutes' WHERE id = $1",
		a.ID)

	ids, err := StaleProcessing(ctx, d, 8)
	if err != nil {
		t.Fatalf("StaleProcessing: %v", err)
	}
	if len(ids) != 1 || ids[0] != a.ID {
		t.Errorf("StaleProcessing: %v", ids)
	}
}

func minimalAnalysis() *models.Analysis {
	return &models.Analysis{
		Status: models.StatusProcessing,
		Mode:   models.ModePrePost,
		GCSURI: "gs://b/x.mp4",
		BusinessContext: models.BusinessContext{
			BrandName: "x", Description: "x", TargetAudience: "x",
			MainPain: "x", ContentHistory: "x",
			Platforms: []string{"tiktok"},
		},
	}
}
```

- [ ] **Step 6: Run, verify fail**

```bash
cd /workspace/api && go test ./internal/db/...
```

Expected: build failure (functions undefined).

- [ ] **Step 7: Implement analyses.go**

`/workspace/api/internal/db/analyses.go`:

```go
package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func Insert(ctx context.Context, d *DB, a *models.Analysis) error {
	bc, err := json.Marshal(a.BusinessContext)
	if err != nil {
		return fmt.Errorf("marshal business_context: %w", err)
	}
	var mi []byte
	if a.MetricsInput != nil {
		mi, err = json.Marshal(a.MetricsInput)
		if err != nil {
			return fmt.Errorf("marshal metrics_input: %w", err)
		}
	}
	row := d.QueryRow(ctx, `
		INSERT INTO analyses (status, mode, gcs_uri, original_name, business_context, metrics_input)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		a.Status, a.Mode, a.GCSURI, a.OriginalName, bc, mi,
	)
	return row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func Get(ctx context.Context, d *DB, id uuid.UUID) (*models.Analysis, error) {
	row := d.QueryRow(ctx, `
		SELECT id, status, mode, gcs_uri, COALESCE(original_name,''),
		       business_context, metrics_input,
		       gvi_result, claude_result,
		       COALESCE(progress_msg,''), COALESCE(error_msg,''),
		       created_at, updated_at, completed_at
		FROM analyses WHERE id = $1`, id)

	var a models.Analysis
	var bc []byte
	var mi []byte
	if err := row.Scan(
		&a.ID, &a.Status, &a.Mode, &a.GCSURI, &a.OriginalName,
		&bc, &mi,
		&a.GVIResult, &a.ClaudeResult,
		&a.ProgressMsg, &a.ErrorMsg,
		&a.CreatedAt, &a.UpdatedAt, &a.CompletedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bc, &a.BusinessContext); err != nil {
		return nil, fmt.Errorf("unmarshal business_context: %w", err)
	}
	if len(mi) > 0 {
		a.MetricsInput = &models.Metrics{}
		if err := json.Unmarshal(mi, a.MetricsInput); err != nil {
			return nil, fmt.Errorf("unmarshal metrics_input: %w", err)
		}
	}
	return &a, nil
}

func UpdateProgress(ctx context.Context, d *DB, id uuid.UUID, msg string) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET progress_msg = $2, updated_at = now() WHERE id = $1`,
		id, msg)
	return err
}

func SetGVI(ctx context.Context, d *DB, id uuid.UUID, gvi json.RawMessage) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET gvi_result = $2, updated_at = now() WHERE id = $1`,
		id, []byte(gvi))
	return err
}

func MarkDone(ctx context.Context, d *DB, id uuid.UUID, claude json.RawMessage) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses
		SET claude_result = $2, status = 'done', completed_at = now(), updated_at = now(),
		    progress_msg = ''
		WHERE id = $1`,
		id, []byte(claude))
	return err
}

func SetError(ctx context.Context, d *DB, id uuid.UUID, msg string) error {
	_, err := d.Exec(ctx, `
		UPDATE analyses SET status = 'error', error_msg = $2, updated_at = now()
		WHERE id = $1`, id, msg)
	return err
}

type ListItem struct {
	ID           uuid.UUID `json:"id"`
	Mode         string    `json:"mode"`
	Status       string    `json:"status"`
	OriginalName string    `json:"original_name,omitempty"`
	Verdict      string    `json:"verdict,omitempty"`
	CreatedAt    string    `json:"created_at"`
}

func List(ctx context.Context, d *DB, limit int) ([]ListItem, error) {
	rows, err := d.Query(ctx, `
		SELECT id, mode, status, COALESCE(original_name,''),
		       COALESCE(claude_result->>'verdict',''),
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM analyses
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var it ListItem
		if err := rows.Scan(&it.ID, &it.Mode, &it.Status, &it.OriginalName, &it.Verdict, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func StaleProcessing(ctx context.Context, d *DB, minutesOld int) ([]uuid.UUID, error) {
	rows, err := d.Query(ctx, fmt.Sprintf(`
		SELECT id FROM analyses
		WHERE status = 'processing'
		  AND updated_at < now() - interval '%d minutes'`, minutesOld))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

- [ ] **Step 8: Run tests against local Postgres**

Ensure Postgres is running (from Task 4):
```bash
docker compose ps | grep postgres
```

```bash
cd /workspace/api
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/video_analyzer?sslmode=disable"
go test ./internal/db/... -count=1
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add api/internal/db/ api/internal/models/
git commit -m "feat(db): analyses schema + CRUD + tests against real postgres"
```

---

## Phase 3: GCS

### Task 10: GCS client (signed URL + delete)

**Files:**
- Create: `/workspace/api/internal/gcs/gcs.go`

Why no unit test here: testing GCS signing requires real GCP credentials or a complex fake. Smoke-tested by Task 12 (handler) + Task 23 (integration).

- [ ] **Step 1: Implement gcs.go**

`/workspace/api/internal/gcs/gcs.go`:

```go
// Package gcs wraps Google Cloud Storage operations needed by the analyzer:
// V4 signed PUT URLs and object deletion.
package gcs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

type Client struct {
	bucket string
	client *storage.Client
}

func New(ctx context.Context, bucket string) (*Client, error) {
	c, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	return &Client{bucket: bucket, client: c}, nil
}

func (c *Client) Close() error { return c.client.Close() }

// SignedPutURL returns a V4 signed URL that allows a PUT upload to objectKey
// using the given Content-Type. Valid for 15 minutes.
func (c *Client) SignedPutURL(objectKey, contentType string) (string, time.Time, error) {
	expires := time.Now().Add(15 * time.Minute)
	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		Expires:     expires,
		ContentType: contentType,
	}
	url, err := c.client.Bucket(c.bucket).SignedURL(objectKey, opts)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("SignedURL: %w", err)
	}
	return url, expires, nil
}

// Delete removes an object. Accepts either a gs://bucket/key URI or a plain key.
func (c *Client) Delete(ctx context.Context, uriOrKey string) error {
	key := uriOrKey
	if strings.HasPrefix(key, "gs://") {
		// strip gs://bucket/
		parts := strings.SplitN(strings.TrimPrefix(key, "gs://"), "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid gs uri: %s", uriOrKey)
		}
		if parts[0] != c.bucket {
			return fmt.Errorf("uri bucket %s does not match client bucket %s", parts[0], c.bucket)
		}
		key = parts[1]
	}
	return c.client.Bucket(c.bucket).Object(key).Delete(ctx)
}

// Bucket returns the bucket name (for building gs:// URIs).
func (c *Client) Bucket() string { return c.bucket }
```

- [ ] **Step 2: Verify it builds**

```bash
cd /workspace/api && go build ./internal/gcs/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/gcs/
git commit -m "feat(gcs): signed v4 PUT URLs + delete"
```

---

## Phase 4: Claude

### Task 11: Copy content-strategy.md into the claude package

**Files:**
- Copy: `/workspace/content-strategy.md` → `/workspace/api/internal/claude/content-strategy.md`

- [ ] **Step 1: Create the directory and copy**

```bash
mkdir -p /workspace/api/internal/claude
cp /workspace/content-strategy.md /workspace/api/internal/claude/content-strategy.md
ls -la /workspace/api/internal/claude/
```

Expected: `content-strategy.md` present.

- [ ] **Step 2: Commit**

```bash
git add api/internal/claude/content-strategy.md
git commit -m "chore(claude): vendor content-strategy.md for go:embed"
```

---

### Task 12: Claude prompt builder + response types (TDD)

**Files:**
- Create: `/workspace/api/internal/claude/types.go`
- Create: `/workspace/api/internal/claude/prompts.go`
- Create: `/workspace/api/internal/claude/prompts_test.go`

- [ ] **Step 1: Write types.go**

`/workspace/api/internal/claude/types.go`:

```go
package claude

import "encoding/json"

// Result is the structured JSON we ask Claude to produce.
// Optional fields use pointers so we can detect absence.
type Result struct {
	HookAnalysis struct {
		Score       int    `json:"score"`
		Why         string `json:"why"`
		Improvement string `json:"improvement"`
	} `json:"hook_analysis"`

	StructureAnalysis struct {
		FrameworkMatch   string   `json:"framework_match"`
		RetentionIssues  []string `json:"retention_issues"`
	} `json:"structure_analysis"`

	VisualAnalysis struct {
		Rhythm          string   `json:"rhythm"`
		FirstFrame      string   `json:"first_frame"`
		DominantLabels  []string `json:"dominant_labels"`
	} `json:"visual_analysis"`

	KeyInsights       []string `json:"key_insights"`
	ActionItems       []string `json:"action_items"`
	ReplicationScript string   `json:"replication_script,omitempty"`
	Verdict           string   `json:"verdict"`
	VerdictReason     string   `json:"verdict_reason"`
}

func (r *Result) AsRaw() (json.RawMessage, error) {
	return json.Marshal(r)
}
```

- [ ] **Step 2: Write failing tests for the prompt builder**

`/workspace/api/internal/claude/prompts_test.go`:

```go
package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func TestSystemPrompt_NotEmpty(t *testing.T) {
	sp := SystemPrompt()
	if len(sp) < 1000 {
		t.Fatalf("system prompt suspiciously short: %d chars", len(sp))
	}
	if !strings.Contains(sp, "Analista") {
		t.Errorf("expected content-strategy.md to be embedded")
	}
}

func TestBuildUserMessage_Pre_Post(t *testing.T) {
	bc := models.BusinessContext{
		BrandName:      "ScrapJobs",
		Description:    "Motor de busca",
		TargetAudience: "Devs",
		Platforms:      []string{"tiktok", "instagram"},
		MainPain:       "Vagas fecham rápido",
		ContentHistory: "Storytelling funciona",
	}
	gvi := json.RawMessage(`{"shotChanges":{"total":5}}`)

	msg := BuildUserMessage(models.ModePrePost, bc, nil, gvi)

	for _, want := range []string{
		"ScrapJobs",
		"Devs",
		"tiktok, instagram",
		"Vagas fecham rápido",
		"Storytelling funciona",
		"pre_post",
		`"shotChanges"`,
		"JSON válido",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected substring %q not found", want)
		}
	}
}

func TestBuildUserMessage_PostMortem_WithMetrics(t *testing.T) {
	bc := models.BusinessContext{BrandName: "X"}
	views := 100
	completion := 0.42
	metrics := &models.Metrics{Views: &views, CompletionRate: &completion}
	msg := BuildUserMessage(models.ModePostMortem, bc, metrics, json.RawMessage("{}"))

	if !strings.Contains(msg, "100") {
		t.Errorf("expected views in prompt")
	}
	if !strings.Contains(msg, "0.42") {
		t.Errorf("expected completion rate in prompt")
	}
}

func TestParseResult_Happy(t *testing.T) {
	raw := []byte(`{
	  "hook_analysis": {"score": 8, "why": "x", "improvement": "y"},
	  "structure_analysis": {"framework_match": "hook→...", "retention_issues": ["a"]},
	  "visual_analysis": {"rhythm": "fast", "first_frame": "x", "dominant_labels": ["a"]},
	  "key_insights": ["a","b","c"],
	  "action_items": ["a","b"],
	  "verdict": "ok",
	  "verdict_reason": "y"
	}`)

	res, err := ParseResult(raw)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if res.HookAnalysis.Score != 8 {
		t.Errorf("Score: %d", res.HookAnalysis.Score)
	}
	if res.Verdict != "ok" {
		t.Errorf("Verdict: %q", res.Verdict)
	}
}

func TestParseResult_MissingRequired(t *testing.T) {
	raw := []byte(`{"hook_analysis":{"score":1,"why":"","improvement":""}}`)
	_, err := ParseResult(raw)
	if err == nil {
		t.Fatal("expected error on missing required fields")
	}
}
```

- [ ] **Step 3: Run, expect failure**

```bash
cd /workspace/api && go test ./internal/claude/...
```

Expected: build failure.

- [ ] **Step 4: Implement prompts.go**

`/workspace/api/internal/claude/prompts.go`:

```go
package claude

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/leoklaser/video-analyzer/api/internal/models"
)

//go:embed content-strategy.md
var contentStrategyMD string

func SystemPrompt() string { return contentStrategyMD }

func modeDescription(m models.Mode) string {
	switch m {
	case models.ModePrePost:
		return "Vídeo ainda não postado. Avalie se vale postar como está. Foco: hook nos primeiros 3s, estrutura, ritmo de corte, CTA. Pergunta central: 'vale a pena postar?'."
	case models.ModeReference:
		return "Vídeo viral de terceiro pra usar como referência. Foco: por que viralizou + como replicar no contexto do usuário. Inclua um replication_script com roteiro adaptado."
	case models.ModePostMortem:
		return "Vídeo já postado. Diagnóstico do que funcionou ou não. Compare métricas com benchmarks (Caso 4 do system prompt). Foco: aprendizado pra próximos."
	}
	return string(m)
}

func formatMetrics(m *models.Metrics) string {
	if m == nil {
		return "(não fornecidas)"
	}
	var b strings.Builder
	if m.Views != nil {
		fmt.Fprintf(&b, "- Views: %d\n", *m.Views)
	}
	if m.AvgWatchTime != nil {
		fmt.Fprintf(&b, "- Avg watch time (s): %g\n", *m.AvgWatchTime)
	}
	if m.CompletionRate != nil {
		fmt.Fprintf(&b, "- Completion rate: %g\n", *m.CompletionRate)
	}
	if m.FollowersGained != nil {
		fmt.Fprintf(&b, "- Followers ganhos: %d\n", *m.FollowersGained)
	}
	if b.Len() == 0 {
		return "(não fornecidas)"
	}
	return b.String()
}

const outputSchemaInstruction = `Responda APENAS em JSON válido, sem texto antes ou depois, sem markdown.
Estrutura obrigatória (campos com nomes EXATOS):
{
  "hook_analysis": { "score": 1-10, "why": "...", "improvement": "..." },
  "structure_analysis": { "framework_match": "...", "retention_issues": ["..."] },
  "visual_analysis": { "rhythm": "...", "first_frame": "...", "dominant_labels": ["..."] },
  "key_insights": ["...", "...", "..."],
  "action_items": ["...", "..."],
  "replication_script": "...",          // apenas quando mode = "reference"
  "verdict": "vai bombar" | "ok" | "vai flopar",
  "verdict_reason": "..."
}`

func BuildUserMessage(mode models.Mode, bc models.BusinessContext, metrics *models.Metrics, gvi json.RawMessage) string {
	platforms := strings.Join(bc.Platforms, ", ")
	if platforms == "" {
		platforms = "(não informadas)"
	}
	return fmt.Sprintf(`## Contexto do negócio
- Marca: %s
- O que faz: %s
- Público-alvo: %s
- Plataformas: %s
- Dor do cliente: %s
- O que já funcionou (histórico): %s

## Modo de análise
%s (slug: %s)

## Métricas
%s

## Dados extraídos do vídeo (Google Video Intelligence)
%s

---

%s`,
		bc.BrandName,
		bc.Description,
		bc.TargetAudience,
		platforms,
		bc.MainPain,
		bc.ContentHistory,
		modeDescription(mode),
		string(mode),
		formatMetrics(metrics),
		string(gvi),
		outputSchemaInstruction,
	)
}

func ParseResult(raw []byte) (*Result, error) {
	var r Result
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unmarshal claude json: %w", err)
	}
	// minimal required-field validation
	if r.Verdict == "" {
		return nil, fmt.Errorf("missing required field 'verdict'")
	}
	if r.HookAnalysis.Why == "" {
		return nil, fmt.Errorf("missing required field 'hook_analysis.why'")
	}
	if len(r.KeyInsights) == 0 {
		return nil, fmt.Errorf("missing required field 'key_insights'")
	}
	if len(r.ActionItems) == 0 {
		return nil, fmt.Errorf("missing required field 'action_items'")
	}
	return &r, nil
}
```

- [ ] **Step 5: Run, verify PASS**

```bash
cd /workspace/api && go test ./internal/claude/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/claude/
git commit -m "feat(claude): prompt builder + result types + parsing"
```

---

### Task 13: Claude HTTP client

**Files:**
- Create: `/workspace/api/internal/claude/client.go`
- Create: `/workspace/api/internal/claude/client_test.go`

- [ ] **Step 1: Write failing test using httptest**

`/workspace/api/internal/claude/client_test.go`:

```go
package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Analyze_Happy(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sk-test" {
			t.Errorf("x-api-key: got %q want sk-test", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version: got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"content": [{"type":"text","text":"{\"hook_analysis\":{\"score\":7,\"why\":\"w\",\"improvement\":\"i\"},\"structure_analysis\":{\"framework_match\":\"x\",\"retention_issues\":[]},\"visual_analysis\":{\"rhythm\":\"fast\",\"first_frame\":\"y\",\"dominant_labels\":[]},\"key_insights\":[\"a\"],\"action_items\":[\"a\"],\"verdict\":\"ok\",\"verdict_reason\":\"r\"}"}]
		}`))
	}))
	defer server.Close()

	c := NewClient("sk-test", "claude-sonnet-4-6", server.URL)
	raw, err := c.Analyze(context.Background(), "system text", "user text")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !strings.Contains(capturedBody, `"system":"system text"`) {
		t.Errorf("request body missing system: %s", capturedBody)
	}

	res, err := ParseResult(raw)
	if err != nil {
		t.Fatalf("ParseResult: %v", err)
	}
	if res.Verdict != "ok" {
		t.Errorf("Verdict: %q", res.Verdict)
	}
}

func TestClient_Analyze_RetriesOnceOn5xx(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"verdict\":\"ok\",\"verdict_reason\":\"\",\"hook_analysis\":{\"score\":1,\"why\":\"w\",\"improvement\":\"\"},\"structure_analysis\":{\"framework_match\":\"\",\"retention_issues\":[]},\"visual_analysis\":{\"rhythm\":\"\",\"first_frame\":\"\",\"dominant_labels\":[]},\"key_insights\":[\"a\"],\"action_items\":[\"a\"]}"}]}`))
	}))
	defer server.Close()

	c := NewClient("sk", "m", server.URL)
	c.retryDelay = 0
	if _, err := c.Analyze(context.Background(), "s", "u"); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls: %d want 2", calls)
	}
}

func TestClient_Analyze_GivesUpAfterRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	c := NewClient("sk", "m", server.URL)
	c.retryDelay = 0
	if _, err := c.Analyze(context.Background(), "s", "u"); err == nil {
		t.Fatal("expected error after retry exhausted")
	}
}

// Sanity: parsed text content must round-trip JSON.
func TestExtractText(t *testing.T) {
	var resp anthropicResponse
	_ = json.Unmarshal([]byte(`{"content":[{"type":"text","text":"hello"}]}`), &resp)
	got := extractText(resp)
	if got != "hello" {
		t.Errorf("extractText: %q", got)
	}
}
```

- [ ] **Step 2: Run, expect failure**

```bash
cd /workspace/api && go test ./internal/claude/...
```

Expected: build failure.

- [ ] **Step 3: Implement client.go**

`/workspace/api/internal/claude/client.go`:

```go
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultEndpoint = "https://api.anthropic.com/v1/messages"

type Client struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
	retryDelay time.Duration
}

func NewClient(apiKey, model, endpoint string) *Client {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Client{
		apiKey:     apiKey,
		model:      model,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 90 * time.Second},
		retryDelay: 5 * time.Second,
	}
}

type anthropicRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    string        `json:"system"`
	Messages  []reqMessage  `json:"messages"`
}

type reqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Analyze sends system + user prompt, returns raw JSON text from Claude.
// Retries once on 5xx after retryDelay.
func (c *Client) Analyze(ctx context.Context, system, user string) ([]byte, error) {
	body, _ := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    system,
		Messages:  []reqMessage{{Role: "user", Content: user}},
	})

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}
		req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http: %w", err)
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("claude %d: %s", resp.StatusCode, truncate(string(raw), 200))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("claude %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}

		var aresp anthropicResponse
		if err := json.Unmarshal(raw, &aresp); err != nil {
			return nil, fmt.Errorf("unmarshal claude resp: %w (body: %s)", err, truncate(string(raw), 200))
		}
		if aresp.Error != nil {
			return nil, fmt.Errorf("claude error: %s — %s", aresp.Error.Type, aresp.Error.Message)
		}
		text := extractText(aresp)
		text = stripJSONFence(text)
		return []byte(text), nil
	}
	return nil, lastErr
}

func extractText(r anthropicResponse) string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// stripJSONFence removes ```json ... ``` wrappers if Claude adds them despite instructions.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

- [ ] **Step 4: Run, verify PASS**

```bash
cd /workspace/api && go test ./internal/claude/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/claude/client.go api/internal/claude/client_test.go
git commit -m "feat(claude): http client with retry-once on 5xx"
```

---

## Phase 5: Analyzer subprocess

### Task 14: Analyzer wrapper (exec.Command)

**Files:**
- Create: `/workspace/api/internal/analyzer/analyzer.go`

- [ ] **Step 1: Implement analyzer.go**

`/workspace/api/internal/analyzer/analyzer.go`:

```go
// Package analyzer runs the analyze-video.js subprocess against a GCS URI and
// returns the structured JSON output.
package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type Runner struct {
	ScriptPath string        // absolute path to analyze-video.js
	NodeBin    string        // "node" by default
	Timeout    time.Duration // hard timeout for the subprocess
}

func New(scriptPath string) *Runner {
	return &Runner{
		ScriptPath: scriptPath,
		NodeBin:    "node",
		Timeout:    4 * time.Minute,
	}
}

// Run executes `node analyze-video.js --uri <uri> --json-stdout` and returns
// the stdout as raw JSON. Captures stderr separately for error reporting.
func (r *Runner) Run(ctx context.Context, gcsURI string) (json.RawMessage, error) {
	cctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, r.NodeBin, r.ScriptPath, "--uri", gcsURI, "--json-stdout")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("analyze-video.js timeout after %s", r.Timeout)
		}
		return nil, fmt.Errorf("analyze-video.js failed: %w (stderr: %s)", err, truncate(stderr.String(), 500))
	}

	out := stdout.Bytes()
	if !json.Valid(out) {
		return nil, fmt.Errorf("analyze-video.js produced invalid JSON: %s", truncate(string(out), 200))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /workspace/api && go build ./internal/analyzer/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/analyzer/
git commit -m "feat(analyzer): exec wrapper for analyze-video.js"
```

---

## Phase 6: Job runner & watchdog

### Task 15: Job runner (runAnalysis)

**Files:**
- Create: `/workspace/api/internal/jobs/runner.go`

- [ ] **Step 1: Implement runner.go**

`/workspace/api/internal/jobs/runner.go`:

```go
// Package jobs orchestrates the per-analysis goroutine: analyzer subprocess,
// Claude API call, DB state transitions, and GCS cleanup.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/leoklaser/video-analyzer/api/internal/analyzer"
	"github.com/leoklaser/video-analyzer/api/internal/claude"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/gcs"
)

type Runner struct {
	DB       *db.DB
	Analyzer *analyzer.Runner
	Claude   *claude.Client
	GCS      *gcs.Client
}

// Run executes the full pipeline for the given analysis id.
// Designed to be called as `go r.Run(ctx, id)` from the handler.
func (r *Runner) Run(ctx context.Context, id uuid.UUID) {
	// Detach from request context: this work outlives the HTTP request.
	jobCtx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()

	a, err := db.Get(jobCtx, r.DB, id)
	if err != nil {
		slog.Error("job: get analysis", "id", id, "err", err)
		return
	}
	gcsURI := a.GCSURI

	defer func() {
		// Cleanup GCS no matter what (success or error).
		if err := r.GCS.Delete(jobCtx, gcsURI); err != nil {
			slog.Warn("job: gcs delete failed (lifecycle rule will catch it)", "uri", gcsURI, "err", err)
		}
	}()

	if err := db.UpdateProgress(jobCtx, r.DB, id, "Analisando estrutura visual..."); err != nil {
		slog.Error("job: update progress", "id", id, "err", err)
		return
	}

	gvi, err := r.Analyzer.Run(jobCtx, gcsURI)
	if err != nil {
		slog.Error("job: analyzer failed", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, friendlyAnalyzerError(err))
		return
	}
	if err := db.SetGVI(jobCtx, r.DB, id, gvi); err != nil {
		slog.Error("job: save gvi", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao salvar dados de análise.")
		return
	}

	if err := db.UpdateProgress(jobCtx, r.DB, id, "Gerando insights com IA..."); err != nil {
		slog.Error("job: update progress 2", "id", id, "err", err)
	}

	user := claude.BuildUserMessage(a.Mode, a.BusinessContext, a.MetricsInput, gvi)
	raw, err := r.Claude.Analyze(jobCtx, claude.SystemPrompt(), user)
	if err != nil {
		slog.Error("job: claude failed", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao gerar insights. Tente novamente.")
		return
	}

	result, err := claude.ParseResult(raw)
	if err != nil {
		// One reinforced retry asking for valid JSON only.
		slog.Warn("job: claude json invalid, retrying", "id", id, "err", err)
		reinforced := user + "\n\nLEMBRETE: responda APENAS em JSON puro, sem markdown, sem texto antes ou depois. Sua resposta anterior foi rejeitada por não ser JSON válido."
		raw, err = r.Claude.Analyze(jobCtx, claude.SystemPrompt(), reinforced)
		if err != nil {
			_ = db.SetError(jobCtx, r.DB, id, "Falha ao gerar insights. Tente novamente.")
			return
		}
		result, err = claude.ParseResult(raw)
		if err != nil {
			_ = db.SetError(jobCtx, r.DB, id, "Resposta da IA inválida após nova tentativa.")
			return
		}
	}

	rawNormalized, _ := result.AsRaw()
	if err := db.MarkDone(jobCtx, r.DB, id, rawNormalized); err != nil {
		slog.Error("job: mark done", "id", id, "err", err)
		_ = db.SetError(jobCtx, r.DB, id, "Falha ao salvar resultado.")
		return
	}
	slog.Info("job: done", "id", id)
}

func friendlyAnalyzerError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Análise demorou demais. Tente um vídeo menor."
	}
	return fmt.Sprintf("Falha ao analisar vídeo: %s", err.Error())
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /workspace/api && go build ./internal/jobs/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/jobs/runner.go
git commit -m "feat(jobs): runAnalysis pipeline with cleanup + retry"
```

---

### Task 16: Watchdog (stale jobs)

**Files:**
- Create: `/workspace/api/internal/jobs/watchdog.go`

- [ ] **Step 1: Implement watchdog.go**

`/workspace/api/internal/jobs/watchdog.go`:

```go
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/db"
)

// StartWatchdog runs every minute marking analyses stuck in 'processing'
// for more than 8 minutes as 'error'. Returns when ctx is canceled.
func StartWatchdog(ctx context.Context, d *db.DB) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			ids, err := db.StaleProcessing(tickCtx, d, 8)
			if err != nil {
				slog.Error("watchdog: query stale", "err", err)
				cancel()
				continue
			}
			for _, id := range ids {
				if err := db.SetError(tickCtx, d, id, "Análise interrompida (timeout)"); err != nil {
					slog.Error("watchdog: mark error", "id", id, "err", err)
				} else {
					slog.Warn("watchdog: marked stale job as error", "id", id)
				}
			}
			cancel()
		}
	}
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /workspace/api && go build ./internal/jobs/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/jobs/watchdog.go
git commit -m "feat(jobs): watchdog for stale processing rows"
```

---

## Phase 7: HTTP handlers + server

### Task 17: Router + middleware + healthz

**Files:**
- Create: `/workspace/api/internal/handlers/router.go`

- [ ] **Step 1: Implement router.go**

`/workspace/api/internal/handlers/router.go`:

```go
// Package handlers wires the chi router, middleware, and HTTP endpoints.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type Deps struct {
	Uploads        *UploadsHandler
	Analyze        *AnalyzeHandler
	Analyses       *AnalysesHandler
	AllowedOrigins []string
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(corsMiddleware(d.AllowedOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/uploads/signed-url", d.Uploads.SignedURL)
		r.Post("/analyze", d.Analyze.Start)
		r.Get("/analyze/{id}", d.Analyze.Get)
		r.Get("/analyses", d.Analyses.List)
	})

	return r
}

func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	allowedSet := map[string]bool{}
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowedSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// writeJSON / writeError are small helpers shared by all handlers in this package.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

(The handler structs `UploadsHandler`, `AnalyzeHandler`, `AnalysesHandler` are defined in Tasks 18–20.)

- [ ] **Step 2: Verify the package compiles in isolation will fail until Task 20** — defer build verification to after Tasks 18-20.

- [ ] **Step 3: Commit**

```bash
git add api/internal/handlers/router.go
git commit -m "feat(handlers): chi router + cors + healthz"
```

---

### Task 18: POST /api/uploads/signed-url handler

**Files:**
- Create: `/workspace/api/internal/handlers/uploads.go`

- [ ] **Step 1: Implement uploads.go**

`/workspace/api/internal/handlers/uploads.go`:

```go
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/gcs"
)

type UploadsHandler struct {
	GCS *gcs.Client
}

type signedURLRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type signedURLResponse struct {
	PutURL    string `json:"put_url"`
	GCSURI    string `json:"gcs_uri"`
	ExpiresAt string `json:"expires_at"`
}

func (h *UploadsHandler) SignedURL(w http.ResponseWriter, r *http.Request) {
	var req signedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename required")
		return
	}
	if req.ContentType == "" {
		req.ContentType = "video/mp4"
	}
	if !strings.HasPrefix(req.ContentType, "video/") {
		writeError(w, http.StatusBadRequest, "content_type must be video/*")
		return
	}

	ext := strings.ToLower(path.Ext(req.Filename))
	if ext == "" {
		ext = ".mp4"
	}
	objectKey := time.Now().UTC().Format("2006-01-02") + "/" + randomHex(16) + ext

	url, expires, err := h.GCS.SignedPutURL(objectKey, req.ContentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign url: "+err.Error())
		return
	}
	gcsURI := "gs://" + h.GCS.Bucket() + "/" + objectKey

	writeJSON(w, http.StatusOK, signedURLResponse{
		PutURL:    url,
		GCSURI:    gcsURI,
		ExpiresAt: expires.UTC().Format(time.RFC3339),
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/handlers/uploads.go
git commit -m "feat(handlers): POST /api/uploads/signed-url"
```

---

### Task 19: POST /api/analyze + GET /api/analyze/:id

**Files:**
- Create: `/workspace/api/internal/handlers/analyze.go`
- Create: `/workspace/api/internal/handlers/analyze_test.go`

- [ ] **Step 1: Write tests for payload validation (table-driven)**

`/workspace/api/internal/handlers/analyze_test.go`:

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStart_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", ``, "invalid JSON"},
		{"missing gcs_uri", `{"mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "gcs_uri"},
		{"wrong bucket", `{"gcs_uri":"gs://other-bucket/x.mp4","mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "gcs_uri"},
		{"bad mode", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"weird","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "mode"},
		{"empty business_context.brand_name", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"pre_post","business_context":{"description":"b","target_audience":"c","main_pain":"d","content_history":"e"}}`, "brand_name"},
		{"metrics without post_mortem", `{"gcs_uri":"gs://video-analyzer-tmp/x.mp4","mode":"pre_post","business_context":{"brand_name":"a","description":"b","target_audience":"c","main_pain":"d","content_history":"e"},"metrics":{"views":1}}`, "metrics"},
	}
	h := &AnalyzeHandler{Bucket: "video-analyzer-tmp"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/analyze", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Start(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body missing %q: %s", tc.want, rec.Body.String())
			}
		})
	}
}
```

- [ ] **Step 2: Implement analyze.go (Start + Get)**

`/workspace/api/internal/handlers/analyze.go`:

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/jobs"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

type AnalyzeHandler struct {
	DB     *db.DB
	Bucket string
	Runner *jobs.Runner
}

type startRequest struct {
	GCSURI          string                  `json:"gcs_uri"`
	OriginalName    string                  `json:"original_name"`
	Mode            string                  `json:"mode"`
	BusinessContext models.BusinessContext  `json:"business_context"`
	Metrics         *models.Metrics         `json:"metrics,omitempty"`
}

func (h *AnalyzeHandler) Start(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.validate(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a := &models.Analysis{
		Status:          models.StatusProcessing,
		Mode:            models.Mode(req.Mode),
		GCSURI:          req.GCSURI,
		OriginalName:    req.OriginalName,
		BusinessContext: req.BusinessContext,
		MetricsInput:    req.Metrics,
	}
	if err := db.Insert(r.Context(), h.DB, a); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create analysis: "+err.Error())
		return
	}
	_ = db.UpdateProgress(r.Context(), h.DB, a.ID, "Iniciando análise...")

	if h.Runner != nil {
		go h.Runner.Run(context.Background(), a.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":     a.ID,
		"status": a.Status,
	})
}

func (h *AnalyzeHandler) validate(req *startRequest) error {
	expectedPrefix := "gs://" + h.Bucket + "/"
	if !strings.HasPrefix(req.GCSURI, expectedPrefix) {
		return fmt.Errorf("gcs_uri must start with %s", expectedPrefix)
	}
	switch req.Mode {
	case "pre_post", "reference", "post_mortem":
		// ok
	default:
		return fmt.Errorf("mode must be pre_post|reference|post_mortem (got %q)", req.Mode)
	}
	bc := req.BusinessContext
	missing := []string{}
	if strings.TrimSpace(bc.BrandName) == "" {
		missing = append(missing, "brand_name")
	}
	if strings.TrimSpace(bc.Description) == "" {
		missing = append(missing, "description")
	}
	if strings.TrimSpace(bc.TargetAudience) == "" {
		missing = append(missing, "target_audience")
	}
	if strings.TrimSpace(bc.MainPain) == "" {
		missing = append(missing, "main_pain")
	}
	if strings.TrimSpace(bc.ContentHistory) == "" {
		missing = append(missing, "content_history")
	}
	if len(missing) > 0 {
		return fmt.Errorf("business_context missing: %s", strings.Join(missing, ", "))
	}
	if req.Metrics != nil && req.Mode != "post_mortem" {
		return fmt.Errorf("metrics only allowed when mode = post_mortem")
	}
	return nil
}

func (h *AnalyzeHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	a, err := db.Get(r.Context(), h.DB, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]any{
		"id":           a.ID,
		"status":       a.Status,
		"mode":         a.Mode,
		"progress_msg": a.ProgressMsg,
		"created_at":   a.CreatedAt,
	}
	if a.CompletedAt != nil {
		resp["completed_at"] = a.CompletedAt
	}
	switch a.Status {
	case models.StatusDone:
		resp["result"] = json.RawMessage(a.ClaudeResult)
	case models.StatusError:
		resp["error"] = a.ErrorMsg
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 3: Run validation tests**

```bash
cd /workspace/api && go test ./internal/handlers/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add api/internal/handlers/analyze.go api/internal/handlers/analyze_test.go
git commit -m "feat(handlers): POST /api/analyze + GET /api/analyze/:id"
```

---

### Task 20: GET /api/analyses (history)

**Files:**
- Create: `/workspace/api/internal/handlers/analyses.go`

- [ ] **Step 1: Implement analyses.go**

`/workspace/api/internal/handlers/analyses.go`:

```go
package handlers

import (
	"net/http"

	"github.com/leoklaser/video-analyzer/api/internal/db"
)

type AnalysesHandler struct {
	DB *db.DB
}

func (h *AnalysesHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := db.List(r.Context(), h.DB, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}
```

- [ ] **Step 2: Verify api package builds**

```bash
cd /workspace/api && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/handlers/analyses.go
git commit -m "feat(handlers): GET /api/analyses"
```

---

### Task 21: Wire main.go (load config → start server → start watchdog)

**Files:**
- Modify: `/workspace/api/cmd/server/main.go`

- [ ] **Step 1: Replace main.go**

Overwrite `/workspace/api/cmd/server/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/analyzer"
	"github.com/leoklaser/video-analyzer/api/internal/claude"
	"github.com/leoklaser/video-analyzer/api/internal/config"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/gcpauth"
	"github.com/leoklaser/video-analyzer/api/internal/gcs"
	"github.com/leoklaser/video-analyzer/api/internal/handlers"
	"github.com/leoklaser/video-analyzer/api/internal/jobs"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	// Decode GCP credentials to disk so SDKs find them.
	credsPath := "/tmp/gcp-sa.json"
	if err := gcpauth.WriteCredsFile(cfg.GoogleAppCredsJSON, credsPath); err != nil {
		slog.Error("gcpauth", "err", err)
		os.Exit(1)
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := db.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db open", "err", err)
		os.Exit(1)
	}
	defer d.Close()
	if err := d.Init(rootCtx); err != nil {
		slog.Error("db init", "err", err)
		os.Exit(1)
	}

	gcsClient, err := gcs.New(rootCtx, cfg.GCSBucket)
	if err != nil {
		slog.Error("gcs", "err", err)
		os.Exit(1)
	}
	defer gcsClient.Close()

	// Resolve absolute path to analyze-video.js (sibling of cmd/server, copied
	// by the Dockerfile to /app/tools/analyze-video.js).
	scriptPath := os.Getenv("ANALYZE_VIDEO_SCRIPT")
	if scriptPath == "" {
		// Default for local dev (run from api/).
		abs, _ := filepath.Abs("tools/analyze-video.js")
		scriptPath = abs
	}
	runner := &jobs.Runner{
		DB:       d,
		Analyzer: analyzer.New(scriptPath),
		Claude:   claude.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel, ""),
		GCS:      gcsClient,
	}

	// Watchdog
	go jobs.StartWatchdog(rootCtx, d)

	mux := handlers.NewRouter(handlers.Deps{
		Uploads:        &handlers.UploadsHandler{GCS: gcsClient},
		Analyze:        &handlers.AnalyzeHandler{DB: d, Bucket: cfg.GCSBucket, Runner: runner},
		Analyses:       &handlers.AnalysesHandler{DB: d},
		AllowedOrigins: cfg.AllowedOrigins,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")

	shutdownCtx, c2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer c2()
	_ = srv.Shutdown(shutdownCtx)
	cancel()
}
```

- [ ] **Step 2: Build the binary**

```bash
cd /workspace/api && go build -o /tmp/server ./cmd/server
```

Expected: no errors. Binary at /tmp/server.

- [ ] **Step 3: Smoke test locally**

Set env vars (use a real ANTHROPIC_API_KEY and base64-encoded SA from Task 5):

```bash
cd /workspace/api
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/video_analyzer?sslmode=disable"
export PORT=8080
export ALLOWED_ORIGINS=http://localhost:5173
export GOOGLE_APPLICATION_CREDENTIALS_JSON=$(base64 -w0 /workspace/gen-lang-client-*.json)
export GCS_BUCKET=video-analyzer-tmp
export GCP_PROJECT_ID=gen-lang-client-0123498892
export ANTHROPIC_API_KEY=<your rotated key>
export ANTHROPIC_MODEL=claude-sonnet-4-6

go run ./cmd/server
```

In another terminal:
```bash
curl -s http://localhost:8080/healthz
# expected: ok
curl -s -X POST http://localhost:8080/api/uploads/signed-url \
  -H "Content-Type: application/json" \
  -d '{"filename":"x.mp4","content_type":"video/mp4"}' | jq .
# expected: { put_url, gcs_uri, expires_at }
```

Stop server with Ctrl+C.

- [ ] **Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "feat(api): wire main with config, db init, gcs, runner, watchdog"
```

---

## Phase 8: Backend integration test

### Task 22: End-to-end backend test with real GVI + Claude

**Files:**
- Create: `/workspace/api/internal/jobs/runner_integration_test.go`
- Create: `/workspace/api/testdata/short.mp4` (5-15 second test video — bring your own)

This test is SKIPPED unless `RUN_INTEGRATION=1` is set, since it makes real API calls (~$0.02).

- [ ] **Step 1: Add a tiny test video**

Bring a 5-15s MP4 (under 5MB) and copy to `/workspace/api/testdata/short.mp4`. Any random short clip works. If you don't have one, record a 5s screen capture.

```bash
mkdir -p /workspace/api/testdata
# ... copy your video to /workspace/api/testdata/short.mp4
ls -lh /workspace/api/testdata/short.mp4
```

- [ ] **Step 2: Write the integration test**

`/workspace/api/internal/jobs/runner_integration_test.go`:

```go
package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leoklaser/video-analyzer/api/internal/analyzer"
	"github.com/leoklaser/video-analyzer/api/internal/claude"
	"github.com/leoklaser/video-analyzer/api/internal/db"
	"github.com/leoklaser/video-analyzer/api/internal/gcs"
	"github.com/leoklaser/video-analyzer/api/internal/models"
)

func TestRunner_FullPipeline(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 to run (calls real GVI + Claude)")
	}

	ctx := context.Background()

	d, err := db.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Init(ctx); err != nil {
		t.Fatalf("db init: %v", err)
	}
	_, _ = d.Exec(ctx, "TRUNCATE analyses")

	bucket := os.Getenv("GCS_BUCKET")
	g, err := gcs.New(ctx, bucket)
	if err != nil {
		t.Fatalf("gcs: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	// Upload the test video to GCS using the signed URL flow (mimics frontend).
	key := "tests/short-" + time.Now().Format("20060102-150405") + ".mp4"
	putURL, _, err := g.SignedPutURL(key, "video/mp4")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := httpPUT(t, putURL, "/workspace/api/testdata/short.mp4", "video/mp4"); err != nil {
		t.Fatalf("upload: %v", err)
	}
	gcsURI := "gs://" + bucket + "/" + key

	scriptPath, _ := filepath.Abs("../../tools/analyze-video.js")
	r := &Runner{
		DB:       d,
		Analyzer: analyzer.New(scriptPath),
		Claude:   claude.NewClient(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("ANTHROPIC_MODEL"), ""),
		GCS:      g,
	}

	a := &models.Analysis{
		Status: models.StatusProcessing,
		Mode:   models.ModePrePost,
		GCSURI: gcsURI,
		BusinessContext: models.BusinessContext{
			BrandName:      "TestBrand",
			Description:    "produto x para devs",
			TargetAudience: "devs sr",
			Platforms:      []string{"tiktok"},
			MainPain:       "perdem tempo",
			ContentHistory: "storytelling funciona",
		},
	}
	if err := db.Insert(ctx, d, a); err != nil {
		t.Fatalf("insert: %v", err)
	}

	r.Run(ctx, a.ID) // blocking; watchdog timeout = 7min

	got, err := db.Get(ctx, d, a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != models.StatusDone {
		t.Fatalf("status: %q error: %q", got.Status, got.ErrorMsg)
	}
	if len(got.ClaudeResult) < 50 {
		t.Errorf("ClaudeResult suspiciously small: %s", got.ClaudeResult)
	}

	// Verify GCS object is gone (or being garbage-collected by lifecycle rule)
	if err := g.Delete(ctx, gcsURI); err == nil {
		t.Logf("note: object was still there before our delete — runner should have removed it")
	}
}
```

Append the helper at the bottom of the same file:

```go
func httpPUT(t *testing.T, url, filePath, contentType string) error {
	t.Helper()
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	req, _ := http.NewRequest("PUT", url, f)
	req.Header.Set("Content-Type", contentType)
	st, err := f.Stat()
	if err != nil {
		return err
	}
	req.ContentLength = st.Size()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
```

- [ ] **Step 3: Run the integration test**

```bash
cd /workspace/api
export RUN_INTEGRATION=1
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/video_analyzer?sslmode=disable"
export GOOGLE_APPLICATION_CREDENTIALS=/workspace/gen-lang-client-*.json
export GCS_BUCKET=video-analyzer-tmp
export ANTHROPIC_API_KEY=<your rotated key>
export ANTHROPIC_MODEL=claude-sonnet-4-6

go test -v -timeout 8m ./internal/jobs/... -run TestRunner_FullPipeline
```

Expected: PASS in ~1-3 minutes. Cost: ~$0.02-0.05.

- [ ] **Step 4: Commit**

```bash
git add api/internal/jobs/runner_integration_test.go
git commit -m "test(jobs): end-to-end integration with real GVI + Claude"
```

---

## Phase 9: Frontend — API client + types

### Task 23: TypeScript types + API client

**Files:**
- Create: `/workspace/web/src/types.ts`
- Create: `/workspace/web/src/api.ts`

- [ ] **Step 1: Write types.ts**

`/workspace/web/src/types.ts`:

```ts
export type Mode = "pre_post" | "reference" | "post_mortem";

export type Platform = "tiktok" | "instagram" | "youtube" | "other";

export interface BusinessContext {
  brand_name: string;
  description: string;
  target_audience: string;
  platforms: Platform[];
  main_pain: string;
  content_history: string;
}

export interface Metrics {
  views?: number;
  avg_watch_time?: number;
  completion_rate?: number;
  followers_gained?: number;
}

export interface SignedURLResponse {
  put_url: string;
  gcs_uri: string;
  expires_at: string;
}

export interface StartAnalyzeRequest {
  gcs_uri: string;
  original_name: string;
  mode: Mode;
  business_context: BusinessContext;
  metrics?: Metrics;
}

export interface AnalysisStatus {
  id: string;
  status: "processing" | "done" | "error";
  mode: Mode;
  progress_msg: string;
  created_at: string;
  completed_at?: string;
  result?: ClaudeResult;
  error?: string;
}

export interface ClaudeResult {
  hook_analysis: { score: number; why: string; improvement: string };
  structure_analysis: { framework_match: string; retention_issues: string[] };
  visual_analysis: { rhythm: string; first_frame: string; dominant_labels: string[] };
  key_insights: string[];
  action_items: string[];
  replication_script?: string;
  verdict: "vai bombar" | "ok" | "vai flopar" | string;
  verdict_reason: string;
}

export interface AnalysisListItem {
  id: string;
  mode: Mode;
  status: "processing" | "done" | "error";
  original_name?: string;
  verdict?: string;
  created_at: string;
}
```

- [ ] **Step 2: Write api.ts**

`/workspace/web/src/api.ts`:

```ts
import type {
  AnalysisListItem,
  AnalysisStatus,
  SignedURLResponse,
  StartAnalyzeRequest,
} from "./types";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers || {}),
    },
  });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status} ${path}: ${text}`);
  }
  return r.json();
}

export async function getSignedURL(filename: string, contentType: string): Promise<SignedURLResponse> {
  return req("/api/uploads/signed-url", {
    method: "POST",
    body: JSON.stringify({ filename, content_type: contentType }),
  });
}

/** PUT the file directly to GCS. Resolves on 2xx, calls onProgress with [0,1]. */
export function uploadToGCS(
  putURL: string,
  file: File,
  onProgress?: (pct: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", putURL);
    xhr.setRequestHeader("Content-Type", file.type || "video/mp4");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(e.loaded / e.total);
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`upload failed: ${xhr.status} ${xhr.responseText}`));
    };
    xhr.onerror = () => reject(new Error("upload network error"));
    xhr.send(file);
  });
}

export async function startAnalyze(req_: StartAnalyzeRequest): Promise<{ id: string; status: string }> {
  return req("/api/analyze", {
    method: "POST",
    body: JSON.stringify(req_),
  });
}

export async function getAnalysis(id: string): Promise<AnalysisStatus> {
  return req(`/api/analyze/${id}`);
}

export async function listAnalyses(): Promise<AnalysisListItem[]> {
  return req("/api/analyses");
}
```

- [ ] **Step 3: Type-check**

```bash
cd /workspace/web && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat(web): API client + types"
```

---

## Phase 10: Frontend components (functional first; design polish later)

### Task 24: AnalysisForm component

**Files:**
- Create: `/workspace/web/src/components/AnalysisForm.tsx`

- [ ] **Step 1: Implement AnalysisForm.tsx**

`/workspace/web/src/components/AnalysisForm.tsx`:

```tsx
import { useState } from "react";
import type { BusinessContext, Metrics, Mode, Platform } from "../types";

interface Props {
  disabled?: boolean;
  onSubmit: (data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
  }) => void;
}

const PLATFORMS: Platform[] = ["tiktok", "instagram", "youtube", "other"];

export function AnalysisForm({ disabled, onSubmit }: Props) {
  const [file, setFile] = useState<File | null>(null);
  const [mode, setMode] = useState<Mode>("pre_post");
  const [bc, setBC] = useState<BusinessContext>({
    brand_name: "",
    description: "",
    target_audience: "",
    platforms: ["tiktok"],
    main_pain: "",
    content_history: "",
  });
  const [metrics, setMetrics] = useState<Metrics>({});

  function togglePlatform(p: Platform) {
    setBC((prev) => ({
      ...prev,
      platforms: prev.platforms.includes(p)
        ? prev.platforms.filter((x) => x !== p)
        : [...prev.platforms, p],
    }));
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return alert("Selecione um arquivo MP4");
    const m = mode === "post_mortem" && Object.keys(metrics).length > 0 ? metrics : undefined;
    onSubmit({ file, mode, businessContext: bc, metrics: m });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium mb-1">Vídeo (MP4)</label>
        <input
          type="file"
          accept="video/mp4,video/*"
          onChange={(e) => setFile(e.target.files?.[0] || null)}
          disabled={disabled}
          className="block w-full text-sm"
          required
        />
      </div>

      <div>
        <label className="block text-sm font-medium mb-1">Modo</label>
        <select
          value={mode}
          onChange={(e) => setMode(e.target.value as Mode)}
          disabled={disabled}
          className="block w-full rounded border border-zinc-700 bg-zinc-900 p-2"
        >
          <option value="pre_post">Pré-postagem</option>
          <option value="reference">Referência (vídeo viral de terceiro)</option>
          <option value="post_mortem">Post-mortem (já postado)</option>
        </select>
      </div>

      <fieldset className="space-y-2 border border-zinc-700 rounded p-3">
        <legend className="text-sm font-medium px-1">Contexto do negócio</legend>
        <Text label="Marca" value={bc.brand_name} onChange={(v) => setBC({ ...bc, brand_name: v })} disabled={disabled} />
        <Text label="O que faz" value={bc.description} onChange={(v) => setBC({ ...bc, description: v })} disabled={disabled} />
        <Text label="Público-alvo" value={bc.target_audience} onChange={(v) => setBC({ ...bc, target_audience: v })} disabled={disabled} />
        <div>
          <label className="block text-sm mb-1">Plataformas</label>
          <div className="flex gap-2 flex-wrap">
            {PLATFORMS.map((p) => (
              <label key={p} className="text-sm inline-flex items-center gap-1">
                <input
                  type="checkbox"
                  checked={bc.platforms.includes(p)}
                  onChange={() => togglePlatform(p)}
                  disabled={disabled}
                />
                {p}
              </label>
            ))}
          </div>
        </div>
        <Text label="Dor do cliente" value={bc.main_pain} onChange={(v) => setBC({ ...bc, main_pain: v })} disabled={disabled} />
        <Text label="O que já funcionou" value={bc.content_history} onChange={(v) => setBC({ ...bc, content_history: v })} disabled={disabled} />
      </fieldset>

      {mode === "post_mortem" && (
        <fieldset className="space-y-2 border border-zinc-700 rounded p-3">
          <legend className="text-sm font-medium px-1">Métricas (opcional)</legend>
          <Num label="Views" value={metrics.views} onChange={(v) => setMetrics({ ...metrics, views: v })} disabled={disabled} />
          <Num label="Avg watch time (s)" value={metrics.avg_watch_time} onChange={(v) => setMetrics({ ...metrics, avg_watch_time: v })} disabled={disabled} />
          <Num label="Completion rate (0-1)" step={0.01} value={metrics.completion_rate} onChange={(v) => setMetrics({ ...metrics, completion_rate: v })} disabled={disabled} />
          <Num label="Followers ganhos" value={metrics.followers_gained} onChange={(v) => setMetrics({ ...metrics, followers_gained: v })} disabled={disabled} />
        </fieldset>
      )}

      <button
        type="submit"
        disabled={disabled}
        className="w-full py-2 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 disabled:cursor-not-allowed font-medium"
      >
        Analisar
      </button>
    </form>
  );
}

function Text({ label, value, onChange, disabled }: { label: string; value: string; onChange: (v: string) => void; disabled?: boolean }) {
  return (
    <div>
      <label className="block text-sm mb-1">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="block w-full rounded border border-zinc-700 bg-zinc-900 p-2 text-sm"
        required
      />
    </div>
  );
}

function Num({ label, value, onChange, step = 1, disabled }: { label: string; value?: number; onChange: (v: number | undefined) => void; step?: number; disabled?: boolean }) {
  return (
    <div>
      <label className="block text-sm mb-1">{label}</label>
      <input
        type="number"
        step={step}
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
        disabled={disabled}
        className="block w-full rounded border border-zinc-700 bg-zinc-900 p-2 text-sm"
      />
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/AnalysisForm.tsx
git commit -m "feat(web): AnalysisForm component"
```

---

### Task 25: AnalysisRunning + AnalysisResult components

**Files:**
- Create: `/workspace/web/src/components/AnalysisRunning.tsx`
- Create: `/workspace/web/src/components/AnalysisResult.tsx`

- [ ] **Step 1: Implement AnalysisRunning.tsx**

`/workspace/web/src/components/AnalysisRunning.tsx`:

```tsx
interface Props {
  progressMsg: string;
  uploadPct?: number;  // 0..1 when uploading
}

export function AnalysisRunning({ progressMsg, uploadPct }: Props) {
  return (
    <div className="rounded border border-zinc-700 p-6 text-center space-y-3">
      <div className="text-3xl">⏳</div>
      <div className="text-sm font-medium">{progressMsg || "Trabalhando..."}</div>
      {typeof uploadPct === "number" && (
        <div className="w-full bg-zinc-800 rounded-full h-2 overflow-hidden">
          <div
            className="bg-emerald-600 h-full transition-all"
            style={{ width: `${Math.round(uploadPct * 100)}%` }}
          />
        </div>
      )}
      <p className="text-xs text-zinc-500">
        Pode levar de 1 a 3 minutos. Pode atualizar a página — o resultado fica no histórico.
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Implement AnalysisResult.tsx**

`/workspace/web/src/components/AnalysisResult.tsx`:

```tsx
import type { ClaudeResult } from "../types";

interface Props {
  result: ClaudeResult;
}

const VERDICT_COLORS: Record<string, string> = {
  "vai bombar": "bg-emerald-600 text-white",
  "ok": "bg-amber-500 text-zinc-900",
  "vai flopar": "bg-rose-600 text-white",
};

export function AnalysisResult({ result }: Props) {
  const verdictClass = VERDICT_COLORS[result.verdict] || "bg-zinc-600 text-white";

  return (
    <div className="space-y-4">
      <div className={`rounded p-4 ${verdictClass}`}>
        <div className="text-xs uppercase tracking-wider opacity-80">Veredito</div>
        <div className="text-2xl font-bold">{result.verdict}</div>
        <div className="text-sm mt-1">{result.verdict_reason}</div>
      </div>

      <Section title="Hook">
        <div className="flex items-baseline gap-3 mb-2">
          <div className="text-3xl font-bold">{result.hook_analysis.score}/10</div>
        </div>
        <p className="text-sm mb-2"><b>Por quê:</b> {result.hook_analysis.why}</p>
        <p className="text-sm"><b>O que melhorar:</b> {result.hook_analysis.improvement}</p>
      </Section>

      <Section title="Estrutura">
        <p className="text-sm mb-2"><b>Match com framework:</b> {result.structure_analysis.framework_match}</p>
        {result.structure_analysis.retention_issues.length > 0 && (
          <ul className="list-disc list-inside text-sm space-y-1">
            {result.structure_analysis.retention_issues.map((x, i) => <li key={i}>{x}</li>)}
          </ul>
        )}
      </Section>

      <Section title="Análise visual">
        <p className="text-sm"><b>Ritmo:</b> {result.visual_analysis.rhythm}</p>
        <p className="text-sm"><b>Primeiro frame:</b> {result.visual_analysis.first_frame}</p>
        <p className="text-sm"><b>Labels dominantes:</b> {result.visual_analysis.dominant_labels.join(", ")}</p>
      </Section>

      <Section title="Insights">
        <ul className="list-disc list-inside text-sm space-y-1">
          {result.key_insights.map((x, i) => <li key={i}>{x}</li>)}
        </ul>
      </Section>

      <Section title="Próximas ações">
        <ol className="list-decimal list-inside text-sm space-y-1">
          {result.action_items.map((x, i) => <li key={i}>{x}</li>)}
        </ol>
      </Section>

      {result.replication_script && (
        <Section title="Roteiro adaptado">
          <pre className="whitespace-pre-wrap text-sm bg-zinc-900 p-3 rounded">{result.replication_script}</pre>
        </Section>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded border border-zinc-700 p-4">
      <h3 className="text-sm font-semibold uppercase tracking-wider text-zinc-400 mb-3">{title}</h3>
      {children}
    </section>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/AnalysisRunning.tsx web/src/components/AnalysisResult.tsx
git commit -m "feat(web): AnalysisRunning + AnalysisResult"
```

---

### Task 26: AnalysesSidebar component

**Files:**
- Create: `/workspace/web/src/components/AnalysesSidebar.tsx`

- [ ] **Step 1: Implement AnalysesSidebar.tsx**

`/workspace/web/src/components/AnalysesSidebar.tsx`:

```tsx
import type { AnalysisListItem } from "../types";

interface Props {
  items: AnalysisListItem[];
  currentId?: string;
  onSelect: (id: string) => void;
  onNew: () => void;
}

const STATUS_DOT: Record<string, string> = {
  processing: "bg-amber-500",
  done: "bg-emerald-500",
  error: "bg-rose-500",
};

const MODE_LABEL: Record<string, string> = {
  pre_post: "Pré",
  reference: "Ref",
  post_mortem: "Post",
};

export function AnalysesSidebar({ items, currentId, onSelect, onNew }: Props) {
  return (
    <aside className="w-64 h-full border-r border-zinc-800 flex flex-col">
      <div className="p-3 border-b border-zinc-800">
        <button
          onClick={onNew}
          className="w-full py-2 rounded bg-zinc-800 hover:bg-zinc-700 text-sm"
        >
          + nova análise
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
        {items.length === 0 && (
          <div className="p-4 text-xs text-zinc-500">Nenhuma análise ainda.</div>
        )}
        {items.map((it) => (
          <button
            key={it.id}
            onClick={() => onSelect(it.id)}
            className={`w-full text-left p-3 border-b border-zinc-900 hover:bg-zinc-900 text-sm flex items-center gap-2 ${
              currentId === it.id ? "bg-zinc-900" : ""
            }`}
          >
            <span className={`inline-block w-2 h-2 rounded-full ${STATUS_DOT[it.status]}`} />
            <span className="text-xs text-zinc-500 w-10">{MODE_LABEL[it.mode]}</span>
            <span className="flex-1 truncate">{it.original_name || it.id.slice(0, 8)}</span>
            {it.verdict && <span className="text-xs text-zinc-400">{it.verdict}</span>}
          </button>
        ))}
      </div>
    </aside>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/AnalysesSidebar.tsx
git commit -m "feat(web): AnalysesSidebar component"
```

---

### Task 27: App.tsx wiring (state machine + orchestration)

**Files:**
- Modify: `/workspace/web/src/App.tsx`

- [ ] **Step 1: Replace App.tsx**

Overwrite `/workspace/web/src/App.tsx`:

```tsx
import { useEffect, useRef, useState } from "react";
import {
  getAnalysis,
  getSignedURL,
  listAnalyses,
  startAnalyze,
  uploadToGCS,
} from "./api";
import { AnalysesSidebar } from "./components/AnalysesSidebar";
import { AnalysisForm } from "./components/AnalysisForm";
import { AnalysisResult } from "./components/AnalysisResult";
import { AnalysisRunning } from "./components/AnalysisRunning";
import type { AnalysisListItem, AnalysisStatus, BusinessContext, Metrics, Mode } from "./types";

type View =
  | { kind: "form" }
  | { kind: "uploading"; pct: number }
  | { kind: "running"; id: string; progressMsg: string }
  | { kind: "result"; status: AnalysisStatus }
  | { kind: "error"; msg: string };

export default function App() {
  const [view, setView] = useState<View>({ kind: "form" });
  const [list, setList] = useState<AnalysisListItem[]>([]);
  const [currentId, setCurrentId] = useState<string | undefined>();
  const pollRef = useRef<number | null>(null);

  async function refreshList() {
    try {
      setList(await listAnalyses());
    } catch (e) {
      console.error(e);
    }
  }

  useEffect(() => {
    refreshList();
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, []);

  function stopPolling() {
    if (pollRef.current) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }

  function startPolling(id: string) {
    stopPolling();
    pollRef.current = window.setInterval(async () => {
      try {
        const s = await getAnalysis(id);
        if (s.status === "processing") {
          setView({ kind: "running", id, progressMsg: s.progress_msg });
        } else {
          stopPolling();
          setView({ kind: "result", status: s });
          refreshList();
        }
      } catch (e) {
        stopPolling();
        setView({ kind: "error", msg: String(e) });
      }
    }, 3000);
  }

  async function handleSubmit(data: {
    file: File;
    mode: Mode;
    businessContext: BusinessContext;
    metrics?: Metrics;
  }) {
    try {
      setView({ kind: "uploading", pct: 0 });
      const signed = await getSignedURL(data.file.name, data.file.type || "video/mp4");
      await uploadToGCS(signed.put_url, data.file, (pct) =>
        setView({ kind: "uploading", pct })
      );
      const { id } = await startAnalyze({
        gcs_uri: signed.gcs_uri,
        original_name: data.file.name,
        mode: data.mode,
        business_context: data.businessContext,
        metrics: data.metrics,
      });
      setCurrentId(id);
      setView({ kind: "running", id, progressMsg: "Iniciando análise..." });
      startPolling(id);
    } catch (e) {
      setView({ kind: "error", msg: String(e) });
    }
  }

  async function handleSelect(id: string) {
    stopPolling();
    setCurrentId(id);
    try {
      const s = await getAnalysis(id);
      if (s.status === "processing") {
        setView({ kind: "running", id, progressMsg: s.progress_msg });
        startPolling(id);
      } else {
        setView({ kind: "result", status: s });
      }
    } catch (e) {
      setView({ kind: "error", msg: String(e) });
    }
  }

  function handleNew() {
    stopPolling();
    setCurrentId(undefined);
    setView({ kind: "form" });
  }

  return (
    <div className="h-full flex bg-zinc-950 text-zinc-100">
      <AnalysesSidebar
        items={list}
        currentId={currentId}
        onSelect={handleSelect}
        onNew={handleNew}
      />
      <main className="flex-1 overflow-y-auto p-8 max-w-3xl mx-auto w-full">
        <h1 className="text-xl font-semibold mb-6">Video Analyzer</h1>
        {view.kind === "form" && (
          <AnalysisForm onSubmit={handleSubmit} />
        )}
        {view.kind === "uploading" && (
          <AnalysisRunning progressMsg="Subindo vídeo..." uploadPct={view.pct} />
        )}
        {view.kind === "running" && (
          <AnalysisRunning progressMsg={view.progressMsg} />
        )}
        {view.kind === "result" && view.status.status === "done" && view.status.result && (
          <AnalysisResult result={view.status.result} />
        )}
        {view.kind === "result" && view.status.status === "error" && (
          <div className="rounded border border-rose-700 bg-rose-950 p-4 text-rose-200">
            <div className="font-medium mb-1">Erro</div>
            <div className="text-sm">{view.status.error}</div>
            <button onClick={handleNew} className="mt-3 text-sm underline">Nova análise</button>
          </div>
        )}
        {view.kind === "error" && (
          <div className="rounded border border-rose-700 bg-rose-950 p-4 text-rose-200">
            <div className="text-sm">{view.msg}</div>
            <button onClick={handleNew} className="mt-3 text-sm underline">Voltar</button>
          </div>
        )}
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Type-check + run dev server**

```bash
cd /workspace/web && npx tsc --noEmit
```

Expected: no errors.

```bash
cd /workspace/web && npm run dev
```

Open http://localhost:5173. With the backend also running (Task 21), do an end-to-end manual test: fill form → upload → see progress → see result.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): App wiring — form → upload → poll → result"
```

---

### Task 28: Visual polish via frontend-design skill

**This task is executed by invoking the `frontend-design` skill.**

- [ ] **Step 1: Verify the functional baseline works**

End-to-end manual test on http://localhost:5173. If the functional UI doesn't work, fix Task 27 issues before polishing.

- [ ] **Step 2: Invoke the skill**

Run the `frontend-design` skill with this brief:

```
Polish the Video Analyzer UI. The functional baseline is in /workspace/web/.
Apply visual treatment that:
- Conveys "fast, honest, no-fluff content analyst" — not generic SaaS, not corporate
- Gives weight to the verdict block (it's the punchline of every analysis)
- Treats the loading state (1-3 min wait) with dignity — not a boring spinner
- Differentiates the three sections (Hook / Structure / Visual) visually so the eye doesn't blur them
- Uses tight, dense layout — content-creator user, expects to skim fast

Existing components to refine (don't rewrite from scratch):
- src/components/AnalysisForm.tsx
- src/components/AnalysisRunning.tsx
- src/components/AnalysisResult.tsx
- src/components/AnalysesSidebar.tsx
- src/App.tsx

Keep the state machine, props, and routing intact. Refine only Tailwind classes, structural markup, and small additions (icons, typography hierarchy, micro-interactions).
```

- [ ] **Step 3: Verify the polished UI still works end-to-end**

`npm run dev` and do the same manual test.

- [ ] **Step 4: Commit (the design skill should have done this, but verify)**

```bash
git log --oneline | head -3
# expected: top commit is from the design pass
```

If the skill didn't commit, do it manually:
```bash
git add web/
git commit -m "style(web): visual polish via frontend-design skill"
```

---

## Phase 11: Local smoke test

### Task 29: End-to-end manual smoke test (local)

- [ ] **Step 1: Both services running**

Terminal 1:
```bash
cd /workspace
docker compose up -d postgres
cd api
# ... export env vars from Task 21 ...
go run ./cmd/server
```

Terminal 2:
```bash
cd /workspace/web
npm run dev
```

- [ ] **Step 2: Test the happy path**

1. Open http://localhost:5173
2. Fill the form with realistic ScrapJobs context (use values from `content-strategy.md`)
3. Upload a short test video (the same one from Task 22)
4. Watch upload progress bar → "Iniciando análise..." → "Analisando estrutura visual..." → "Gerando insights com IA..." → result page
5. Verify the verdict block, hook score, and at least 3 key_insights appear
6. Click sidebar → switch between current and any past analysis
7. Click "nova análise" → form clears, you can submit again

- [ ] **Step 3: Test the error path**

1. Submit with an invalid GCS bucket name in `.env` (e.g. set `GCS_BUCKET=does-not-exist`), restart backend.
2. Form submission should fail at upload or analyze step.
3. Error banner appears with a sensible message.
4. Revert env var; restart.

- [ ] **Step 4: Verify DB and GCS state**

```bash
psql -h localhost -U postgres -d video_analyzer -c "SELECT id, status, mode, original_name, created_at, completed_at FROM analyses ORDER BY created_at DESC LIMIT 5;"
```

```bash
gcloud storage ls gs://$GCS_BUCKET/ --recursive
# expected: no objects (or only very recent failed-cleanup ones)
```

No commit; this is verification only.

---

## Phase 12: Deploy

### Task 30: Dockerfiles + Caddyfile + .dockerignore

**Files:**
- Create: `/workspace/api/Dockerfile`
- Create: `/workspace/api/.dockerignore`
- Create: `/workspace/web/Dockerfile`
- Create: `/workspace/web/Caddyfile`
- Create: `/workspace/web/.dockerignore`

- [ ] **Step 1: api/Dockerfile**

```dockerfile
# Build stage (Go)
FROM golang:1.22-alpine AS gobuild
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Runtime stage (Node sidecar + Go binary)
FROM node:20-alpine
WORKDIR /app
COPY tools/package*.json ./tools/
RUN cd tools && npm ci --omit=dev
COPY tools/analyze-video.js ./tools/
COPY --from=gobuild /server /server
ENV ANALYZE_VIDEO_SCRIPT=/app/tools/analyze-video.js
EXPOSE 8080
CMD ["/server"]
```

- [ ] **Step 2: api/.dockerignore**

```
.git
testdata
*.log
node_modules
.env
.env.*
coverage.*
```

- [ ] **Step 3: web/Dockerfile**

```dockerfile
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

FROM caddy:2-alpine
COPY --from=build /app/dist /usr/share/caddy
COPY Caddyfile /etc/caddy/Caddyfile
EXPOSE 80
```

- [ ] **Step 4: web/Caddyfile**

```caddyfile
:80 {
  root * /usr/share/caddy
  try_files {path} /index.html
  file_server
  encode gzip
}
```

- [ ] **Step 5: web/.dockerignore**

```
.git
node_modules
dist
.env
.env.*
```

- [ ] **Step 6: Local Docker build smoke test**

```bash
cd /workspace/api && docker build -t va-api:test .
cd /workspace/web && docker build --build-arg VITE_API_URL=http://localhost:8080 -t va-web:test .
```

Both should succeed.

- [ ] **Step 7: Commit**

```bash
git add api/Dockerfile api/.dockerignore web/Dockerfile web/Caddyfile web/.dockerignore
git commit -m "chore(deploy): Dockerfiles + Caddyfile for api/web"
```

---

### Task 31: Push to GitHub remote

- [ ] **Step 1: Create GitHub repo and push**

Create a private repo on github.com (web UI) — e.g. `video-analyzer`. Don't initialize with README.

```bash
cd /workspace
git remote add origin git@github.com:<your-username>/video-analyzer.git
git branch -M main
git push -u origin main
```

Expected: push succeeds. Verify .gitignore did its job:
```bash
git ls-files | grep -E '(json|\.env)$'
# expected: only package.json, package-lock.json, tsconfig*.json — NO secrets
```

If you see `gen-lang-client-*.json` listed, STOP. The credentials leaked. Rotate the key in GCP Console and force-cleanup the repo history before continuing.

---

### Task 32: Railway project + Postgres plugin

- [ ] **Step 1: Create Railway project**

Either via Railway web UI (railway.app → new project → empty project) or CLI:

```bash
npm i -g @railway/cli
railway login
cd /workspace && railway init video-analyzer
```

- [ ] **Step 2: Add Postgres plugin**

Railway dashboard → New → Database → PostgreSQL. Wait for provisioning. Note that `DATABASE_URL` is now available as `${{Postgres.DATABASE_URL}}` to be referenced from other services.

---

### Task 33: Deploy api service

**Prereq (BLOCKING):** rotate Anthropic key (Task 9 from brainstorm — `https://console.anthropic.com/settings/keys`).

- [ ] **Step 1: Create api service in Railway**

Railway dashboard → New → GitHub Repo → select `video-analyzer` repo → set Root Directory to `/api`.

- [ ] **Step 2: Set env vars**

In the api service Variables tab, add:

| Key | Value |
|---|---|
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` |
| `PORT` | `8080` |
| `ALLOWED_ORIGINS` | (will fill after Task 34) |
| `GOOGLE_APPLICATION_CREDENTIALS_JSON` | (paste the base64 string from Task 5 step 8) |
| `GCS_BUCKET` | `video-analyzer-tmp` |
| `GCP_PROJECT_ID` | (your project ID) |
| `ANTHROPIC_API_KEY` | (your rotated key) |
| `ANTHROPIC_MODEL` | `claude-sonnet-4-6` |

- [ ] **Step 3: Generate domain**

Settings → Networking → Generate Domain. Copy the URL (e.g. `https://api-xxxx.up.railway.app`).

- [ ] **Step 4: Trigger deploy and watch logs**

If Railway didn't auto-deploy, push any commit:
```bash
cd /workspace && git commit --allow-empty -m "chore: trigger api deploy" && git push
```

Watch Deployments → Build Logs → Deploy Logs. Expect to see:
```
{"level":"INFO","msg":"listening","port":"8080"}
```

- [ ] **Step 5: Smoke test the deployed api**

```bash
curl -s https://api-xxxx.up.railway.app/healthz
# expected: ok

curl -s -X POST https://api-xxxx.up.railway.app/api/uploads/signed-url \
  -H "Content-Type: application/json" \
  -d '{"filename":"x.mp4","content_type":"video/mp4"}' | jq .
# expected: { put_url, gcs_uri, expires_at }
```

---

### Task 34: Deploy web service + update CORS

- [ ] **Step 1: Create web service**

Railway dashboard → New → GitHub Repo → same repo → Root Directory: `/web`.

- [ ] **Step 2: Set build-arg env var**

In Variables tab:
- `VITE_API_URL` = `https://api-xxxx.up.railway.app` (from Task 33)

Railway will pass this as `--build-arg` automatically based on the Dockerfile ARG declaration.

- [ ] **Step 3: Generate domain**

Settings → Networking → Generate Domain. Copy URL (e.g. `https://web-yyyy.up.railway.app`).

- [ ] **Step 4: Update CORS on the api service**

Edit api service Variables:
- `ALLOWED_ORIGINS` = `https://web-yyyy.up.railway.app`

This will trigger a re-deploy of the api service.

- [ ] **Step 5: Update GCS bucket CORS to include the production web domain**

```bash
cat > /tmp/cors.json <<EOF
[
  {
    "origin": ["http://localhost:5173", "https://web-yyyy.up.railway.app"],
    "method": ["PUT", "GET"],
    "responseHeader": ["Content-Type", "x-goog-content-length-range"],
    "maxAgeSeconds": 3600
  }
]
EOF

gcloud storage buckets update gs://video-analyzer-tmp --cors-file=/tmp/cors.json
```

- [ ] **Step 6: Wait for redeploy and smoke test the web**

Open https://web-yyyy.up.railway.app in a browser. Verify the form loads.

---

### Task 35: Production smoke test

- [ ] **Step 1: End-to-end production test**

1. Open https://web-yyyy.up.railway.app
2. Fill the form with real ScrapJobs context
3. Upload a short test MP4
4. Watch upload → progress messages → result
5. Verify verdict, hook score, and insights appear

- [ ] **Step 2: Check production state**

In Railway dashboard:
- api logs should show the full lifecycle log lines
- Postgres → Query: `SELECT id, status, completed_at FROM analyses ORDER BY created_at DESC LIMIT 3;`

In GCP Console:
- Storage → video-analyzer-tmp → should be empty (or only have very fresh objects)

- [ ] **Step 3: Test from another device / mobile**

Open the web URL on a phone or another computer. Verify it works end-to-end. Single-user demo is now public — anyone with the URL can use it.

- [ ] **Step 4: Final commit (if needed) and tag**

```bash
cd /workspace
git tag v0.1.0-mvp
git push --tags
```

🎉 **Done.** The video analyzer is deployed.

---

## Post-deploy notes

- **Monitor costs**: GCP billing dashboard + Anthropic console. The free tier should cover initial usage, but a runaway loop could rack up Claude bills fast. Set a billing alert.
- **Watch logs**: Railway → api service → Logs. Look for repeated errors.
- **First user**: try real videos. Note what falls short — that informs the next iteration (yt-dlp for URL inputs? richer metrics? export as PDF?).
- **Backup plan**: if something breaks in prod, revert by pushing a fix. Don't try to manually edit the Railway container.
