import os
import sys
from pathlib import Path
import subprocess
import argparse
import shutil
import difflib

command_context = {}
error_context = {}

# Function to source environment variables from a file
def source_env(env_file):
    env = {}
    try:
        result = subprocess.run(['bash', '-c', f'source {env_file} && env'], stdout=subprocess.PIPE, check=True)
        for line in result.stdout.decode('utf-8').splitlines():
            if line.strip() and '=' in line:
                key, value = line.split('=', 1)
                env[key.strip()] = value.strip()
    except subprocess.CalledProcessError as e:
        print(f"Error sourcing environment variables from {env_file}: {e}")
        sys.exit(1)
    return env

# Helper function to run commands
def run_command(command, success_message, error_message):
    try:
        proc = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
        print(success_message)
        return True, proc.stdout.decode('utf-8')
    except subprocess.CalledProcessError as e:
        print(f"{error_message}: {e.stderr.decode('utf-8')}")
        return False, e.stderr.decode('utf-8')

# Function to run the migration assessment command
def run_assess_migration(export_dir, env):
    print("Running migration assessment...")
    command = [
        'yb-voyager', 'assess-migration',
        '--export-dir', str(export_dir),
        '--source-db-type', env.get('SOURCE_DB_TYPE', ''),
        '--source-db-host', env.get('SOURCE_DB_HOST', ''),
        '--source-db-user', env.get('SOURCE_DB_USER', ''),
        '--source-db-schema', env.get('SOURCE_DB_SCHEMA', ''),
        '--source-db-password', env.get('SOURCE_DB_PASSWORD', ''),
        '--source-db-name', env.get('SOURCE_DB_NAME', ''),
        '--start-clean', 't',
        '--yes', '--iops-capture-interval', '5'
    ]
    command_context['assess_migration_command'] = ' '.join(command)
    success, output = run_command(command, "Migration assessment completed successfully.", "Error running migration assessment")
    if not success:
        error_context['assess_migration_error'] = output
    return success

# Function to export schema
def export_schema(export_dir, env):
    print("Exporting schema...")
    source_db_schema = env.get('SOURCE_DB_SCHEMA', '')
    if env.get('SOURCE_DB_TYPE', '') == 'mysql':
        source_db_schema = ''
    command = [
        'yb-voyager', 'export', 'schema',
        '--export-dir', str(export_dir),
        '--source-db-type', env.get('SOURCE_DB_TYPE', ''),
        '--source-db-host', env.get('SOURCE_DB_HOST', ''),
        '--source-db-user', env.get('SOURCE_DB_USER', ''),
        '--source-db-password', env.get('SOURCE_DB_PASSWORD', ''),
        '--source-db-name', env.get('SOURCE_DB_NAME', ''),
        '--start-clean', 't',
        '--yes'
    ]
    if source_db_schema:
        command.extend(['--source-db-schema', source_db_schema])
    command_context['export_schema_command'] = ' '.join(command)
    success, output = run_command(command, "Schema export completed successfully.", "Error exporting schema")
    if not success:
        error_context['export_schema_error'] = output
    return success

# Function to run user-provided scripts
def run_user_scripts(scripts):
    executed_scripts = []
    for script in scripts:
        print(f"Running user-provided script: {script}")
        script_path = Path(script)
        try:
            if script_path.suffix in ['.sh', '.bash']:
                subprocess.run(['bash', str(script_path)], check=True)
            elif script_path.suffix in ['.py']:
                subprocess.run([sys.executable, str(script_path)], check=True)
            else:
                print(f"Unsupported script type: {script_path.suffix}")
                return False
            executed_scripts.append(script)
            print(f"User-provided script '{script}' executed successfully.")
        except subprocess.CalledProcessError as e:
            print(f"Error running user-provided script '{script}': {e}")
            return False
    return executed_scripts

# Function to import schema
def import_schema(export_dir, env):
    print("Importing schema...", end='', flush=True)
    command = [
        'yb-voyager', 'import', 'schema',
        '--export-dir', str(export_dir),
        '--target-db-host', env.get('TARGET_DB_HOST', ''),
        '--target-db-user', env.get('TARGET_DB_USER', ''),
        '--target-db-password', env.get('TARGET_DB_PASSWORD', ''),
        '--target-db-name', env.get('TARGET_DB_NAME', ''),
        '--start-clean', 't',
        '--yes',
        '--continue-on-error', 'true'
    ]
    command_context['import_schema_command'] = ' '.join(command)
    success, output = run_command(command, "", "Error importing schema")
    if not success:
        error_context['import_schema_error'] = output
    failed_sql_file = export_dir / 'schema' / 'failed.sql'
    if success and not failed_sql_file.exists():
        print("\rSchema import completed successfully.")
        return True
    elif failed_sql_file.exists():
        print("\rImport schema failed. 'failed.sql' file found.")
        return False
    else:
        print("\rImport schema failed.")
        return False

