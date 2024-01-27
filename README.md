# Integration Tests

These tests build a proper stack of the point-c software and verify that it runs correctly.

## Tests

### Download

Downloads different amounts of random data. Data is downloaded directly from caddy and from the VPN and the amount of time to download each is printed.

### Speedtest

[`librespeed`](https://github.com/librespeed/speedtest) is leveraged to benchmark `point-c`'s speed. A direct connection to Caddy and a VPN connection are benchmarked.

#### Results

```bash
$ uname -a
Linux laptop 6.1.0-16-amd64 #1 SMP PREEMPT_DYNAMIC Debian 6.1.67-1 (2023-12-12) x86_64 GNU/Linux
$ 
```