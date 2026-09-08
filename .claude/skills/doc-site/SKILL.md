---
name: doc-site
description: Documentation Site
---

# Documentation Site

Hugo-based documentation site for go-yaml, published to
https://go-openapi.github.io/go-yaml/.

Every page is hand-written. Nothing is generated from source: the API reference
is pkg.go.dev, and the site links to it rather than re-rendering the godoc.

## Running locally

```bash
cd hack/doc-site/hugo
go run gendoc.go

# Visit http://localhost:1313/go-yaml/
# Auto-reloads on changes to docs/doc-site/
```

## Site structure

```
hack/doc-site/hugo/
  hugo.yaml               # main Hugo config
  go-yaml.yaml.template   # version parameters, filled at build time
  go-yaml.yaml            # generated; not committed
  metrics.yaml            # conformance scores, merged into site params
  gendoc.go               # dev server launcher
  layouts/                # shortcodes and partials overriding the theme
  themes/hugo-relearn/    # Relearn documentation theme

docs/doc-site/            # content, mounted by Hugo
  getting-started/        # install, which layer to use, migrating
  usage/                  # the codec: Marshal/Unmarshal and its options
  advanced/               # parser, AST, tokens, YAMLPath, JSON
  about/                  # the fork, conformance, performance, status
  project/                # README, licence, contributing
```

## What may go on the site

**Only what is true of committed code on master.** A page may say a thing is
not there yet; it may never describe something unbuilt. Plans and open designs
live in `.claude/plans/`, not here.

## Conformance numbers

`metrics.yaml` holds the conformance scores and is merged into
`site.Params.metrics`. Reference them with the Relearn `siteparam` shortcode
rather than typing a number into prose:

```markdown
{{% siteparam "metrics.buckets_matched" %}} of {{% siteparam "metrics.buckets" %}}
generator buckets agree with the oracle.
```

Available: `metrics.buckets`, `metrics.buckets_matched`, `metrics.cases`,
`metrics.decoder_scoreable`, `metrics.decoder_passing`, `metrics.tojson_scoreable`,
`metrics.tojson_passing`, `metrics.oracle_scoreable`, `metrics.oracle_passing`.

Version parameters come from `go-yaml.yaml`: `goyaml.goVersion`,
`goyaml.latestRelease`, `goyaml.versionMessage`, `goyaml.buildTime`.

Hugo math functions (`sub`, `mul`, `add`) are not available in markdown content.
Add a computed value to `metrics.yaml` instead.

## Adding a page

1. Create `docs/doc-site/<section>/<name>.md` with Hugo front matter.
2. Set `weight:` to place it in the sidebar.
3. Use Relearn shortcodes: `{{% notice %}}`, `{{% expand %}}`, `{{< tabs >}}`, etc.
4. Link a Go symbol to pkg.go.dev on its first mention in a page.

## Relearn theme features used

- `{{% notice style="info" %}}` -- callout boxes
- `{{% expand title="..." %}}` -- collapsible sections
- `{{< tabs >}}` / `{{% tab %}}` -- tabbed content
- `{{< cards >}}` / `{{% card %}}` -- side-by-side cards
- `{{% icon icon="star" color=orange %}}` -- inline icons
- `{{% siteparam "key" %}}` -- site param substitution
- `{{< mermaid >}}` -- diagrams
- `{{% goversion "go1.25" %}}` -- minimum Go version pill (custom shortcode)
