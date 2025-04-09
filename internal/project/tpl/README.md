# {{ .ProjectName }}

{{ .ProjectName }} is a CLI tool scaffolded with [`cobra-cli`](https://github.com/inovacc/cobra-cli), designed to help you build Go-based command-line applications with ease.

---

## 🧰 Features

- 🧱 Modular command structure (based on `cmd/` folder)
- 🛠 Auto-wired license and template generation
- ⚙️ Configurable using `config.yaml`
- 🧪 Easily testable with `afero`-based FS abstraction

---

## 🚀 Usage

```bash
# Run
# default config.yaml are set if config parameter is omit
{{ .ProjectName }} --config config.yaml 

# List available subcommands
{{ .ProjectName }} help

