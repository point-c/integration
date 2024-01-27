# Integration Tests for `point-c`

This suite contains integration tests designed to validate the functionality and performance of the [`point-c`](https://github.com/point-c/caddy) module within a Dockerized Caddy environment. These tests ensure that `point-c` works as expected under various conditions, using Caddy as a web server.

## Prerequisites

Before running the tests, ensure you have the following installed:
- Docker
- Go (1.21 or newer)

## Tests Overview

### Download Test

This test evaluates the data transfer capabilities of `point-c` by downloading varying amounts of random data through both direct Caddy connections and VPN-tunneled connections. The test measures and outputs the time taken for each download, providing insights into the throughput and efficiency of data transfer.
### Speedtest

Utilizing the [`librespeed`](https://github.com/librespeed/speedtest) tool, this test benchmarks the network speed of `point-c`. It compares the speed of a direct connection to Caddy with that of a connection routed through the VPN, helping to quantify the performance impact of `point-c`.
A table is printed after the test with the results.

## Usage Instructions

1. **Prepare the Environment**: Ensure Docker and Go are correctly installed and configured on your system.

2. **Clone the Repository**: Clone the repository containing the integration tests to your local machine.

3. **Navigate to the Test Directory**: Change into the directory containing the tests.

4. **Run the Tests**: Execute the following command to start the integration tests:

```bash
sudo go test -v ./tests/...
```

The -v flag provides verbose output, allowing you to see the progress and results of each test.