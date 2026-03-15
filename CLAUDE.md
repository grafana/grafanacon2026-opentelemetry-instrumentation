# Claude Instructions

## Repository structure

This repo has a `main` branch and 6 exercise branches (`exercise/01` through `exercise/06`). Each exercise branch is stacked on top of the previous one (i.e., `exercise/02` is based on `exercise/01`, etc.).

When making changes to an earlier exercise branch, rebase all subsequent exercise branches on top of it to keep the stack consistent.

Each exercise branch should have exactly one commit (squash all commits before rebasing downstream branches).

## Validating changes

Always run tests after making changes to go or js files:

```bash
make test
```

This runs both backend and frontend tests.
