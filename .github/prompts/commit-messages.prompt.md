---
description: Suggest commit messages for staged changes
mode: agent
tools: ['changes', 'codebase', 'runCommands', 'search', 'think']
model: GPT-5 mini (Preview)
---
## Your Task

- Suggest 3 commit messages for staged changes in Git (run `make diff-staged-changes`)
- If nothing is staged yet, ask the user whether you should stage all files in behalf of them (run `git add -A && make diff-staged-changes`)
- Put each suggestion in a code block labeled A/B/C so that the user can easily select and copy
- Do commit with the message selected by the user when they ask you to

## Commit Message Guideline

- Commit messages must be in English
- Title must be within 50 characters
- A blank line following the title must be present
- Body should use bullet points to describe brief change summaries
- Add rationale based on the discussion with the user in the past (if available)
- Follow Conventional Commits
- Choose type from: feat | fix | refactor | perf | chore | docs | test
- Add scope with main directory name if needed
