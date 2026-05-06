# Royal Mail adapter research

Status: researched, not implemented yet.

`parcelcli` should treat Royal Mail as another browser-backed/public-page carrier first, not as an account-authenticated integration. The goal is private-use parcel tracking from a reference number, without storing a Royal Mail account password or sending data to a third-party aggregator.

## What is available

### Public tracking page

Royal Mail’s current public page is:

- `https://www.royalmail.com/track-your-item`
- hash route also works: `https://www.royalmail.com/track-your-item#/results?trackNumber=<TRACKING_NUMBER>`

A normal server-side fetch from this box can be blocked by Akamai (`HTTP/2 403`, `AkamaiGHost`), but headless Chrome can load the page.

The page loads a public config JSON at:

- `https://www.royalmail.com/spalp/rml_track_and_trace/json`

Important config values observed on 2026-05-06:

- `epsApiConfig.baseUrl`: `https://api-web.royalmail.com`
- `epsApiConfig.mailpieces`: `/mailpieces/v3/`
- `epsApiConfig.microSummary`: `/mailpieces/microsummary/v1/summary/`
- `epsApiConfig.events`: `events`
- `epsApiConfig.ibmClientId`: `3fadedde9e1872d59642d8f0526632aa`
- `epsApiConfig.accessDeniedErrorCode`: `E0015`
- `epsApiConfig.invalidPostcodeErrorCode`: `E1454`

Browser probing with Royal Mail’s own test link `ZW924750388GB` showed the form calls:

- `OPTIONS https://api-web.royalmail.com/mailpieces/microsummary/v1/summary/ZW924750388GB` → `200`
- `GET https://api-web.royalmail.com/mailpieces/microsummary/v1/summary/ZW924750388GB` → `404`

The rendered page message for that stale test reference was:

> Sorry, we're currently unable to confirm the status of your item with reference ZW924750388GB . Please try again tomorrow.

Direct non-browser `curl`/Python calls to `https://api-web.royalmail.com/...` from this environment timed out or failed (`HTTP/2 stream ... INTERNAL_ERROR`, no HTTP/1.1 response), so the first implementation should use Chrome/CDP like the Evri adapter.

### Official / account APIs

Royal Mail has official server-side Tracking APIs on the developer portal, e.g. `https://developer.royalmail.net/product/175625/api/76888`. These are intended for account customers and require API credentials.

The older PHP package `elliotjreed/royal-mail-tracking` is useful for response modelling, but it is not credentialless. It targets official endpoints like:

- default endpoint: `https://api.royalmail.net/mailpieces/v2`
- Events: detailed history for one tracking number
- Summary: latest data for up to 30 tracking numbers
- Signature: proof/signature metadata/image where available

Useful response fields from that package’s examples:

- `mailPieces.summary.oneDBarcode`
- `mailPieces.summary.productName`
- `mailPieces.summary.statusCategory`
- `mailPieces.summary.statusDescription`
- `mailPieces.summary.statusHelpText`
- `mailPieces.summary.summaryLine`
- `mailPieces.summary.lastEventCode`
- `mailPieces.summary.lastEventName`
- `mailPieces.summary.lastEventDateTime`
- `mailPieces.summary.lastEventLocationName`
- `mailPieces.events[]` with `eventCode`, `eventName`, `eventDateTime`, `locationName`
- `errors[]` / `mailPieces.error` with Royal Mail error codes such as `E1142`

### Home Assistant implementation

`jampez77/RoyalMail` is useful, but again not credentialless. It models the Royal Mail mobile/app API and requires username/password or refresh token.

Observed constants:

- `IBM_CLIENT_ID = "e83f49c439b0ebc0b130692bcb8b1cde"`
- `TOKENS_URL = "https://api.royalmail.net/login/v1/tokens"`
- `MAILPIECES_URL = "https://api.royalmail.net/mailpieces/v3.1/user/{guid}/history/{ibmClientId}?limit=6"`
- `MAILPIECE_URL = "https://api.royalmail.net/mailpieces/v3.1/{mailPieceId}/events"`
- `SUBSCRIPTION_URL = "https://api.royalmail.net/pushapi/app/v2/subscription/track/{mailPieceId}"`
- mobile origin header: `consumermobile.royalmail.com`

