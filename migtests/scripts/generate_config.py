#!/usr/bin/env python3

import os
import argparse
from jinja2 import Template, Environment, meta
import yaml

def render_template(template_path, env_vars):
    with open(template_path) as f:
        content = f.read()
    env = Environment()
    ast = env.parse(content)
    required_vars = meta.find_undeclared_variables(ast)

    missing = [v for v in required_vars if v not in env_vars]
    if missing:
        print(f"Warning: Missing environment variables: {missing}")

    template = Template(content)
    return template.render(**env_vars)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--template", required=True, help="Path to template config")
    parser.add_argument("--output", required=True, help="Path to write filled config")
    args = parser.parse_args()

    rendered = render_template(args.template, os.environ)

    # Optional: validate YAML syntax
    try:
        yaml.safe_load(rendered)
    except yaml.YAMLError as e:
        print("ERROR: Invalid YAML after rendering")
        print(e)
        exit(1)

    with open(args.output, "w") as f:
        f.write(rendered)
    print(f"✅ Config generated at {args.output}")

if __name__ == "__main__":
    main()
