# Config Compatibility (application.yaml / application.v2.yaml)

## Load Order

DDBOT now supports dual-file fallback loading:

1. Built-in defaults
2. `application.yaml` (legacy)
3. `application.v2.yaml` (new, highest priority)

If both define the same logical field, v2 wins.

## Compatibility Semantics

- Only `application.yaml`: startup remains compatible with current behavior.
- Only `application.v2.yaml`: runs with the new structure.
- Both files: legacy values are fallback defaults, v2 overrides conflicts.

## Deprecation Warnings

When legacy keys are detected, startup logs non-blocking migration warnings:

`[DEPRECATED_CONFIG] old_key -> new_key (value_source=file:line)`

Each legacy key is warned once per process startup, followed by a summary count.

## Hot Reload

Hot reload watches both files:

- `application.yaml`
- `application.v2.yaml`

Any change triggers the same merge precedence rules.

## v2-only Keys

- `providers.roblox.*` is supported only in `application.v2.yaml`.
- These keys are loaded by a v2 pass-through rule and do not trigger `UNKNOWN_V2_CONFIG`.
- `roblox.*` in `application.yaml` is not a supported compatibility path.

## Migration Command

```bash
ddbot migrate-config --in application.yaml --out application.v2.yaml
```

The command produces:

- a v2 config file
- a migration report (field mappings, unknown legacy fields, default injections)

If the target output already exists, a new `.migrated` file is written instead of overwriting.
