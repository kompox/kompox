---
applyTo: "**/*.md"
---
- Always use markdown reference list in the end of the document for links to other documents.

```markdown
- Reference: [K4x-ADR-010] (Introduce Defaults pseudo-resource for CRD ingestion)
- Update [Kompox-CRD.ja.md] to include Defaults and crdPath/appId specifications.
- Refer to [2025-10-15-kom.ja.md] for related KOM and Resource ID task.

[K4x-ADR-010]: ../../design/adr/K4x-ADR-010.md
[Kompox-CRD.ja.md]: ../../design/v1/Kompox-CRD.ja.md
[2025-10-15-kom.ja.md]: ./2025-10-15-kom.ja.md
```
