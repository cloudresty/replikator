# Contributing to Replikator

Thank you for your interest in contributing to Replikator! We welcome contributions of all kinds, including bug reports, feature requests, code, and documentation improvements.

&nbsp;

## How to Contribute

### 1. Fork the Repository

Click the "Fork" button at the top right of the [Replikator GitHub page](https://github.com/cloudresty/replikator) to create your own copy of the repository.

&nbsp;

### 2. Clone Your Fork

```bash
git clone https://github.com/your-username/replikator.git
cd replikator
```

&nbsp;

### 3. Create a Branch

Create a new branch for your changes:

```bash
git checkout -b my-feature-or-bugfix
```

&nbsp;

### 4. Make Your Changes

- Follow the existing code style and Domain-Driven Design (DDD) architecture conventions.
- Add or update tests as appropriate.
- Update documentation in the `README.md` if your changes affect usage or annotations configuration.
- We utilize `controller-runtime`—please ensure any new controllers or predicates follow optimal performance patterns.

&nbsp;

### 5. Run Tests & Validation

Before submitting your changes, make sure all tests pass and your code is properly linted.

Navigate to the application folder and run:

```bash
cd app
go test ./...
golangci-lint run ./...
```

&nbsp;

### 6. Commit and Push

Commit your changes with a clear message:

```bash
git add .
git commit -m "Describe your change"
git push origin my-feature-or-bugfix
```

&nbsp;

### 7. Open a Pull Request

Go to your fork on GitHub and open a pull request (PR) against the `main` branch of the upstream repository. Please include a clear description of your changes, the motivation behind them, and reference any related issues.

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

## Code of Conduct

By participating in this project, you agree to abide by the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/0/code_of_conduct/).

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

## Reporting Issues

If you find a bug or have a feature request, please [open an issue](https://github.com/cloudresty/replikator/issues) and provide as much detail as possible, including your Kubernetes version and Replikator version.

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

## Style Guide

- Use `gofmt` and `golangci-lint` to format and check your code.
- Write clear, concise commit messages.
- Document exported functions and types, especially within the `entity` and `usecase` domains.
- Add or update unit tests for new features or bug fixes.

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

## Questions

If you have questions or need help, feel free to open an issue or start a discussion.

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

Thank you for helping make Replikator an enterprise-grade Kubernetes tool!

&nbsp;

🔝 [back to top](#contributing-to-replikator)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty</sub>

&nbsp;
