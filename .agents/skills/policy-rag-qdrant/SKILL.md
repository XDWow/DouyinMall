---
name: policy-rag-qdrant
description: Offline policy-document ingestion and local RAG for this repo using Eino and Qdrant. Use when Codex needs to ingest Jinritemai or similar rule pages, clean and normalize content, split by chapter or clause with length fallback, store embeddings in Qdrant, wire the agent to local retrieval, or explain this implementation in an interview.
---

# Policy RAG Qdrant

Use this skill for the repository's policy knowledge-base path. Keep the implementation focused on offline preprocessing plus Qdrant retrieval, not the removed managed knowledge-base flow.

## Core Workflow

1. Inspect the current implementation before editing. Start with:
   - `backend/cmd/knowledge-init/`
   - `backend/internal/agent/infra/rag/local_qdrant.go`
   - `backend/internal/agent/ioc/components.go`
   - `backend/internal/agent/orchestrator/node/shared/rag/rag_node.go`
2. Keep ingestion split into two phases:
   - `prepare`: load page -> clean text -> structure-first chunking -> artifact JSON
   - `store`: artifact -> embedding -> Qdrant
3. Keep chunking structure-first:
   - split by chapter or clause headings such as `Chapter 1`, `1.1`, `2.3.1`, or list/title hierarchy first
   - use recursive length splitting only when a structural block is still too large
   - preserve metadata such as `title_path`, `chapter_title`, `section_title`, `clause_title`, `chunk_strategy`
   - merge trivial fragments that do not have standalone meaning
4. Keep embedding and retrieval consistent:
   - use the same embedder family for indexing and retrieval
   - match Qdrant collection and vector dimension to the embedding model
   - prefer metadata filters like `knowledge_id`, `category`, and `title_path` for targeted retrieval
5. Keep runtime retrieval Qdrant-only:
   - do not reintroduce the old managed knowledge-base path
   - normalize metadata returned by the official Eino Qdrant retriever before downstream use
   - apply application-side score filtering if wrapper behavior does not fully enforce the requested threshold

## Decision Rules

- If the user only wants offline cleanup or chunk review, stop at `prepare` and inspect the artifact.
- If the user wants the local knowledge base ready for querying, run `prepare` and then `store`.
- If the user asks for interview framing, describe the solution as "offline preprocessing plus local vector retrieval" and then expand into structure-aware chunking, embeddings, and Qdrant retrieval.

## Working Style

- Prefer small, verifiable changes over broad refactors.
- Keep SKILL content lean; load extra details from `references/repo-map.md` only when you need exact file paths, commands, or interview wording.
- When explaining the design, emphasize why policy documents benefit from structure-first chunking: better semantic boundaries, cleaner recall, and easier answer grounding.

## Output Expectations

- For code tasks, point to the concrete files changed and mention how the flow now works end to end.
- For interview prep, provide:
  - a one-sentence answer
  - a 30-second answer
  - two or three follow-up points for deeper questioning

## Reference

- Read `references/repo-map.md` for the repository map, commands, env vars, and interview-ready framing.
