# Integration Tests

These tests build a Docker stack using Caddy and the [`point-c`](https://github.com/point-c/caddy) module to verify that it runs correctly.

## Tests

### Download

Downloads different amounts of random data. Data is downloaded directly from caddy and from the VPN and the amount of time to download each is printed.

### Speedtest

[`librespeed`](https://github.com/librespeed/speedtest) is leveraged to benchmark `point-c`'s speed. A direct connection to Caddy and a VPN connection are benchmarked.

## Usage

```bash
sudo go test -v ./tests/...
```