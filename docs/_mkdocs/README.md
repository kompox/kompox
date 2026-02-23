# `_mkdocs` assets

This directory contains MkDocs build-time resources that should not be published as site pages.

- `custom/`: MkDocs Material custom templates referenced by `theme.custom_dir` (for example, `partials/alternate.html`).

The top-level `mkdocs.yml` excludes directories matching `_*/**` from documentation content, so files here are treated as build helpers rather than docs pages.