# Function to analyze schema
def analyze_schema(export_dir):
    print("Analyzing schema...")
    command = [
        'yb-voyager', 'analyze-schema',
        '--export-dir', str(export_dir),
        '--output-format', 'html'
    ]
    command_context['analyze_schema_command'] = ' '.join(command)
    success, output = run_command(command, "Schema analysis completed successfully.", "Error analyzing schema")
    if not success:
        error_context['analyze_schema_error'] = output
    return success

# Function to export data
def export_data(export_dir, env):
    print("Exporting data...")
    command = [
        'yb-voyager', 'export', 'data',
        '--export-dir', str(export_dir),
        '--source-db-type', env.get('SOURCE_DB_TYPE', ''),
        '--source-db-host', env.get('SOURCE_DB_HOST', ''),
        '--source-db-user', env.get('SOURCE_DB_USER', ''),
        '--source-db-schema', env.get('SOURCE_DB_SCHEMA', ''),
        '--source-db-password', env.get('SOURCE_DB_PASSWORD', ''),
        '--source-db-name', env.get('SOURCE_DB_NAME', ''),
        '--start-clean', 't',
        '--yes'
    ]
    command_context['export_data_command'] = ' '.join(command)
    success, output = run_command(command, "Data export completed successfully.\nRun export data status to see the exported row counts.", "Error exporting data")
    if not success:
        error_context['export_data_error'] = output
    return success

# Function to import data
def import_data(export_dir, env):
    print("Importing data...")
    command = [
        'yb-voyager', 'import', 'data',
        '--export-dir', str(export_dir),
        '--target-db-host', env.get('TARGET_DB_HOST', ''),
        '--target-db-user', env.get('TARGET_DB_USER', ''),
        '--target-db-password', env.get('TARGET_DB_PASSWORD', ''),
        '--target-db-name', env.get('TARGET_DB_NAME', ''),
        '--start-clean', 't',
        '--yes'
    ]
    command_context['import_data_command'] = ' '.join(command)
    success, output = run_command(command, "Data import completed successfully.\nRun import data status to see the imported row counts.", "Error importing data")
    if not success:
        error_context['import_data_error'] = output
    return success

