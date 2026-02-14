---
name: git-commit
description: Suggest commit messages based on staged changes and assist with committing using an editor.
---
## Tasks

- Suggest commit messages based on staged changes in the current branch.

## Basic Steps

1) Get staged changes
    - Use the following command to get staged changes: `make git-diff-cached`
    - This command will:
        - Display the diff of staged changes (if any)
        - Show the next commit message file path to create (e.g., `_tmp/git-commit/0038.txt`)
        - Display "No staged changes to commit." if nothing is staged
2) Handle case when there are no staged changes
    - If there are no staged changes, ask the user if they want to stage all files.
    - Only if the user agrees, run the following to get the diff: `git add -A && make git-diff-cached`
3) Create new file `_tmp/git-commit/NNNN.txt` with commit message suggestions.
    - Use the file path shown by `make git-diff-cached` (e.g., `_tmp/git-commit/0038.txt`)
    - `NNNN` is a 4-digit zero-padded sequence number (e.g., `0001`, `0002`, ..., `0038`)
    - Suggest 3 commit message options.
    - Label each suggestion with `# Plan A/B/C` (see the example in the next section).
4) Commit with editor assistance.
    - Execute one of the following:
        - Task `Git: Commit with editor`
        - Terminal command `make git-commit-with-editor`
    - Run this step in the foreground (do NOT background it).
    - An editor will open for the user to edit the commit message file. Wait until the user closes the editor, which will complete the commit.
    - Do not proceed to the next step until the `git commit` process has exited.
5) Confirmation
    - Report the success or failure of `git commit`.
    - Run `make git-show` to check the latest commit message and confirm that one of Plan A/B/C is reflected. If it appears that the commit was made without selecting a plan, warn the user.

## Example of `_tmp/git-commit/NNNN.txt`

```
# Plan A
docs: git-commit skill and AGENTS.md improvements

- git-commit/SKILL.md: Organize steps in a bulleted list
- AGENTS.md: Clarify document structure

# Plan B
docs(dev): Project document structure maintenance
- .github/skills/git-commit/SKILL.md: Organize basic steps in a bulleted list and add confirmation step
- AGENTS.md: Clarify document classification for users and developers, specify placement of ADR/specs/tasks

# Plan C
docs: Document organization and skill step improvements
- git-commit/SKILL.md: Structure steps for better readability
- AGENTS.md: Clarify document placement rules
```

## Commit Message Guidelines

- Commit messages should be written in English.
- The title should be within 50 characters for English.
- Insert a blank line after the title.
- Use bullet points in the body to concisely explain the summary of changes.
- Add reasons based on past discussions with users (if possible).
- Follow Conventional Commits.
  - type: `feat | fix | refactor | perf | chore | docs | test`
  - Use directory names as scopes when necessary (e.g., `design/adr`).