from pathlib import Path
from mkdocs.config.defaults import MkDocsConfig


def on_post_build(config: MkDocsConfig) -> None:
    """MkDocs native hook: called after the site is built.
    Generate /index.html that redirects to the default language path so that
    `mkdocs serve` でもルートで言語トップに遷移できる。
    """
    create_root_index(config)


def create_root_index(config: MkDocsConfig) -> None:
    """Generate a root index.html that redirects to the default language.

    This runs on on_post_build so it works for both `mkdocs build` and `mkdocs serve`.
    It uses i18n plugin configuration to determine the default language link if available;
    otherwise falls back to ./en/.
    """
    site_dir = Path(config.site_dir)
    site_dir.mkdir(parents=True, exist_ok=True)

    # Try to read i18n plugin config to find default language link
    default_link = "./en/"
    try:
        i18n = config.plugins.get("i18n")
        if i18n and hasattr(i18n, "config"):
            for lang in getattr(i18n.config, "languages", []) or []:
                is_default = getattr(lang, "default", False)
                build = getattr(lang, "build", True)
                link = getattr(lang, "link", None)
                if is_default and build:
                    if link:
                        default_link = link if link.startswith("./") else f".{link}"
                    break
    except Exception:
        pass

    content = f"""<!doctype html>
<html>
  <head>
    <meta http-equiv=\"refresh\" content=\"0; url={default_link}\" />
    <link rel=\"canonical\" href=\"{default_link}\" />
    <title>Redirecting...</title>
  </head>
  <body>
    <p>If you are not redirected automatically, please <a href=\"{default_link}\">click here</a>.</p>
  </body>
</html>
"""
    (site_dir / "index.html").write_text(content, encoding="utf-8")