# Function to generate HTML report
def generate_report(export_dir, assess_migration_success, export_schema_success, analyze_schema_success, import_schema_success, export_data_success, import_data_success, user_scripts_executed, replace_files_executed, reports_dir, env):
    print("Generating final report...")

    # Assess Migration status
    assess_migration_status = "Successful &#10004;" if assess_migration_success else f"Failed &#10060;<br><br>Error: {error_context.get('assess_migration_error', '')}"
    assess_migration_command = command_context.get('assess_migration_command', [])

    # Export Schema status
    export_schema_status = "Successful &#10004;" if export_schema_success else f"Failed &#10060;<br><br>Error: {error_context.get('export_schema_error', '')}"
    export_schema_command = command_context.get('export_schema_command', [])

    # Analyze Schema status
    analyze_schema_status = "Successful &#10004;" if analyze_schema_success else f"Failed &#10060;<br><br>Error: {error_context.get('analyze_schema_error', '')}"
    analyze_schema_command = command_context.get('analyze_schema_command', [])

    # Import Schema status
    import_schema_status = "Successful &#10004;" if import_schema_success else f"Failed &#10060;<br><br>Error: {error_context.get('import_schema_error', '')}"
    import_schema_command = command_context.get('import_schema_command', [])

    # Export Data status
    export_data_status = "Successful &#10004;" if export_data_success else f"Skipped or Failed<br><br>Error: {error_context.get('export_data_error', '')}"
    export_data_command = command_context.get('export_data_command', [])

    # Import Data status
    import_data_status = "Successful &#10004;" if import_data_success else f"Skipped or Failed<br><br>Error: {error_context.get('import_data_error', '')}"
    import_data_command = command_context.get('import_data_command', [])

    # Load assessment report content if available
    assessment_report_content = ""
    assessment_report_file = export_dir / 'assessment' / 'reports' / 'assessmentReport.html'
    if assess_migration_success and assessment_report_file.exists():
        with open(assessment_report_file, 'r') as f:
            assessment_report_content = f.read()

    # Determine source database schema
    source_db_schema = env.get('SOURCE_DB_SCHEMA', '')
    if env.get('SOURCE_DB_TYPE', '') == 'mysql':
        source_db_schema = ''

    # Load schema analysis report content if available
    schema_analysis_report_content = ""
    schema_analysis_report_file = export_dir / 'reports' / 'schema_analysis_report.html'
    if analyze_schema_success and schema_analysis_report_file.exists():
        with open(schema_analysis_report_file, 'r') as f:
            schema_analysis_report_content = f.read()

    # Determine failed SQL content if exists
    failed_sql_content = ""
    failed_sql_file = export_dir / 'schema' / 'failed.sql'
    if not import_schema_success and failed_sql_file.exists():
        with open(failed_sql_file, 'r') as f:
            failed_sql_content = f.read()
    
    diff_content = ""
    for old_file, paths in replace_files_executed.items():
        old_path = Path(paths['old_file'])
        new_path = Path(paths['new_file'])
        backup_path = Path(paths['backup_file'])

        with open(backup_path, 'r', encoding='utf-8') as backup_f, open(old_path, 'r', encoding='utf-8') as old_f:
            old_lines = old_f.readlines()
            backup_lines = backup_f.readlines()

            # Calculate diff
            diff = difflib.unified_diff(backup_lines, old_lines, fromfile=str(backup_path), tofile=str(old_path))
            diff_content += "\n".join(diff) + "\n"

    # HTML report content
    html_report_content = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <title>Summary Report</title>
        <style>
            body {{ font-family: Arial, sans-serif; }}
            h1 {{ color: #333; }}
            pre {{
                background-color: #f4f4f4;
                padding: 10px;
                border: 1px solid #ddd;
                white-space: pre-wrap; /* Preserve line breaks and wrap long lines */
                word-wrap: break-word; /* Wrap long words */
                font-size: 14px; /* Adjust font size */
            }}
            .summary {{
                font-weight: bold;
            }}
            .collapsible {{
                background-color: #f9f9f9;
                color: #444;
                cursor: pointer;
                padding: 10px;
                width: 100%;
                border: none;
                text-align: left;
                outline: none;
                font-size: 15px;
            }}
            .active, .collapsible:hover {{
                background-color: #ccc;
            }}
            .content {{
                padding: 0 18px;
                display: none;
                overflow: hidden;
                background-color: #f9f9f9;
            }}
        </style>
        <script>
            document.addEventListener("DOMContentLoaded", function() {{
                var coll = document.getElementsByClassName("collapsible");
                for (var i = 0; i < coll.length; i++) {{
                    coll[i].addEventListener("click", function() {{
                        this.classList.toggle("active");
                        var content = this.nextElementSibling;
                        if (content.style.display === "block") {{
                            content.style.display = "none";
                        }} else {{
                            content.style.display = "block";
                        }}
                    }});
                }}
            }});
        </script>
    </head>
    <body>
        <h2>Migration Summary Report</h1>
        <div class="summary">
        </div>
        <button type="button" class="collapsible">Assess Migration <br>Status: {assess_migration_status}</button>
        <div class="content">
            <p>Command Run: {assess_migration_command}</p>
            <h3>Report</h3>
            <pre>{assessment_report_content}</pre>
        </div>
        <button type="button" class="collapsible">Export Schema <br>Status: {export_schema_status}</button>
        <div class="content">
            <p>Command Run: {export_schema_command}</p>
        </div>
        <button type="button" class="collapsible">Analyze Schema <br>Status: {analyze_schema_status}</button>
        <div class="content">
            <p>Command Run: {analyze_schema_command}</p>
            <h3>Schema Analysis Report</h3>
            <pre>{schema_analysis_report_content}</pre>
        </div>
        {"<button type='button' class='collapsible'>User Provided Scripts Executed</button><div class='content'><p>" + ", ".join(user_scripts_executed) + "</p></div>" if user_scripts_executed else ""}
        {
            (
                "<button type='button' class='collapsible'>File Replacements Done &#10004;</button>"
                "<div class='content'><p>" +
                "<br>".join(
                    f"{replace_files_executed[old]['new_file']} -> {old}<br><br>Backup Created: {replace_files_executed[old]['backup_file']} <br>"
                    for old in replace_files_executed
                ) +
                "</p>" +
                (
                    "<button type='button' class='collapsible'>View Diff</button><div class='content'><pre>" + diff_content + "</pre></div>"
                    if replace_files_executed else ""
                ) +
                "</div><br>"
            ) if replace_files_executed else ""
        }

        <button type="button" class="collapsible">Import Schema <br>Status: {import_schema_status}</button>
        <div class="content">
            <p>Command Run: {import_schema_command}</p>
            {f'<p>Failed.sql:</p><pre>{failed_sql_content}</pre>' if not import_schema_success else ''}
        </div>
        <button type="button" class="collapsible">Export Data <br>Status: {export_data_status}</button>
        <div class="content">
            <p>Command Run: {export_data_command}</p>
        </div>
        <button type="button" class="collapsible">Import Data <br>Status: {import_data_status}</button>
        <div class="content">
            <p>Command Run: {import_data_command}</p>
        </div>
    </body>
    </html>
    """

    # Write HTML report to file
    report_file = reports_dir / 'report.html'
    with open(report_file, 'w') as f:
        f.write(html_report_content)

    print(f"Report generated: {report_file}")

def handle_file_replacements(replace_files_dict):
    replace_files_executed = {}
    for old_file, new_file in replace_files_dict.items():
        old_path = Path(old_file)
        new_path = Path(new_file)
        if not old_path.exists():
            print(f"Error: The old file '{old_file}' does not exist.")
            sys.exit(1)
        if not new_path.exists():
            print(f"Error: The new file '{new_file}' does not exist.")
            sys.exit(1)
        backup_path = old_path.with_suffix(old_path.suffix + '.bak')
        try:
            shutil.copy(old_path, backup_path)  # Backup the old file
            shutil.copy(new_path, old_path)     # Copy the new file to the old file location
            print(f"Copied '{new_file}' to '{old_file}', backup created as '{backup_path}'")
            # Store old file, new file, and backup file in the dictionary
            replace_files_executed[old_file] = {
                'old_file': old_file,
                'new_file': new_file,
                'backup_file': str(backup_path)
            }
        except Exception as e:
            print(f"Error: Failed to copy '{new_file}' to '{old_file}': {e}")
            sys.exit(1)
    return replace_files_executed


# Main function
def main(export_dir=None, env_file='env.sh', run_data=False, user_scripts=None, replace_files=None):
    if export_dir is None:
        export_dir = Path.cwd()
        print(f"No export directory provided. Using the current directory as the export directory: {export_dir}")
    else:
        export_dir = Path(export_dir)

    if not export_dir.is_dir():
        print(f"Error: Export directory '{export_dir}' does not exist.")
        sys.exit(1)

    env = source_env(env_file)

    assess_migration_success = run_assess_migration(export_dir, env)
    export_schema_success = export_schema(export_dir, env)
    analyze_schema_success = analyze_schema(export_dir)

    # Run user-provided scripts after analyzing schema
    executed_scripts = []
    if user_scripts:
        executed_scripts = run_user_scripts(user_scripts)
        if not executed_scripts:
            print(f"Error: Failed to run user-provided scripts.")
            sys.exit(1)

    # Handle file replacements after analyzing schema and before importing schema
    replace_files_executed = {}
    if replace_files:
        replace_files_dict = parse_replace_files(replace_files)
        replace_files_executed = handle_file_replacements(replace_files_dict)

    import_schema_success = import_schema(export_dir, env)

    export_data_success = False
    import_data_success = False
    if run_data:
        export_data_success = export_data(export_dir, env)
        import_data_success = import_data(export_dir, env)

    reports_dir = export_dir / 'sanity-reports'
    try:
        reports_dir.mkdir(parents=True, exist_ok=True)
    except Exception as e:
        print(f"Error: Failed to create reports directory '{reports_dir}': {e}")
        sys.exit(1)

    generate_report(
        export_dir,
        assess_migration_success,
        export_schema_success,
        analyze_schema_success,
        import_schema_success,
        export_data_success,
        import_data_success,
        executed_scripts,
        replace_files_executed,
        reports_dir,
        env
    )

# Parse the replace files argument into a dictionary
def parse_replace_files(replace_files):
    replace_files_dict = {}
    for pair in replace_files:
        old_file, new_file = pair.split(':')
        replace_files_dict[old_file.strip()] = new_file.strip()
    return replace_files_dict

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Database migration script.')
    parser.add_argument('--export_dir', nargs='?', default=None, help='Directory for export operations')
    parser.add_argument('--env_file', default='env.sh', help='Environment file')
    parser.add_argument('--run_data', action='store_true', help='Run data export and import steps')
    parser.add_argument('--user_scripts', nargs='+', help='User-provided scripts to run after schema analysis')
    parser.add_argument('--replace_files', nargs='+', help='List of old:new file pairs for replacement')
    args = parser.parse_args()
    main(args.export_dir, args.env_file, args.run_data, args.user_scripts, args.replace_files)
