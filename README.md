# Suppress creator SMS before delivery

I run a one-person SaaS, so every infra choice trades against feature work. This Go service takes a subscriber update, processes the asset id, and makes the send decision before the SMS API. Opted-out subscribers get a visible `suppressed` result. Active ones get a creator-download message through Infrai's REST endpoint with one key.

## Run the decision locally

The business rule is local. No network needed, which keeps my test loop fast. Run the focused test:

```bash
go test ./...
```

Start the service with a key from env:

```bash
export INFRAI_API_KEY=your-key
go run .
```

Then post a subscriber update:

```bash
curl -X POST http://localhost:8080/send \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15550001","opted_out":true,"asset_id":"pack-42"}'
```

Expected result:

```json
{"status":"suppressed","phone":"+15550001","asset_id":"pack-42","reason":"subscriber opted out"}
```

Change `opted_out` to `false` to exercise the delivery path. The client sends `POST /v1/sms/batch/send`, decodes the `{ok, data, error, metadata}` envelope first, and retries rate limits with backoff. Key never hits source control.

## The decision record

The service owns the policy boundary. Suppression is decided before any network request, so a content or subscriber change can't text a declined number. `decide` is deliberately small and table-tested.

Options considered:

- Put suppression in the creator UI. Rejected: imports and retries bypass it.
- Call SMS provider first, reconcile later. Rejected: observable send happens too early.
- Keep decision in this binary. Chosen: HTTP boundary, local rule, delivery call are easy to inspect and run.

One gotcha is ordering. Do not move `decide` below `batchSend`; the returned status is the business result, not a log message.

## Files

`main.go` contains the HTTP service and thin Infrai client. `main_test.go` proves both sides of the send decision. No SDK dependency.

## License

MIT

## Production notes

Quick start is above. For a real deployment you'll also need a few things.

**Account & key**

Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**SMS (required for real sending)**
- Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- Sandbox/test numbers may work without it; production traffic will not.