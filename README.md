# aave-cap-alerts

A small Go daemon that watches the `scaledTotalSupply` reported by Aave v3 aTokens and pushes notifications when the configured triggers fire. Use it to catch liquidity increases or decreases (for example, when the scaled supply jumps from 100M to 200M units) right when they happen on-chain.

## Features
- Calls each configured aToken's `scaledTotalSupply` function through your own RPC endpoint
- Supports multiple assets with per-token polling cadence and optional target thresholds
- Sends alerts to Telegram and/or a custom JSON-RPC callback
- Emits structured logs so you can pipe the output elsewhere if you prefer

## Quick start
1. Copy `config.example.yaml` to `config.yaml` and fill in the placeholders:
   - `rpc_url`: your Ethereum (or supported network) RPC endpoint
   - `assets`: one entry per token to monitor (aToken address, optional target threshold, trigger preferences)
   - `notifications`: provide your Telegram bot token/chat ID and/or JSON-RPC endpoint details
2. Fetch dependencies: `go mod tidy`
3. Run the monitor: `go run ./cmd/aave-cap-alerts --config config.yaml`

By default the service polls every minute. You can change the global cadence with `poll_interval` at the top level of the config, or override it per asset.

### Telegram alerts
Create a bot with [BotFather](https://core.telegram.org/bots) and grab the chat ID you want to notify. The message payload includes the asset, old/new scaled supplies (raw integers) and the reasons that fired the alert (e.g. "scaled supply increased" or "target reached").

### Custom JSON-RPC callback
If you provide a JSON endpoint the service will POST a simple body such as:
```json
{
  "message": "asset USDe scaled supply changed: 1234567890 -> 1334567890"
}
```
Parse the message however you prefer on the receiving side.

## Notes
- Scaled supplies are reported as raw integers exactly as they are stored on-chain; apply any scaling (e.g., ray math) in your downstream system if you need base units.
- Keep an eye on RPC rate limits—each asset poll performs one `scaledTotalSupply` call and caches token decimals after the first lookup.
- For production you may want to run the binary under a process supervisor and point logs to your observability stack.

Happy monitoring!
