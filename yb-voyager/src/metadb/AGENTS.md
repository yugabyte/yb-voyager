# metadb Package — Engineering Standards

> **Scope:** files under `yb-voyager/src/metadb/`. These are standards for **writing** code here, not only for reviewing it.
> Repo-wide standards live in `AGENTS.md` at each parent directory up to the repo root — read those as well.

## Migration Status Record (MSR)

- MSR is a large, growing struct. When adding new fields, group related fields into sub-structs instead of adding flat fields at the top level.
- JSON tags on MSR fields must remain stable for backward compatibility. Renaming a JSON tag is a breaking change for users upgrading mid-migration.
- Field ordering convention for cutover-related flags: `Requested` → `Detected` → `Processed`.
- Add descriptive comments to MSR fields explaining their semantics and which process/role sets them.

## Testing

- Test the upgrade path: verify that a metaDB created by an older voyager version still works with new code.
