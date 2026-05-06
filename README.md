# parcelcli

A small Go CLI for tracking parcels from the terminal, built for local assistants and agent workflows.

`parcelcli` starts with **Evri** and **Royal Mail** because their public tracking pages are useful but hostile to plain HTTP. The CLI drives a real headless Chrome session, extracts the rendered tracking state, and returns a stable JSON shape that agents can safely consume.

> Unofficial and unaffiliated. This project is not affiliated with, endorsed by, sponsored by, or connected to Evri or any courier. Carrier names are used descriptively only.

## Install

From source:

```sh
git clone git@github.com:cavit99/parcelcli.git
cd parcelcli
go install ./cmd/parcelcli
```

Requirements:

- Go 1.24+
- Google Chrome for Evri tracking (`/Applications/Google Chrome.app/...` on macOS by default)

## Quick start

```sh
parcelcli track TRACKING_NUMBER --carrier evri --postcode POSTCODE
parcelcli track TRACKING_NUMBER --carrier royalmail
```

Machine-readable output:

```sh
parcelcli track TRACKING_NUMBER --carrier evri --postcode POSTCODE --json
```

Example JSON shape:

```json
{
  "carrier": "evri",
  "tracking_number": "TRACKING_NUMBER",
  "postcode": "POSTCODE",
  "status": "delayed",
  "status_text": "We're sorry, your parcel has been delayed...",
  "terminal": false,
  "delivered": false,
  "delayed": true,
  "last_event": {
    "time": "10:58 - Tue May 05",
    "text": "We're sorry, your parcel has been delayed..."
  },
  "source": {
    "method": "browser",
    "url": "https://www.evri.com/track/parcel/...",
    "fetched_at": "2026-05-06T08:30:00Z"
  }
}
```

## Commands

### `track`

Track one parcel.

```sh
parcelcli track TRACKING_NUMBER --carrier evri --postcode POSTCODE [--json]
```

Flags:

- `--carrier evri|royalmail` — carrier slug.
- `--postcode` — required for Evri detail pages; not required for Royal Mail by default.
- `--timeout 35s` — total browser wait budget.
- `--chrome PATH` — override Chrome path.
- `--json` — stable JSON for agents/scripts.

### `detect`

Conservative carrier detection. This is intentionally not magic yet because UK tracking formats overlap.

```sh
parcelcli detect TRACKING_NUMBER
```

### `watch`

Local polling state for assistants.

```sh
parcelcli watch add TRACKING_NUMBER --carrier evri --postcode POSTCODE --label "Amazon order"
parcelcli watch list
parcelcli watch run --json
parcelcli watch remove ID
```

`watch run` only emits material changes by default: status, latest event, or ETA changes. Pass `--all` to emit unchanged watches too.

State lives locally:

- macOS: `~/Library/Application Support/parcelcli/watch.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/parcelcli/watch.json`

### `doctor`

```sh
parcelcli doctor --json
```

Reports local readiness and where watch state lives.

## Agent contract

The full agent contract is in [`AGENTS.md`](AGENTS.md). Short version:

- Prefer `--json` for all agent calls.
- Ask for postcode when Evri is missing it; do not guess.
- Poll politely: 15–30 minutes is fine for active parcels, slower otherwise.
- Notify only on material changes.
- Do not paste raw page text to users; summarize the normalized status.
- Store parcel state locally only.

## Status model

Canonical statuses:

- `unknown`
- `pre_advice`
- `accepted`
- `in_transit`
- `out_for_delivery`
- `delayed`
- `delivery_attempted`
- `ready_for_pickup`
- `delivered`
- `returned`
- `exception`
- `not_found`

## Roadmap

Carrier adapters are intentionally isolated. Planned next carriers:

1. Parcelforce — Royal Mail research notes: [`docs/royalmail.md`](docs/royalmail.md)
2. DPD UK
3. DHL
4. UPS
5. FedEx
6. Yodel

Each carrier may use a different backend: official API, public browser tracking, or optional aggregator. The normalized output stays the same.

## Development

```sh
go test ./...
go vet ./...
go build -o ./dist/parcelcli ./cmd/parcelcli
```

## Why browser-backed Evri?

Evri’s public tracking flow works in a real browser but blocks naïve HTTP clients. The page obtains frontend keys and calls protected platform APIs under browser/WAF context. `parcelcli` uses Chrome DevTools Protocol to load the public page, wait for rendered tracking text, capture relevant API observations, then normalize the result.
