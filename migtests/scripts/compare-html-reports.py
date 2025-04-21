#!/usr/bin/env python3

import argparse
import re
from bs4 import BeautifulSoup
import difflib
import sys
from collections import Counter

def normalize_text(text):
    """Normalize and clean up text content."""
    text = text.replace('\xa0', ' ')  # Replace non-breaking spaces with regular spaces
    text = text.replace('\u200B', '')  # Remove zero-width spaces
    text = re.sub(r'\s+', ' ', text)  # Collapse any whitespace sequences
    return text.strip()  # Remove leading/trailing spaces

def extract_and_normalize_texts(elements):
    """Extract and normalize inner text from HTML elements."""
    return [normalize_text(el.get_text(separator=" ")) for el in elements]

def extract_divs(soup):
    """Extract and normalize divs, optionally sorting them based on certain conditions."""
    all_divs = soup.find_all("div")
    filtered_divs = []
    should_sort = False

    for div in all_divs:
        # Sorting the content within "Sharding Recommendations" since the tables can be in different order
        prev_h2 = div.find_previous("h2")
        if prev_h2 and "Sharding Recommendations" in prev_h2.get_text():
            should_sort = True
        else:
            should_sort = False  # Reset should_sort for each div

        # Skipping the parent wrapper div since it causes issues to be reported twice
        if "wrapper" not in div.get("class", []):  # Exclude wrapper divs
            div_text = normalize_text(div.get_text(separator=" ")).strip()
            if should_sort:
                div_text = " ".join(sorted(div_text.split()))  # Sort content within div
            filtered_divs.append(div_text)

    return sorted(filtered_divs) if any(s for s in filtered_divs) else filtered_divs


def extract_paragraphs(soup):
    """Extract and filter paragraphs, skipping specific ones based on conditions."""
    paragraphs = soup.find_all("p")
    skip_first_paragraph = False
    filtered_paragraphs = []

    for p in paragraphs:
        # Skip the first <p> after the <h2> Migration Complexity Explanation since nesting causes issues to be reported twice
        prev_element = p.find_previous("h2")
        if prev_element and "Migration Complexity Explanation" in prev_element.get_text():
            if not skip_first_paragraph:
                skip_first_paragraph = True
                continue
        
        # Skip paragraphs that contain "Database Version:" or "Voyager Version:" as it vary according to the environment
        if p.find("strong") and any(skip_text in p.get_text() for skip_text in ["Database Version:", "Voyager Version:", "Target YB Version:"]):
            continue
        
        filtered_paragraphs.append(p)

    return extract_and_normalize_texts(filtered_paragraphs)

def normalize_table_names(values):
        """Extract and normalize the table names from <td> and <th> and sort them."""
        table_names = []
        for v in values:
            #Get the text and split it into lines
            text_lines = v.get_text(separator="\n").split("\n")

            #Filter out empty lines and normalize each line
            normalized_names = [normalize_text(line).strip() for line in text_lines if line.strip()]

            table_names.extend(normalized_names)
        return sorted(table_names)

def extract_table_data(tables):
    """Sort tables by rows and their contents, including headers."""
    sorted_tables = []
    for table in tables:
        rows = table.find_all("tr")
        table_data = []
        table_headers = []
        for row in rows:
            columns = row.find_all("td")
            headers = row.find_all("th")
            if len(columns) > 1:
                table_data.append(normalize_table_names(columns))
            if len(headers) > 1:
                table_headers.append(normalize_table_names(headers))
        if table_data:
            sorted_tables.append(table_data)
        if table_headers:
            sorted_tables.append(table_headers)
    return sorted_tables

def extract_html_data(html_content):
    """Main function to extract structured data from HTML content."""
    soup = BeautifulSoup(html_content, 'html.parser')

    data = {
        "title": normalize_text(soup.title.string) if soup.title and soup.title.string else "No Title",
        "headings": extract_and_normalize_texts(soup.find_all(['h1', 'h2', 'h3', 'h4', 'h5', 'h6'])),
        "paragraphs": extract_paragraphs(soup),
        "tables": extract_table_data(soup.find_all("table")),
        "links": {
            k: v for k, v in sorted(
                {normalize_text(a.get("href") or ""): normalize_text(a.text) for a in soup.find_all("a")}.items()
            )
        },
        "spans": extract_and_normalize_texts(soup.find_all("span")),
        "divs": extract_divs(soup)
    }

    return data

