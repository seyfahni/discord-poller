# Use MySQL behind a storage interface

The hosted service will use MySQL for durable multi-server data, with storage hidden behind a small application-facing interface so other implementations can be added later. This keeps the initial production choice explicit while requiring an in-memory test adapter to verify the seam; the trade-off is maintaining behavioral consistency across adapters.
