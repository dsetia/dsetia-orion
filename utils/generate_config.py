#!/usr/bin/env python3
"""
Configuration Generator for Orion

Generates environment-specific configuration files by merging base templates
with environment-specific overrides.

Directory structure:
  config/base/          - Base configuration templates (version controlled)
  config/environments/  - Environment-specific overrides (version controlled)
  config/generated/     - Generated configs (gitignored, created at runtime)
"""

import json
import sys
import argparse
from pathlib import Path


def merge_configs(base, override):
    """
    Deep merge override dictionary into base configuration.

    Args:
        base: Base configuration dictionary
        override: Override values to merge in

    Returns:
        Merged configuration dictionary
    """
    result = base.copy()
    for key, value in override.items():
        if isinstance(value, dict) and key in result and isinstance(result[key], dict):
            result[key] = merge_configs(result[key], value)
        else:
            result[key] = value
    return result


def list_available_environments():
    """List all available environment configuration files."""
    env_dir = Path("config/environments")
    if not env_dir.exists():
        return []
    return [f.stem for f in env_dir.glob("*.json")]


def generate_config(env, verbose=False):
    """
    Generate configuration files for the specified environment.

    Args:
        env: Environment name (e.g., 'dev', 'staging', 'production')
        verbose: Print detailed output
    """
    base_dir = Path("config/base")
    env_file = Path(f"config/environments/{env}.json")
    output_dir = Path("config/generated")

    # Validate inputs
    if not base_dir.exists():
        print(f"Error: Base configuration directory not found: {base_dir}", file=sys.stderr)
        sys.exit(1)

    if not env_file.exists():
        available = list_available_environments()
        print(f"Error: Environment configuration not found: {env_file}", file=sys.stderr)
        if available:
            print(f"Available environments: {', '.join(available)}", file=sys.stderr)
        sys.exit(1)

    # Create output directory
    output_dir.mkdir(exist_ok=True)

    # Load environment overrides
    if verbose:
        print(f"Loading environment config: {env_file}")
    with open(env_file) as f:
        env_config = json.load(f)

    # Generate each config file
    generated_count = 0
    for base_file in sorted(base_dir.glob("*.json")):
        with open(base_file) as f:
            base_config = json.load(f)

        config_name = base_file.stem
        overrides = env_config.get(config_name, {})

        final_config = merge_configs(base_config, overrides)

        output_file = output_dir / base_file.name
        with open(output_file, 'w') as f:
            json.dump(final_config, f, indent=2)

        generated_count += 1
        if verbose:
            print(f"  Generated: {output_file}")
            if overrides:
                print(f"    Applied overrides: {list(overrides.keys())}")
        else:
            print(f"Generated {output_file}")

    if verbose:
        print(f"\nSuccessfully generated {generated_count} configuration file(s) for '{env}' environment")


def main():
    parser = argparse.ArgumentParser(
        description="Generate environment-specific configuration files for Orion",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s staging              Generate configs for staging environment
  %(prog)s production -v        Generate configs for production with verbose output
  %(prog)s --list               List available environments

Configuration Structure:
  config/base/db.json           Base database configuration template
  config/base/minio.json        Base MinIO configuration template
  config/environments/staging.json   Environment-specific overrides
  config/generated/             Output directory (gitignored)
        """
    )

    parser.add_argument(
        'environment',
        nargs='?',
        help='Target environment (e.g., dev, staging, production)'
    )

    parser.add_argument(
        '-v', '--verbose',
        action='store_true',
        help='Enable verbose output'
    )

    parser.add_argument(
        '-l', '--list',
        action='store_true',
        help='List available environments and exit'
    )

    args = parser.parse_args()

    # Handle --list flag
    if args.list:
        available = list_available_environments()
        if available:
            print("Available environments:")
            for env in available:
                print(f"  - {env}")
        else:
            print("No environment configurations found in config/environments/")
        sys.exit(0)

    # Require environment argument if not listing
    if not args.environment:
        parser.print_help()
        sys.exit(1)

    generate_config(args.environment, args.verbose)


if __name__ == "__main__":
    main()
