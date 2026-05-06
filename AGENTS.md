# AGENTS.md — parcelcli operating contract

`parcelcli` is a local, user-owned parcel tracking CLI. It is safe for agents to call for read-only tracking.

This repo is the sole source of truth for parcel tracking behavior, supported carriers, examples, and agent instructions. Workspace `TOOLS.md` files should only point here, not duplicate carrier lists or command surfaces.

## Golden path

```sh
parcelcli track <tracking-number> --carrier evri --json
parcelcli track <tracking-number> --carrier evri --postcode <postcode> --json
parcelcli track <tracking-number> --carrier royalmail --json
parcelcli track <tracking-number> --carrier ups --json
parcelcli track <tracking-number> --carrier fedex --json
parcelcli track <tracking-number> --carrier dhl --json
```

## Rules for agents

- Use `--json`; treat human-readable output as display-only.
- For Evri, postcode is optional: without it, use rough public tracking; with it, use fuller/detail tracking. Do not infer a postcode from private memory unless the user clearly asks for detailed home/office tracking.
- For Royal Mail, UPS, FedEx, and DHL, no postcode is required by default. If a carrier asks for postcode later, return/ask for that explicitly; do not guess.
- Do not poll fast. Use 15–30 minute intervals for active delivery watches; longer for non-active parcels.
- Notify only on material changes: status enum, latest event, ETA, courier/handover code, delivery, exception, or blocker.
- Never expose raw carrier page dumps to chat. Summarize `status`, `status_text`, and `last_event`.
- Keep watch state local. Do not send tracking numbers/postcodes to third-party aggregators unless explicitly configured and approved.
- If a carrier returns `unsupported` or `credentials_required`, say that plainly and stop.

## Current carrier support

- `evri` — headless Chrome / CDP against the public Evri tracking page. `--postcode` is optional for fuller/detail tracking.
- `royalmail` — headless Chrome / CDP against the public Royal Mail tracking page. No postcode by default.
- `ups` — headless Chrome / CDP against the public UPS tracking page. No postcode by default.
- `fedex` — headless Chrome / CDP against the public FedEx tracking page. No postcode by default.
- `dhl` — headless Chrome / CDP against the public DHL tracking page. No postcode by default.

## Platform notes

- macOS default Chrome path is `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`.
- Linux discovery checks `google-chrome`, `chromium`, then `chromium-browser` on PATH.
- Headless Chrome/CDP does not normally need a display server, but minimal containers may need browser shared libraries, fonts, and CA certificates.

## Useful commands

```sh
parcelcli doctor --json
parcelcli detect <tracking-number> --json
parcelcli watch add <tracking-number> --carrier evri --label "label"
parcelcli watch add <tracking-number> --carrier evri --postcode <postcode> --label "label"
parcelcli watch add <tracking-number> --carrier royalmail --label "label"
parcelcli watch add <tracking-number> --carrier ups --label "label"
parcelcli watch add <tracking-number> --carrier fedex --label "label"
parcelcli watch add <tracking-number> --carrier dhl --label "label"
parcelcli watch run --json
```

## Error handling

- Missing Evri postcode: continue with rough tracking unless the user asked for address-specific detail.
- Chrome missing: tell the user Chrome/Chromium is required or pass `--chrome`. On Linux, `google-chrome`, `chromium`, or `chromium-browser` on PATH should work headlessly; no X11 desktop is normally required.
- Timeout/WAF: retry once later; do not loop aggressively.
- Unsupported carrier: explain current support and suggest adding an adapter.
