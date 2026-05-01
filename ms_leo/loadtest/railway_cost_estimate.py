#!/usr/bin/env python3
"""
Simple Railway cost estimator for 24h load tests.

Inputs:
  --avg-cpu           average vCPU used during test (e.g. 0.85)
  --avg-ram-gb        average RAM in GB used during test (e.g. 1.2)
  --hours             test hours, default 24
  --cpu-price         USD per vCPU-hour
  --ram-price         USD per GB-hour
"""

from __future__ import annotations

import argparse


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Estimate Railway spend from CPU and RAM averages")
    parser.add_argument("--avg-cpu", type=float, required=True, help="Average used vCPU")
    parser.add_argument("--avg-ram-gb", type=float, required=True, help="Average used RAM (GB)")
    parser.add_argument("--hours", type=float, default=24.0, help="Test duration in hours")
    parser.add_argument(
        "--cpu-price",
        type=float,
        default=0.000463,
        help="USD per vCPU-hour (set your Railway plan value)",
    )
    parser.add_argument(
        "--ram-price",
        type=float,
        default=0.000231,
        help="USD per GB-hour (set your Railway plan value)",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    cpu_cost = args.avg_cpu * args.hours * args.cpu_price
    ram_cost = args.avg_ram_gb * args.hours * args.ram_price
    total = cpu_cost + ram_cost

    print("Railway load-test cost estimate")
    print("--------------------------------")
    print(f"Duration (hours):        {args.hours:.2f}")
    print(f"Avg CPU (vCPU):          {args.avg_cpu:.4f}")
    print(f"Avg RAM (GB):            {args.avg_ram_gb:.4f}")
    print(f"CPU price ($/vCPU-hour): {args.cpu_price:.9f}")
    print(f"RAM price ($/GB-hour):   {args.ram_price:.9f}")
    print("--------------------------------")
    print(f"CPU cost (USD):          {cpu_cost:.6f}")
    print(f"RAM cost (USD):          {ram_cost:.6f}")
    print(f"Total cost (USD):        {total:.6f}")


if __name__ == "__main__":
    main()
