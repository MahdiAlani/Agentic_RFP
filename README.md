# Agentic RFP

AI-assisted RFP response tool. Upload your product documentation into a
**workspace** (knowledge base), then open a **project** (an RFP) to chat,
draft grounded answers, and work the RFP with AI.

## Architecture

| Piece | Stack | Role |
|-------|-------|------|
| `backend/` | Go (net/http, pgx) | REST API + JWT auth; produces jobs to Redis |
| `workers/` | Python | Consume Redis streams — embed documents, generate answers/chat |
| `frontend/` | React + Vite | Web UI |
| Postgres (pgvector) | Docker | Data + vector store |
| Redis | Docker | Job queue (streams: `embed`, `generate`) |
| MinIO | Docker | Object storage for uploaded files |

## Prerequisites

- Docker Desktop
- Go 1.26+
- Node 18+
- Python 3.11+

## Quick start

```powershell
./start.ps1
```

This brings up the infra containers, waits for Postgres, applies all migrations,
and launches the API + frontend in separate windows. Then, in a new terminal,
start the workers:

```powershell
cd workers
pip install -r requirements.txt
python main.py
```

- Frontend: http://localhost:5173
- API: http://localhost:8080
- MinIO console: http://localhost:9001 (minioadmin / minioadmin)

Stop everything (API, frontend, containers) with:

```powershell
./stop.ps1
```

(Workers stop with Ctrl+C in their terminal. Containers are stopped but their
data volumes are kept.)

## Manual start

```bash
docker compose up -d                              # postgres, redis, minio

# apply migrations (repo root)
for f in migrations/*.up.sql; do
  docker compose exec -T postgres psql -U postgres -d app < "$f"
done

cd backend && go run ./cmd/api                     # API on :8080
cd frontend && npm install && npm run dev          # UI on :5173
cd workers && pip install -r requirements.txt && python main.py
```

## Configuration

All services read the repo-root `.env` (Postgres, Redis, MinIO, `JWT_SECRET`).

## Status

Working end to end:

- **Auth** — register/login with bcrypt-hashed passwords and JWTs.
- **Workspaces & projects** — a workspace is a knowledge base; a project is one RFP.
- **Document upload** — browser uploads straight to MinIO via presigned URLs;
  the API confirms the object landed and queues it.
- **Embedding** — workers extract text (PDF, DOCX, plain text), chunk it, embed
  it with OpenAI and store vectors in pgvector (HNSW index).
- **Question extraction** — an uploaded RFP is read by the model and turned
  into one ordered answer row per question.
- **Answer drafting** — each question retrieves the closest chunks from the
  workspace knowledge base plus the project's own supporting docs (never the
  RFP itself, never another project's files), drafts a grounded answer and
  records the chunks it used as citations.
- **Chat** — multi-turn chat over the same retrieval.
- **Queue** — Redis Streams with consumer groups: a slow `embed` stream and a
  fast `generate` stream so bulk indexing never delays chat. Crashed workers'
  jobs are reclaimed; jobs failing 3 times go to a dead-letter stream.

Known gaps:

- **No per-user authorization on by-ID routes.** Only workspace list/create are
  scoped to the caller; any signed-in user who knows an ID can read or change
  another user's data. Pinned by `tests/e2e/test_isolation.py` (strict xfail).
- Citations list every retrieved chunk, not only the ones the answer used.

## Testing

| Suite | Needs | Run |
|-------|-------|-----|
| Go unit tests | nothing | `cd backend && go test ./...` |
| Worker unit tests | nothing (DB, Redis, MinIO and OpenAI are faked) | `cd workers && pip install -r requirements-dev.txt && pytest` |
| API end-to-end | running API + infra | `pytest tests/e2e` (from repo root) |
| Full RAG pipeline | above + workers + `OPENAI_API_KEY` | `RUN_LLM_E2E=1 pytest tests/e2e` |

The e2e suite registers throwaway users, drives the real HTTP API and cleans
up after itself. It skips if the API isn't reachable; set `E2E_REQUIRED=1` to
make that a failure instead. The pipeline tests call OpenAI (a few cents per
run), so they're opt-in.

CI (`.github/workflows/ci.yml`) runs the Go and worker unit tests, builds the
frontend, and runs the API e2e suite against Postgres, Redis and MinIO on every
push and pull request. The LLM pipeline is not run in CI.

## Kubernetes (worker autoscaling)

`k8s/` runs the Python workers as two Deployments — `WORKER_TYPE=embed` and
`WORKER_TYPE=generate` — each scaled by [KEDA](https://keda.sh) on its Redis
stream lag. CPU-based HPA doesn't work here: workers block on Redis and OpenAI,
so a backed-up pod looks idle.

```bash
docker build -t agentic-rfp-worker:latest workers/
kubectl create secret generic agentic-rfp-env --from-env-file=.env
kubectl apply -f k8s/
```

`embed` scales to zero between uploads; `generate` keeps one replica warm since
chat is user-facing. Jobs orphaned by a killed pod are recovered by an
`XAUTOCLAIM` loop and dead-lettered to `<stream>:dead` after 3 attempts.

## TODO
- Add ownership checks to every by-ID route (see Known gaps), then drop the
  xfail marker in `tests/e2e/test_isolation.py`.
- Fix Chats to only allow one new conversation at a time. I.e. you can't just spam new Chat.
