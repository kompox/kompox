# Kompox Design Docs

This directory is the central hub for Kompox design and architecture documents.

## Document Identity (doc-id)

In this project, documents are managed and cross-referenced by **doc-id**.

- **doc-id** is the string defined by `id:` in a Markdown document's YAML front matter.
- A doc-id must be unique across the entire repository.
- A Markdown document without front matter does not have a doc-id.
- For documents that have a doc-id, cross-document references should use doc-id.

## Document Types and Indexes

Documents are organized by type, and each type has its own subdirectory and index.

| Document Type | Description |
|---|---|
| [ADR](./adr/README.md) | Architecture Decision Records for key technical decisions and their rationale. |
| [Plans](./plans/README.md) | Higher-level plans for multi-document changes and execution sequencing. |
| [Tasks](./tasks/README.md) | Short-term implementation tasks with checklists, tests, and progress tracking. |
| [Spec v1 (current)](./v1/README.md) | Current design/specification documents that serve as the primary implementation reference. |
| [Spec v2 (future)](./v2/README.md) | Future design documents and roadmap-oriented specifications. |
| [Public Docs](./pub/README.md) | Documents intended for sharing outside the organization. |
| [Index Generator](./gen/README.md) | Specification for generating and maintaining document indexes under `design/`. |

See [Index Generator](./gen/README.md) for details on index generation.
