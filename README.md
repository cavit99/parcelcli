# parcelcli

A small Go CLI for tracking parcels from the terminal, built for local assistants and agent workflows.

`parcelcli` uses carrier-specific adapters to drive public tracking flows and return one stable JSON shape. Current browser-backed carriers: **Evri**, **Royal Mail**, **UPS**, **FedEx**, and **DHL**.

> Unofficial and unaffiliated. This project is not affiliated with, endorsed by, sponsored by, or connected to any carrier. Carrier names are used descriptively only.

## Install

From source:

```sh
git clone git@github.com:cavit99/parcelcli.git
cd parcelcli
go install ./cmd/parcelcli
```

Requirements:

- Go 1.24+
- Google Chrome for browser-backed carriers (`/Applications/Google Chrome.app/...` on macOS by default)

## Quick start

```sh
parcelcli track TRACKING_NUMBER --carrier evri --postcode POSTCODE --json
parcelcli track TRACKING_NUMBER --carrier royalmail --json
parcelcli track TRACKING_NUMBER --carrier ups --json
parcelcli track TRACKING_NUMBER --carrier fedex --json
parcelcli track TRACKING_NUMBER --carrier dhl --json
```

Example JSON shape:

```json
{
  "carrier": "ups",
  "tracking_number": "TRACKING_NUMBER",
  "status": "delivered",
  "status_text": "DATE Delivered",
  "terminal": true,
  "delivered": true,
  "delayed": false,
  "last_event": {
    "time": "DATE",
    "text": "Delivered",
    "location": "CITY, REGION"
  },
  "source": {
    "method": "browser",
    "url": "https://www.ups.com/track?...",
    "fetched_at": "2026-05-06T09:30:11Z"
  }
}
```

## Commands

### `track`

Track one parcel.

```sh
parcelcli track TRACKING_NUMBER --carrier evri --postcode POSTCODE [--json]
parcelcli track TRACKING_NUMBER --carrier royalmail [--json]
parcelcli track TRACKING_NUMBER --carrier ups [--json]
parcelcli track TRACKING_NUMBER --carrier fedex [--json]
parcelcli track TRACKING_NUMBER --carrier dhl [--json]
```

Flags:

- `--carrier evri|royalmail|ups|fedex|dhl` — carrier slug.
- `--postcode` — required for Evri; not required for Royal Mail, UPS, FedEx, or DHL by default.
- `--timeout 35s` — total browser wait budget.
- `--chrome PATH` — override Chrome path.
- `--json` — stable JSON for agents/scripts.

### `detect`

Conservative carrier detection. Detection avoids overclaiming because formats overlap.

```sh
parcelcli detect TRACKING_NUMBER --json
```

### `watch`

Local polling state for assistants.

```sh
parcelcli watch add TRACKING_NUMBER --carrier evri --postcode POSTCODE --label "Amazon order"
parcelcli watch add TRACKING_NUMBER --carrier royalmail --label "letter"
parcelcli watch add TRACKING_NUMBER --carrier ups --label "UPS parcel"
parcelcli watch add TRACKING_NUMBER --carrier fedex --label "FedEx parcel"
parcelcli watch add TRACKING_NUMBER --carrier dhl --label "DHL parcel"
parcelcli watch list --json
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

Reports carrier readiness and where watch state lives.

## Carrier docs

- [`docs/evri.md`](docs/evri.md)
- [`docs/royalmail.md`](docs/royalmail.md)
- [`docs/ups.md`](docs/ups.md)
- [`docs/fedex.md`](docs/fedex.md)
- [`docs/dhl.md`](docs/dhl.md)

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

1. Parcelforce
2. DPD UK
3. Yodel

Each carrier may use a different backend: official API, public browser tracking, or optional aggregator. The normalized output stays the same.

## Development

```sh
go test ./...
go vet ./...
go build -o ./dist/parcelcli ./cmd/parcelcli
```
