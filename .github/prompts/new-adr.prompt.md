---
description: Create a new ADR file
mode: agent
tools: ['runCommands', 'edit', 'search', 'todos', 'changes', 'fetch']
---
## Your Task

- Create a new ADR file in [design/adr].
  - Standard path: `K4x-ADR-NNN.md`
  - Assign unused sequential numbers for `NNN`.
- Always write in English.
- Honor the general instructions: [md] and [en].
- Follow the guidelines: [README.md].
- Refer the existing ADR files as examples for structure and style.
- Use the same set of front-matter fields as in the provided example.
- Maintain a markdown link list in the end of the file.
  - Use the label reference style like `[K4x-ADR-013]` in the content.
  - Do not mention meta instruction files like [README.md].
- Run make gen-index after creating the file to update the ADR index.

## ADR Example

Task file path: `design/adr/K4x-ADR-014.md`

```markdown
---
id: K4x-ADR-014
title: Introduce New Feature
status: proposed
date: 2025-10-23
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-014: Introduce New Feature

## Context

In [K4x-ADR-013], we discussed that ...

## References

- [K4x-ADR-013]
- [Kompox-KubeConverter.ja.md]

[K4x-ADR-013]: ./K4x-ADR-013.md
[Kompox-KubeConverter.ja.md]: ../v1/Kompox-KubeConverter.ja.md
```

[design/adr]: ../../design/adr
[README.md]: ../../design/adr/README.md
[md]: ../instructions/md.instructions.md
[en]: ../instructions/en.instructions.md
