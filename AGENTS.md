# Agent Instructions

All code changes must use modern language paradigms and idiomatic style. Functions must be documented using Doxygen-compatible comments detailing purpose, parameters, return values, and global variables. When modifying code, apply formatting and prettification but leave comment-only changes unformatted. Integrate Doxygen with Sphinx using Breathe for documentation builds aimed at Read the Docs. Every modified file must be formatted with `gofmt` and contain Doxygen documentation.

## Extended Guidelines

- Any file touched must be fully refactored to modern Go **1.23.x** idioms. Decompose complex logic, unroll loops, flatten nested structures, remove outdated patterns, and replace special functions with standard equivalents. Emphasize modern design patterns throughout.
- Ensure that all code is thoroughly commented. Each function requires Doxygen-compatible documentation.
- Always run `gofmt` or an equivalent formatter on changed files after modifications.
- Before building or testing, install a comprehensive suite of development tools from **apt**, **pip**, and **npm**, including packages from common PPAs and repositories, to provide a maximally provisioned environment.

