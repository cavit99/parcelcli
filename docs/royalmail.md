# Royal Mail adapter

Royal Mail is supported through its public tracking page using headless Chrome / CDP. No Royal Mail account credentials are required by default.

## Command

```sh
parcelcli track TRACKING_NUMBER --carrier royalmail --json
```

No postcode is required by default.

## Method

The adapter opens Royal Mail’s public tracker, fills `#barcode-input`, submits the form, and captures rendered status text plus observed public API calls.

Public page:

```txt
https://www.royalmail.com/track-your-item
```

Observed public API call:

```txt
https://api-web.royalmail.com/mailpieces/microsummary/v1/summary/<TRACKING_NUMBER>
```

Plain HTTP calls from this environment are unreliable because Royal Mail/Akamai can reject or stall non-browser requests. The browser path is the supported path.

## Normalized fields

The adapter returns the standard `model.Result` shape:

- `status`, `status_text`, `terminal`, `delivered`, `delayed`
- `last_event` and `events` when Royal Mail exposes event data
- `source.method = browser`
- `raw.api_observations` with only relevant Royal Mail API observations

## Status mapping

The adapter maps Royal Mail event codes first where available, then rendered/API text fallback:

- delivered: `EVKSP`, `EVKOP`, `EVKSF`, delivered text
- ready for pickup: `EVPLA`, available-for-collection text
- delivery attempted: `EVKNR`, unable-to-deliver / attempted text
- out for delivery: `EVGPD`, out-for-delivery / ready-for-delivery text
- in transit: `EVNSR`, `EVODO`, `EVORI`, `EVOAC`, `EVAIE`, `EVAIP`, `EVPPA`, `EVDAV`, `EVIMC`, `EVDAC`, `EVNRT`, `EVOCO`, `RSRXS`, `RORXS`, `EVNDA`, `EVBAV`, `EVKLS`, `EVIAV`, in-transit text
- returned: returned-to-sender text
- exception: restricted/prohibited/duplicate/access-denied text or `E0015`
- not found: `E1142`, not-recognised / cannot-be-located text

## Optional account APIs

Royal Mail also has official/account APIs and mobile-account endpoints, but those require credentials and are intentionally not used by the default adapter. Add a separate explicit mode later if account history or push subscriptions are ever needed.

## Privacy

Tracking numbers stay local. Do not send them to aggregators unless explicitly configured and approved.