It is good inspiration for status-code mapping:

- in transit: `EVNSR`, `EVODO`, `EVORI`, `EVOAC`, `EVAIE`, `EVAIP`, `EVPPA`, `EVDAV`, `EVIMC`, `EVDAC`, `EVNRT`, `EVOCO`, `RSRXS`, `RORXS`, `EVNDA`, `EVBAV`, `EVKLS`, `EVIAV`
- delivery failed: `EVKNR`
- delivered: `EVKSP`, `EVKOP`, `EVKSF`
- available for collection: `EVPLA`
- collected: `EVPLC`
- delivery today / out-for-delivery candidate: `EVGPD`

## Recommended `parcelcli` design

### Phase 1: credentialless browser adapter

Add `internal/carriers/royalmail` with a `Tracker` that:

1. Navigates to `https://www.royalmail.com/track-your-item` in Chrome.
2. Accepts/declines cookies if the banner blocks input.
3. Fills `#barcode-input` with the tracking number.
4. Clicks `#submit`.
5. Listens for `api-web.royalmail.com/mailpieces/microsummary/v1/summary/<tracking>` responses.
6. If a JSON response body is available, normalize that first.
7. If the API response body is unavailable or non-JSON, parse the rendered body text and normalize known messages.

This mirrors the Evri strategy and avoids storing Royal Mail account credentials.

Expected command shape:

```sh
parcelcli track ZW924750388GB --carrier royalmail --json
```

No postcode should be required by default. If the public API returns `E1454` / postcode-required data for a specific flow, return `requires: ["postcode"]` rather than guessing.

### Phase 2: optional authenticated account mode

Only if Cavit explicitly wants account-level tracking/history/push subscriptions, add a separate optional mode using Royal Mail account auth. Do not make this the default; it is more sensitive and less portable.

Potential flags/config later:

- `--carrier royalmail-account`
- env/config for refresh token, not plaintext password where avoidable

### Phase 3: Parcelforce handoff

Royal Mail public config includes a Parcelforce redirect URL:

- `https://www.parcelforce.com/track-trace?trackNumber=`

If Royal Mail returns/prints a Parcelforce redirect state, normalize as `carrier: parcelforce` with source URL and add a Parcelforce adapter rather than pretending it is Royal Mail.

## Status mapping

Normalize by event code first, then text fallback.

Recommended mapping:

- delivered: `EVKSP`, `EVKOP`, `EVKSF`, status category/text containing `Delivered`
- ready for pickup: `EVPLA`, text/category containing `Available for collection`
- delivered/collected terminal: `EVPLC`, text/category containing `Collected`
- delivery attempted: `EVKNR`, text containing `delivery attempted`, `unable to deliver`
- out for delivery: `EVGPD`, text containing `ready for delivery`, `out for delivery`, `due to be delivered today`
- in transit: known in-transit code list above, category/text containing `In Transit`
- returned: category/text containing `Returned to Sender`
- exception: category/text containing `Restricted`, `Prohibited`, `Duplicate`, `Access Denied`, `E0015`
- not found: `E1142`, text containing `don't recognise`, `cannot be located`, `not recognised`
- unknown: stale/temporary messages such as `unable to confirm the status... try again tomorrow`

## Test fixtures needed

Before shipping, collect 3–5 real or safely shareable references covering:

1. in transit / accepted
2. out for delivery
3. delivered
4. invalid/not found
5. stale/temporary-unavailable

Without a live non-stale tracking number, implementation can be structurally correct but response parsing from successful JSON remains under-tested.

## Privacy notes

- Keep watches local in `~/Library/Application Support/parcelcli/watch.json`.
- Do not send Royal Mail tracking numbers/postcodes to third-party aggregators unless explicitly configured and approved.
- Do not store Royal Mail account credentials for the default adapter.
