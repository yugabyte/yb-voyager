#!/usr/bin/env python3

import yaml

def generate_yaml(num_tables=200):
    # Define the base configuration
    config = {
        "file_table_map": "",
        "additional_flags": {
            "--delimiter": "\\t",
            "--format": "text"
        },
        "row_count": {},
        "resumption": {
            "max_retries": 3,
            "min_interrupt_seconds": 30,
            "max_interrupt_seconds": 120,
            "min_restart_wait_seconds": 15,
            "max_restart_wait_seconds": 45
        }
    }

    # Generate table mappings
    table_mappings = []
    for i in range(1, num_tables + 1):
        table_name = f"smsa{i}"
        table_mappings.append(f"SMSA.txt:{table_name}")
        config["row_count"][table_name] = 60  # Default row count for each table

    # Join the table mappings into the file_table_map field
    config["file_table_map"] = ",".join(table_mappings)

    # Generate YAML content
    yaml_content = yaml.dump(config, default_flow_style=False)

    # Write the YAML content to a file
    with open("config.yaml", "w") as yaml_file:
        yaml_file.write(yaml_content)

    print("YAML file generated successfully.")

# Example usage: Generate for 200 tables (you can adjust the number)
generate_yaml(200)
