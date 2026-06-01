# Contributing to G-man CLI

First off, thank you for considering contributing to G-man CLI! It’s developers like you who help make our command-line ecosystem and system daemon a powerful, industrial-grade solution for the Steam automation community.

By contributing to this project, you agree to abide by its terms and follow the coding standards outlined below.

---

## 🟢 How Can I Contribute?

### Reporting Bugs

* **Check the Issues:** Search existing GitHub issues to see if the bug has already been reported.
* **Provide Details:** Use a clear, descriptive title. Include steps to reproduce, the expected behavior, and what actually occurred.
* **Environment:** Mention your Go version, OS, client environment, and any specific logs from `g-mand`.

### Suggesting Enhancements

* **Open an Issue:** Describe the feature you'd like to see and, most importantly, **why** it would be useful for the system control toolchain.
* **Scope & Generality:** G-man CLI is built to be a universal, lightweight orchestrator.
  * **Keep it Generic:** We do **not** accept highly specialized business logic, proprietary rules, or narrow niche system parameters tailored for a single private trading bot.
  * **Leverage the Driver System:** Game-specific logic should be built strictly inside decoupled adapters (like `pkg/game/tf2.go`) implementing the `Driver` and `InventoryProvider` interfaces.

### Pull Requests

* **Fork and Branch:** Create a branch from `main` with a descriptive name (e.g., `feat/dota2-driver` or `fix/socket-permissions`).
* **Atomic Commits:** Keep your commits small, clean, and highly focused.
* **Tests:** Every new driver, helper, or bug fix **must** include corresponding unit tests inside `pkg/game` or the respective package.
* **Documentation & Formatting:** Run `make format` to format your Go files and keep the `README.md` / `README_RU.md` files updated.

## 🛠 Development Standards

G-man CLI is built with a focus on high performance, thread-safety, and minimal resource footprints.

### 1. Code Style & Formatting

* **Formatting:** All Go source files must be formatted with the standard Go rules. We enforce this via:
  ```shell
  make format
  ```
  *(This automatically applies license headers via `addlicense` and runs `golangci-lint run --fix` to clean up imports).*
* **Linting:** We enforce strict linting rules. Ensure there are **0 issues** by running:
  ```shell
  make lint
  ```
* **Naming:** Exported variables and structures must use `PascalCase`, while internal ones use `camelCase`. Avoid package name stuttering (e.g., use `game.Driver` instead of `game.GameDriver`).

### 2. Concurrency, State & Safety

* **No Global State:** Global variables are strictly forbidden. Use structs and pass dependencies (like clients, stores, and registries) explicitly through constructors.
* **Thread-Safety:** The daemon `g-mand` is highly concurrent (handling gRPC requests, event bus subscriptions, and background workers simultaneously). Use proper read-write mutexes (`sync.RWMutex`) or atomic variables for all shared state.
* **Clean Defers & Exit Codes:** Do **not** call `os.Exit(1)` inside blocks where `defer` statements are registered (this causes the `exitAfterDefer` linter warning). Instead, use the deferred exit code check pattern:
  ```go
  var exitCode int
  defer func() {
      if exitCode != 0 {
          os.Exit(exitCode)
      }
  }()
  ```

### 3. Error Handling

* **Wrap Errors:** Use `fmt.Errorf("context: %w", err)` to provide contextual tracing for error propagation.
* **Handle / Silence Lints:** Explicitly discard expected ignorable errors (like `w.Flush()`) using a blank identifier (`_ =`) to maintain compliance with `gosec`.

### 4. Structured Logging

* **Zero raw prints:** Do not use `fmt.Println` or the standard `log` package inside daemon systems. Use G-man's structured `pkg/log` package.
* **Contextual Log Fields:** Always attach structured context fields where appropriate (e.g., `log.Uint32("appid", id)`).

## 📦 Dependency Policy

We strive to keep the dependency tree of G-man CLI as lean and modular as possible.

* **Standard Library First:** Always prefer Go standard library packages.
* **Permissive Licenses:** New dependencies must have permissive licenses (MIT, BSD, Apache 2.0).

## 💬 Commit Messages

We strictly follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

* `feat:` for new features (e.g., `feat(game): add cs2 driver adapter`).
* `fix:` for bug fixes (e.g., `fix(daemon): resolve memory leak on exit`).
* `docs:` for documentation changes.
* `refactor:` for code changes that neither fix a bug nor add a feature.

## 🧪 Testing & Verification

We rely heavily on automated testing to ensure the daemon and registry remain perfectly stable.

* **Race Detector:** Always run tests with the race detector enabled to ensure concurrency safety:
  ```shell
  make race
  ```
* **Coverage:** Generate and verify coverage reports using:
  ```shell
  make cover
  ```

---

## 🔒 Security Vulnerabilities

If you discover a security vulnerability (especially related to credentials handling or IPC channels security), **do not open a public issue**. Instead, please contact the maintainers privately at `arsenii.komolov@yandex.ru` (or via Telegram: `t.me/LemonadeAK`).

---

## ⚖️ License

By contributing, you agree that your contributions will be licensed under the project's **BSD 3-Clause License**. See [LICENSE](LICENSE) for full details.

---

<div align="center">
  <sub>Happy coding, and see you in the console!</sub>
</div>