def generate_diff_list(list1, list2, section_name, file1_path, file2_path):
    """Generate a structured diff for ordered lists (headings, paragraphs, spans, divs) showing only differences."""
    if section_name == "tables":
        # If list1 and list2 are already lists of table names, directly sort them
        list1 = [sorted(table) if isinstance(table, list) else table for table in list1]
        list2 = [sorted(table) if isinstance(table, list) else table for table in list2]
    diff = list(difflib.unified_diff(
        [f"{i+1}. {item}" for i, item in enumerate(list1)],
        [f"{i+1}. {item}" for i, item in enumerate(list2)],
        fromfile=f"{file1_path}",
        tofile=f"{file2_path}",
        lineterm="",
        n=0  # Ensures ONLY differences are shown (no context lines)
    ))
    return "\n".join(diff) or None  # Return None if no differences

def dict_to_list(dict_data):
    """Convert dictionary to list of formatted strings."""
    return [f"{k} -> {v}" for k, v in dict_data.items()]

def compare_html_tags(html_data1, html_data2):
    """Compare the unique tags and their counts in the two HTML reports."""

    def get_tag_counts(html_content):
        """Extracts all tag names and their counts from the given HTML content."""
        soup = BeautifulSoup(html_content, 'html.parser')
        return Counter(tag.name for tag in soup.find_all())

    tags_count1 = get_tag_counts(html_data1)
    tags_count2 = get_tag_counts(html_data2)

    tags1 = set(tags_count1.keys())
    tags2 = set(tags_count2.keys())

    missing_tags_in_file1 = tags2 - tags1  # Tags in file2 but missing in file1
    missing_tags_in_file2 = tags1 - tags2  # Tags in file1 but missing in file2

    differences = {}

    if missing_tags_in_file1:
        differences["extra_tags_in_the_actual_report"] = "\n".join(missing_tags_in_file1)
    if missing_tags_in_file2:
        differences["missing_tags_in_the_actual_report"] = "\n".join(missing_tags_in_file2)

    # Check for tag count mismatches
    tag_count_mismatches = {
        tag: f"Expected: {tags_count1[tag]}, Found: {tags_count2[tag]}"
        for tag in (tags1 & tags2)  # Tags present in both files
        if tags_count1[tag] != tags_count2[tag]
    }

    if tag_count_mismatches:
        differences["tag_count_mismatches"] = "\n".join(
            f"{tag}: {count}" for tag, count in tag_count_mismatches.items()
        )

    return differences

def compare_html_data(html_data1, html_data2, file1, file2):

    differences = {}

    for section in html_data1.keys():
        if html_data1[section] != html_data2[section]:
            if isinstance(html_data1[section], list):  # For headings, paragraphs, spans, divs
                diff = generate_diff_list(html_data1[section], html_data2[section], section, file1, file2)
            elif isinstance(html_data1[section], dict):  # For links dictionary
                diff = generate_diff_list(
                    dict_to_list(html_data1[section]),
                    dict_to_list(html_data2[section]),
                    section, file1, file2
                )
            else:  # Title (single string)
                diff = generate_diff_list([html_data1[section]], [html_data2[section]], section, file1, file2)

            if diff:  # Only store sections that have differences
                differences[section] = diff
    
    return differences

def read_and_compare_html_files(file1, file2):
    """Read and extract structured data from HTML files."""
    with open(file1, "r", encoding="utf-8") as f1, open(file2, "r", encoding="utf-8") as f2:
        html_data1 = f1.read()
        html_data2 = f2.read()
    
    tags_differences = compare_html_tags(html_data1, html_data2)
    data_differences = compare_html_data(extract_html_data(html_data1), extract_html_data(html_data2), file1, file2)

    # Print differences here
    if not tags_differences and not data_differences:
        print("The reports are matching.")
    else:
        print("The reports are not matching:")
        
        if tags_differences:
            print("\n=== TAG DIFFERENCES ===")
            for section, diff_text in tags_differences.items():
                print(f"\n=== {section.replace('_', ' ').upper()} ===")
                print(diff_text)
        
        if data_differences:
            for section, diff_text in data_differences.items():
                print(f"\n=== {section.upper()} DIFFERENCES ===")
                print(diff_text)
        
        sys.exit(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Compare two HTML reports.")
    parser.add_argument("report1", help="Path to the first HTML report")
    parser.add_argument("report2", help="Path to the second HTML report")
    
    args = parser.parse_args()
    read_and_compare_html_files(args.report1, args.report2)
