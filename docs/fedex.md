# FedEx adapter

FedEx is supported through its public tracking page using headless Chrome / CDP. No FedEx account credentials are required by default.

## Command

```sh
parcelcli track TRACKING_NUMBER --carrier fedex --json
```

No postcode is required.

## Method

The adapter opens FedEx’s public tracker:

```txt
https://www.fedex.com/fedextrack/?trknbr=<TRACKING_NUMBER>
```

The page may redirect internally to:

```txt
https://www.fedex.com/wtrk/track/?trknbr=<TRACKING_NUMBER>&trkqual=...
```

The browser flow obtains a public OAuth token and calls:

```txt
https://api.fedex.com/auth/oauth/v2/token?...grant_type=client_credentials...
https://api.fedex.com/track/v2/shipments
```

Direct non-browser HTTP is unreliable from this environment; use the browser-backed adapter. The adapter parses rendered page text and records relevant FedEx API observations for debugging.

## Normalized fields

The adapter extracts:

- message after `DELIVERY DETAILS`
- latest scan/event from the newest visible timestamp row
- origin after `FROM`
- destination after `TO`
- observed FedEx auth/track API calls

Example successful render observed for `TRACKING_NUMBER` on 2026-05-06:

```json
{
  "carrier": "fedex",
  "tracking_number": "TRACKING_NUMBER",
  "status": "delayed",
  "status_text": "Your package is still on the way and we’re actively working to get you a new delivery date.",
  "last_event": {
    "time": "DATE TIME",
    "text": "Delivery updated",
    "location": "CITY, REGION"
  },
  "raw": {
    "from": "ORIGIN CITY, REGION",
    "to": "DESTINATION CITY, REGION"
  }
}
```

FedEx public tracking can later return `not_found` for the same number if its public web flow says it cannot find the tracking number; the adapter reflects the current rendered carrier result rather than caching older state.

## Status mapping

Rendered text fallback:

- delivered: `Delivered`
- out for delivery: `Out for delivery` when not paired with a delivery-date delay/update
- delayed: `new delivery date`, `Delivery updated`, `Delay`, `Delayed`
- in transit: `We have your package`, `On the way`, `In transit`, `Arrived`, `Departed`
- pre-advice: `Label created`, `Shipment information sent`
- exception: `Delivery exception`, `Clearance delay`, `Action required`
- delivery attempted: `Delivery attempted`, `Attempted delivery`
- ready for pickup: `Available for pickup`, `Hold at location`
- returned: `Return to shipper`, `Returned`
- not found: `We can’t find that tracking number`, `No record of this tracking`, `Not found`, `Unable to retrieve`

## Privacy

No FedEx credentials are stored. Tracking numbers stay local unless an approved aggregator mode is deliberately added later.
