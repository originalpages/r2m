# Changelog

## 0.1.0
- First release: Mermaid diagrams (`tree`, `deps`, `arch`) from a local folder or GitHub repo
- One self-contained Go binary, nothing to install, works offline
- Plain-word usage: `r2m [folder] [overview|deps|folders|all] [save]`, and a
  question-and-answer mode when started without arguments
- Opens one browser page with overview, dependencies and folders (Mermaid library embedded)
- Copy the Mermaid source and zoom from the page; save each diagram as PNG or SVG; pictures, page and saved Markdown carry a footer with the GitHub URL
- Big dependency graphs get a folder picker (and `--focus <folder>`) instead of one huge diagram
- Import parsers for JS/TS, Python, Go and Java; understands `go.mod` and `tsconfig` paths
- Script options: `--type`, `--depth`, `--max-nodes`, `--out`, `--md`, `--update`
- GitHub Action (`action.yml`), static web page (GitHub Pages), and Linux/macOS/Windows release binaries
