# Corrugation

### What Corrugation is **NOT**

Corrugation is **NOT** a replacement to Inventree or any other parts inventory system.
It is targeted towards household items, those which need not have prices added or even require quantities specified, nor specific part numbers.
For example, I don't want to have to say how many pencils, and which brand - only that a drawer contains pencils.

## Embedding Model Configuration

The text and image embedding models (and their associated prefixes) can be configured at three levels. Each level overrides the one below it:

1. **Per-user override** — set in *Settings → My Settings*. Applies only to that user's records and searches.
2. **Global config** — set in *Settings → Global Settings* (admin only). Acts as the server-wide default for all users who have no per-user override. Persisted in the database and survives restarts.
3. **CLI flags** — `--infinity-text-model`, `--infinity-image-model`, `--infinity-text-query-prefix`, `--infinity-text-document-prefix`. Used to seed the global config on first run (when the database fields are empty). Once the global config has been set — either by the seed or by an admin via the UI — CLI flags have no effect on the resolved model.

In short: **per-user > global config > CLI seed**.
