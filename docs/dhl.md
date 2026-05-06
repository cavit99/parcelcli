# DHL adapter

DHL is supported through its public tracking page using headless Chrome / CDP. No DHL account credentials are required by default.

## Command

```sh
parcelcli track TRACKING_NUMBER --carrier dhl --json
```

No postcode is required.

## Method

The adapter opens DHL’s public tracker:

```txt
https://www.dhl.com/global-en/home/tracking/tracking-parcel.html?submit=1&tracking-id=<TRACKING_NUMBER>
```

Observed browser APIs:

```txt
https://www.dhl.com/utapi?trackingNumber=<TRACKING_NUMBER>&language=en&requesterCountryCode=GB&source=tt
https://www.dhl.de/int-verfolgen/data/search?piececode=<TRACKING_NUMBER>&language=en...
```

Direct non-browser HTTP can return 403/error pages, so the supported path is browser-backed rendered text parsing with relevant API observations recorded for debugging.

## Normalized fields

The adapter extracts:

- headline shipment status
- latest event from `Detailed tracking history`
- event location embedded in the timestamp line when present
- destination country/region when present
- observed DHL tracking API calls

Verified fixture on 2026-05-06:

```json
{
  "carrier": "dhl",
  "tracking_number": "TRACKING_NUMBER",
  "status": "in_transit",
  "status_text": "Customs clearance process started",
  "last_event": {
    "time": "DATE TIME, COUNTRY",
    "text": "The customs clearance process for import into the destination country/region has started. Please find more information here.",
    "location": "COUNTRY"
  }
}
```

## Status mapping

Rendered text fallback:

- delivered: `Delivered`
- out for delivery: `Out for delivery`
- in transit: customs clearance started, arrived, departed, transported, flight/aircraft events
- exception: customs/clearance delay, held, problem, action required
- pre-advice: label/booked text
- ready for pickup: ready/available for pickup
- returned: returned / return to sender
- not found: could not be found / unknown shipment / not found

## Privacy

No DHL credentials are stored. Tracking numbers stay local unless an approved aggregator mode is deliberately added later.
