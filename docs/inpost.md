# InPost

InPost is supported through the public ShipX tracking API. No postcode, browser, or InPost account credentials are required by default.

```sh
parcelcli track TRACKING_NUMBER --carrier inpost --json
```

## Method

The adapter calls:

```text
https://api-shipx-pl.easypack24.net/v1/tracking/<TRACKING_NUMBER>
```

It normalizes the response into the shared parcelcli result shape:

- root shipment `status`
- `tracking_details[]` as events
- latest event by timestamp where available
- terminal/delivered/delayed flags

The InPost UK website route `/tracking/api` is not used because it is protected by Cloudflare Turnstile. Browser automation can load the public page, but the tracking card is not a reliable headless data source.

## Status Mapping

- `created`, `confirmed`, `offer_*` -> `pre_advice`
- accepted/sent/collected/courier branch statuses -> `in_transit`
- `ready_to_pickup*` -> `ready_for_pickup`
- `out_for_delivery*` -> `out_for_delivery`
- `delivered` -> `delivered`
- return-to-sender and pickup-expired statuses -> `returned`
- undelivered attempt statuses -> `delivery_attempted`
- `delay_in_delivery` -> `delayed`
- canceled, rejected, missing, wrong-address, oversized, and claim statuses -> `exception`

## Notes

- The API returns `not_found` for tracking numbers outside its accessible dataset or retention window.
- InPost's newer authenticated group APIs require OAuth credentials with `api:tracking:read`; parcelcli does not store or request those credentials.
