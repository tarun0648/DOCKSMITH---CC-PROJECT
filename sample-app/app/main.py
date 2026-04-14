#!/usr/bin/env python3
"""
Docksmith sample app - demonstrates all Docksmithfile instructions.
This app prints environment info and a simple greeting, then exits cleanly.
"""
import os
import sys
import json

def main():
    greeting = os.environ.get("GREETING", "Hello")
    app_name = os.environ.get("APP_NAME", "Docksmith")
    run_mode = os.environ.get("RUN_MODE", "default")

    print("=" * 50)
    print(f"  {greeting}, from {app_name}!")
    print("=" * 50)
    print(f"  Run mode : {run_mode}")
    print(f"  Python   : {sys.version.split()[0]}")
    print(f"  WorkDir  : {os.getcwd()}")
    print()

    # Show all environment variables (sorted)
    print("  Environment variables:")
    for k, v in sorted(os.environ.items()):
        if k in ("GREETING", "APP_NAME", "RUN_MODE"):
            print(f"    {k}={v}")

    print()
    print("  Files in working directory:")
    try:
        for f in sorted(os.listdir(".")):
            print(f"    {f}")
    except Exception as e:
        print(f"    (error listing: {e})")

    print()
    print("  Container isolation check:")
    # Try to write a file inside the container
    try:
        with open("/tmp/container-proof.txt", "w") as f:
            f.write("This file was created inside the container.\n")
        print("  /tmp/container-proof.txt written inside container (should NOT appear on host)")
    except Exception as e:
        print(f"  Could not write test file: {e}")

    print()
    print("  Docksmith sample app completed successfully.")
    print("=" * 50)

if __name__ == "__main__":
    main()
