# AGENTS.md — parcelcli operating contract

`parcelcli` is a local, user-owned parcel tracking CLI. It is safe for agents to call for read-only tracking.

## Golden path

```sh
parcelcli track <tracking-number> --carrier evri --postcode <postcode> --json
```

## Rules for agents

- Use `--json`; treat human-readable output as display-only.
- For Evri, postcode is required. Ask the user if it is missing; do not infer from private memory unless the user clearly intends the usual home/office address.
- Do not poll fast. Use 15–30 minute intervals for active delivery watches; longer for non-active parcels.
- Notify only on material changes: status enum, latest event, ETA, courier/handover code, delivery, exception, or blocker.
- Never expose raw carrier page dumps to chat. Summarize `status`, `status_text`, and `last_event`.
- Keep watch state local. Do not send tracking numbers/postcodes to third-party aggregators unless explicitly configured and approved.
- If a carrier returns `unsupported` or `credentials_required`, say that plainly and stop.

## Current carrier support

- `evri` — implemented via headless Chrome / CDP against the public Evri tracking page. Requires `--postcode`.

## Useful commands

```sh
parcelcli doctor --json
parcelcli detect <tracking-number> --json
parcelcli watch add <tracking-number> --carrier evri --postcode <postcode> --label "label"
parcelcli watch run --json
```

## Error handling

- Missing postcode: ask for postcode.
- Chrome missing: tell the user Chrome is required or pass `--chrome`.
- Timeout/WAF: retry once later; do not loop aggressively.
- Unsupported carrier: explain current support and suggest adding an adapter.
