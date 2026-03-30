# gitea-pages

> A static pages server for [Gitea](https://github.com/go-gitea/gitea)

[![GitHub: Release](https://img.shields.io/github/v/release/deadnews/gitea-pages?logo=github&logoColor=white)](https://github.com/deadnews/gitea-pages/releases/latest)
[![Docker: ghcr](https://img.shields.io/badge/docker-gray.svg?logo=docker&logoColor=white)](https://github.com/deadnews/gitea-pages/pkgs/container/gitea-pages)
[![CI: Main](https://img.shields.io/github/actions/workflow/status/deadnews/gitea-pages/main.yml?branch=main&logo=github&logoColor=white&label=main)](https://github.com/deadnews/gitea-pages)
[![CI: Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/deadnews/gitea-pages/refs/heads/badges/coverage.json)](https://github.com/deadnews/gitea-pages)

## Installation

```sh
docker pull ghcr.io/deadnews/gitea-pages
```

## Configuration

| Variable             | Default    | Description                |
| -------------------- | ---------- | -------------------------- |
| `GITEA_PAGES_SERVER` |            | Gitea server URL           |
| `GITEA_PAGES_TOKEN`  |            | Gitea API token            |
| `GITEA_PAGES_BRANCH` | `gh-pages` | Branch to serve pages from |
| `GITEA_PAGES_ADDR`   | `:8000`    | Listen address             |

## Deploying a site

Deploy with any CI step that pushes to the pages branch:

```yml
- name: Deploy Docs
  uses: peaceiris/actions-gh-pages@v4
  with:
    github_token: ${{ secrets.GITHUB_TOKEN }}
    force_orphan: true
    publish_dir: site
```

## Endpoints

### GET /health

Health check endpoint.

```sh
curl http://127.0.0.1:8000/health
```

### GET /{owner}/{repo}/{path...}

Serves static files from the pages branch of the specified repository.

```sh
# Serves index.html from the pages branch of owner/repo
curl http://127.0.0.1:8000/myorg/myrepo/

# Serves a specific file
curl http://127.0.0.1:8000/myorg/myrepo/assets/style.css
```
