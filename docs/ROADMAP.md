# Documentation Roadmap

This roadmap outlines future efforts to fully document the Biscuit codebase.

1. **Source Auditing** - Review files under `src/` and `biscuit/` for missing
   comments. Document functions, types, and exported variables using Doxygen
   style comments.
2. **Automated Builds** - Integrate Doxygen and Sphinx generation into the
   build system via `setup.sh` and optional CI scripts.
   - Install the `breathe` package with pip.

- Generate documentation by running `doxygen docs/Doxyfile` followed by
  `sphinx-build -b html docs docs/_build/html`.
- Alternatively invoke `make doc` from `docs/` to build both steps
  sequentially.

3. **Architecture Ports** - Create an `arch/x86_64_v1` directory mirroring the
   existing x86 implementation. The port should maintain feature parity while
   documenting each file's purpose and differences.
4. **Continuous Improvement** - Expand documentation coverage and address
   warnings reported by `doxygen docs/Doxyfile`.
