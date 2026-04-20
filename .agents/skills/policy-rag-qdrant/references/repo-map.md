# Repo Map

## Key Files

- `backend/cmd/knowledge-init/main.go`
  Entry point for offline ingestion commands.
- `backend/cmd/knowledge-init/service.go`
  Orchestrates `prepare` and `store`.
- `backend/cmd/knowledge-init/structure_splitter.go`
  Structure-first chunking with length fallback.
- `backend/cmd/knowledge-init/qdrant_store.go`
  Writes artifact chunks to Qdrant through the Eino indexer.
- `backend/internal/agent/infra/rag/search.go`
  Shared search contract.
- `backend/internal/agent/infra/rag/local_qdrant.go`
  Runtime Qdrant retrieval service.
- `backend/internal/agent/ioc/components.go`
  Wires the agent runtime to the local Qdrant service.
- `backend/internal/agent/orchestrator/node/shared/rag/rag_node.go`
  Uses the retrieval service in the orchestrator.

## Commands

Prepare artifact:

```bash
cd backend
go run ./cmd/knowledge-init --mode prepare --config cmd/knowledge-init/host.yaml --url "<policy-url>" --artifact tmp/knowledge/policy.json
```

Store into Qdrant:

```bash
cd backend
go run ./cmd/knowledge-init --mode store --config cmd/knowledge-init/host.yaml --artifact tmp/knowledge/policy.json
```

Verify:

```bash
cd backend
go build -p=1 ./cmd/agent
go test -p=1 ./cmd/knowledge-init ./internal/agent/infra/rag
```

## Runtime Env Vars

- `QDRANT_HOST`
- `QDRANT_PORT`
- `QDRANT_COLLECTION`
- `QDRANT_API_KEY`
- `QDRANT_USE_TLS`

## Interview Framing

One sentence:

`I wrote a project-level skill that guides offline policy cleanup, structure-aware chunking, vector indexing, and Qdrant retrieval integration.`

30-second version:

`I turned the policy knowledge-base workflow into a reusable skill. The core idea is offline preprocessing plus local vector retrieval: load and clean the policy page, split it with structure-first chunking, generate embeddings, write them into Qdrant, and let the agent retrieve from Qdrant at runtime. The skill captures the file map, commands, and design decisions so the workflow is repeatable instead of rediscovered each time.`

Follow-up points:

- `The chunking strategy is structure-first with length fallback, which fits policy and rule documents well.`
- `Separating prepare and store lets us validate cleanup and chunking offline without re-fetching the page each time.`
- `Using Qdrant as the only runtime retrieval path avoids drift between managed and local knowledge-base implementations.`
