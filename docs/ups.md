# UPS adapter research

Status: researched, ready to implement as browser-backed public tracking.

UPS has both an official OAuth API and a public web tracking flow. For `parcelcli`, the default should be the public browser flow: no UPS account credentials, no third-party aggregator, local-only tracking state.

## Public tracking page

Main page:

- `https://www.ups.com/track?loc=en_US`

Deep-link shape:

- `https://www.ups.com/track?loc=en_US&tracknum=<TRACKING_NUMBER>&requester=ST/trackdetails`

Direct plain HTTP is not enough for reliable automation: the site uses Akamai/bot checks, invisible reCAPTCHA/fingerprint endpoints, and browser context. But headless Chrome can load the page and render tracking status.

Local probes on 2026-05-06:

### `TRACKING_NUMBER`

Rendered result:

- status: `Delivered`
- latest update: `DATE Delivered`
- delivered to: `CITY, REGION`
- received by: `RECIPIENT`
- proof of delivery link present

### `1Z999AA10123456784`

Rendered result:

- status: `Delivered`
- latest update: `No Information Available`
- delivered to: `LONGVIEW, TX US`
- received by: `TAYLOR`

## Public web API behind the page

The page calls:

- `OPTIONS https://webapis.ups.com/track/api/Track/GetStatus?loc=en_US`
- `POST https://webapis.ups.com/track/api/Track/GetStatus?loc=en_US`

Observed request body is base64-looking in Chrome DevTools protocol `PostDataEntries`, but decodes to JSON:

```json
{
  "Locale": "en_US",
  "TrackingNumber": ["1z52w87a0430663188"],
  "isBarcodeScanned": false,
  "Requester": "st",
  "ClientUrl": "https://www.ups.com/track?loc=en_US&tracknum=TRACKING_NUMBER&requester=ST/trackdetails",
  "returnToValue": "",
  "AssociatedBcdnNumber": null
}
```

Direct `curl` to `https://webapis.ups.com/track/api/Track/GetStatus?loc=en_US` from this environment hung/was killed, even with JSON body and origin/referer headers. So the implementation should not depend on direct HTTP unless later proven stable. Use Chrome/CDP and parse either response JSON if accessible or rendered text if not.

CDP `network.GetResponseBody` returned empty for the `GetStatus` POST during probes, but the rendered DOM had the useful status text. The adapter should treat rendered DOM parsing as first-class rather than as a weak fallback.

## Official UPS Tracking API

Official docs live in `UPS-API/api-documentation`, especially:

- `https://github.com/UPS-API/api-documentation/blob/main/Tracking.yaml`

Production endpoint:

- `https://onlinetools.ups.com/api/track/v1/details/{inquiryNumber}`

Auth:

- OAuth2 client credentials
- token endpoint in docs: `https://wwwcie.ups.com/security/v1/oauth/token` for test; production via `onlinetools.ups.com`/UPS developer portal app credentials

Required request headers:

- `transId`
- `transactionSrc`
- `Authorization: Bearer <token>`

Useful official response fields:

- `trackResponse.shipment[].package[].currentStatus.code`
- `trackResponse.shipment[].package[].currentStatus.description`
- `trackResponse.shipment[].package[].currentStatus.simplifiedTextDescription`
- `deliveryDate[].date`, type `SDD`, `RDD`, `DEL`
- `activity[]` events with status/location/date/time
- `deliveryInformation.location`, `receivedBy`, `signature`, `pod`, `deliveryPhoto`
- warnings such as `TW0001` / `Tracking Information Not Found` may arrive under HTTP 200

Open-source packages found (`ripe-tech/ups-api-js`, `anatelli10/ts-shipment-tracking`, old PHP packages) all use official UPS credentials or third-party aggregators. They are useful for response modelling and status mapping, not for credentialless tracking.

## Recommended `parcelcli` design

Add `internal/carriers/ups` with a browser-backed tracker:

1. Normalize tracking number to uppercase and strip spaces.
2. Navigate to `https://www.ups.com/track?loc=en_US&tracknum=<TRACKING_NUMBER>&requester=ST/trackdetails`.
3. Optionally click cookie banner (`Essential Cookies Only` is fine) if it blocks content.
4. Wait for one of:
   - tracking number appears in body
   - `Tracking Details`
   - `Delivered`
   - `On the Way`
   - `Out for Delivery`
   - `Exception`
   - `We could not locate`
5. Capture observations for `webapis.ups.com/track/api/Track/GetStatus`.
6. Parse rendered body text into `model.Result`.
7. If future CDP runs expose JSON response body, normalize JSON first and use rendered text as fallback.

Expected command shape:

```sh
parcelcli track TRACKING_NUMBER --carrier ups --json
```

No postcode should be required.

## Rendered text parsing targets

From body lines, useful anchors:

- `Tracking Details`
- status line after it: `Delivered`, `On the Way`, `Out for Delivery`, etc.
- latest update line after `Latest Update`, or in delivered pages a line like `DATE Delivered`
- `Delivered To` followed by location line
- `Received By` followed by recipient line
- `Proof of Delivery` present for delivered parcels

For `TRACKING_NUMBER`, a correct normalized result should look roughly like:

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
  },
  "raw": {
    "delivered_to": "CITY, REGION",
    "received_by": "RECIPIENT"
  }
}
```

## Status mapping

Text/status fallback:

- delivered: `Delivered`, text containing `delivered`, `left at`, proof of delivery present
- out for delivery: `Out for Delivery`, `out for delivery today`
- in transit: `On the Way`, `In Transit`, `Departed`, `Arrived`, `Processing at UPS Facility`
- accepted/pre-advice: `Label Created`, `Shipment Ready for UPS`, `UPS has not received the package yet`
- delivery attempted: `Delivery Attempted`, `We missed you`, `attempted`
- ready for pickup: `Ready for Pickup`, `available for pickup`, `Access Point`
- exception: `Exception`, `Clearance`, `weather delay`, `address information required`, `action required`
- delayed: `Delayed`, `Delay`
- returned: `Return to Sender`, `Returned`
- not_found: `We could not locate`, `Tracking Information Not Found`, `invalid tracking number`

Official API code hints from docs/examples:

- `011`: delivered
- `021`: in transit
- `003`: order processed / label created
- `005`: shipped / picked up
- `007`: exception

## Detection

UPS 1Z tracking numbers are easy to detect:

- starts with `1Z`
- usually 18 chars total
- uppercase alphanumeric

Also UPS can track other inquiry numbers/reference numbers, but `parcelcli detect` should only mark those as `possible`, not confident.

## Privacy notes

- No UPS credentials by default.
- Do not request proof-of-delivery images/signatures unless explicitly asked; official docs treat signature/POD/photo as sensitive.
- Keep watch state local.
- No TrackingMore/AfterShip/etc. unless explicitly configured and approved.
