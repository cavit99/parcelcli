# InPost

InPost UK is supported through the public `tracking.inpost.co.uk` tracking API. No postcode, browser, or InPost account credentials are required by default.

```sh
parcelcli track TRACKING_NUMBER --carrier inpost --json
```

## Method

The adapter calls:

```text
https://tracking.inpost.co.uk/api/v2.0/<TRACKING_NUMBER>
```

It normalizes the response into the shared parcelcli result shape:

- tracking events keyed by consignment number
- latest event by timestamp where available
- terminal/delivered/delayed flags

The newer InPost UK website route `/tracking/api` is not used because it is protected by Cloudflare Turnstile. Browser automation can load the public page, but the tracking card is not a reliable headless data source.

## Status Mapping

- `PRD` / `Ready For Dispatch` -> `pre_advice`
- `PSC` / `Parcel Stored by Customer` -> `accepted`
- sent/collected/courier/in-transit descriptions -> `in_transit`
- `ready_to_pickup*` -> `ready_for_pickup`
- `out_for_delivery*` -> `out_for_delivery`
- delivered event codes/descriptions -> `delivered`
- return-to-sender and pickup-expired statuses -> `returned`
- undelivered attempt statuses -> `delivery_attempted`
- delay descriptions -> `delayed`
- canceled, rejected, missing, wrong-address, oversized, and claim descriptions -> `exception`

## Notes

- The API returns `not_found` for tracking numbers outside its accessible dataset or retention window.
- InPost's newer authenticated group APIs require OAuth credentials with `api:tracking:read`; parcelcli does not store or request those credentials.
