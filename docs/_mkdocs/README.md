# MkDocs helper files (not published)

This directory contains MkDocs-specific helper files that are not part of the public site content.

- hooks/root_index.py: MkDocs hook to generate site/index.html that redirects to the default language path (e.g., ./en/)
- requirements.txt: Python package requirements for building and deploying the docs

These files are excluded from the site output via `exclude_docs` in the repository's `mkdocs.yml`.
