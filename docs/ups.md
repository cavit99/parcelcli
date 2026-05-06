# UPS adapter

UPS is supported through its public tracking page using headless Chrome / CDP. No UPS account credentials are required by default.

## Command

```sh
parcelcli track TRACKING_NUMBER --carrier ups --json
```

No postcode is required.

## Method

The adapter opens UPS’s public tracker with a deep link:

```txt
https://www.ups.com/track?loc=en_US&tracknum=<TRACKING_NUMBER>&requester=ST/trackdetails
```

The page calls this public web endpoint in-browser:

```txt
https://webapis.ups.com/track/api/Track/GetStatus?loc=en_US
```

Direct non-browser HTTP is unreliable from this environment because UPS uses Akamai/bot checks, invisible reCAPTCHA/fingerprint endpoints, and browser context. The adapter therefore treats rendered DOM text as the primary source and records relevant API observations for debugging.

## Normalized fields

The adapter extracts:

- current status after `Tracking Details`
- latest visible event/update line
- delivered-to location when present
- received-by name when present
- observed `webapis.ups.com/track/api/Track/GetStatus` calls

Example fixture verified on 2026-05-06:

```json
{
  "carrier": "ups",
  "tracking_number": "TRACKING_NUMBER",
  "status": "delivered",
  "status_text": "DATE Delivered",
  "terminal": true,
  "delivered": true,
  "last_event": {
    "time": "DATE",
    "text": "Delivered",
    "location": "CITY, REGION"
  }
}
```

## Status mapping

Rendered text fallback:

- delivered: `Delivered`, `left at`, proof-of-delivery text
- out for delivery: `Out for Delivery`
- in transit: `On the Way`, `In Transit`, `Departed`, `Arrived`, `Processing at UPS Facility`
- pre-advice: `Label Created`, `Shipment Ready for UPS`, UPS has not received the package
- delivery attempted: `Delivery Attempted`, `We missed you`, attempted text
- ready for pickup: `Ready for Pickup`, `available for pickup`, `Access Point`
- exception: `Exception`, `Clearance`, address/action-required text
- delayed: `Delayed`, `Delay`
- returned: `Return to Sender`, `Returned`
- not found: `We could not locate`, `Tracking Information Not Found`, invalid tracking text

Official UPS API code hints, if an authenticated adapter is added later:

- `011`: delivered
- `021`: in transit
- `003`: order processed / label created
- `005`: shipped / picked up
- `007`: exception

## Official UPS API

UPS’s official Tracking API is OAuth-based and not used by default:

```txt
https://onlinetools.ups.com/api/track/v1/details/{inquiryNumber}
```

It requires a UPS developer app, OAuth token, `transId`, and `transactionSrc`. Keep that as a separate explicit mode if ever needed.

## Privacy

No UPS credentials are stored. Do not request proof-of-delivery images/signatures unless explicitly asked; UPS treats those as sensitive. Tracking numbers stay local unless an approved aggregator mode is deliberately added later.
