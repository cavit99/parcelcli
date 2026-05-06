# Evri adapter

Evri is supported through its public tracking page using headless Chrome / CDP.

## Inputs

- Tracking number: Evri says public tracking accepts 16-character parcel tracking codes or 8-digit calling-card numbers.
- Postcode: optional; rough public tracking works without it, detailed parcel page tracking can include it.

## Method

The adapter opens:

```txt
https://www.evri.com/track/parcel/<tracking>/details
https://www.evri.com/track/parcel/<tracking>/details?postcode=<POSTCODE>
```

using headless Chrome via Chrome DevTools Protocol. Plain HTTP is unreliable because the public site uses browser-only frontend keys and WAF-protected platform API calls.

The adapter waits for rendered page text such as:

- `Your parcel from`
- `Update on your parcel`
- `Barcode number`
- `delayed`
- `delivered`

Then it extracts:

- status text,
- sender line when visible,
- latest event time/text,
- event rows,
- observed Evri API calls for debugging.

## Privacy

Tracking number stays local. Postcode is only sent to Evri when explicitly provided. Debug output should not be shared casually because it can contain parcel metadata.
