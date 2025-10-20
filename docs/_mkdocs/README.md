# MkDocs helper files (not published)

This directory contains MkDocs-specific helper files that are not part of the public site content.

- hooks/post_build_root_redirect.py: MkDocs hook to generate site/index.html that redirects to the default language path (e.g., ./en/)
- requirements.txt: Python package requirements for building and deploying the docs

Notes
- Files under this directory are excluded from the site output via `exclude_docs` in the repository's `mkdocs.yml`.
- Multiple hook files can be listed in `mkdocs.yml` under `hooks:`; their execution order follows the list order.
- Naming convention (suggested): `<event>_<purpose>.py`, e.g., `post_build_root_redirect.py`.
