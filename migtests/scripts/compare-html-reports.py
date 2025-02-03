#!/usr/bin/env python3

import argparse
import re
from bs4 import BeautifulSoup
import difflib
import sys

def extract_html_data(html_content):
    """Extracts structured data from HTML content and normalizes it."""
    soup = BeautifulSoup(html_content, 'html.parser')

    def normalize_text(text):
        # Replace non-breaking spaces and other similar characters
        text = text.replace('\xa0', ' ')
        text = text.replace('\u200B', '')
        text = re.sub(r'\s+', ' ', text)
        return text.strip()

    def extract_texts(elements):
        """Extract and normalize inner text while keeping structure."""
        return [normalize_text(el.get_text(separator=" ")) for el in elements]

    all_divs = soup.find_all("div")
    filtered_divs = []
    should_sort = False

    for div in all_divs:
        prev_h2 = div.find_previous("h2")
        if prev_h2 and "Sharding Recommendations" in prev_h2.get_text():
            should_sort = True

        if "wrapper" not in div.get("class", []):  # Keep the wrapper exclusion
            filtered_divs.append((div, should_sort))

    # Extract and process div text
    div_texts_sorted = []
    for div, should_sort in filtered_divs:
        div_text = normalize_text(div.get_text(separator=" ")).strip()
        if should_sort:
            div_text = " ".join(sorted(div_text.split()))  # Sort content within div
        div_texts_sorted.append(div_text)

    div_texts_sorted = sorted(div_texts_sorted) if any(s for _, s in filtered_divs) else div_texts_sorted

    paragraphs = soup.find_all("p")

    # Flag to skip the first <p> after the <h2> Migration Complexity Explanation
    skip_first_paragraph = False
    filtered_paragraphs = []

    for p in paragraphs:
        prev_element = p.find_previous("h2")
        if prev_element and "Migration Complexity Explanation" in prev_element.get_text():
            if not skip_first_paragraph:
                skip_first_paragraph = True
                continue
        # Skip paragraph that start with "Database Version:"
        if p.find("strong") and "Database Version:" in p.get_text():
            continue
        filtered_paragraphs.append(p)

    def normalize_table_names(tds):
        """Extract and normalize the table names from <td> and sort them."""
        table_names = []
        for td in tds:
            names = [normalize_text(name).strip() for name in td.get_text(separator="\n").split("\n") if name.strip()]
            table_names.extend(names)
        return sorted(table_names)

    def sort_table_data(tables):
        """Sort tables by rows and within rows by their contents."""
        sorted_tables = []
        for table in tables:
            rows = table.find_all("tr")
            table_data = []
            for row in rows:
                cols = row.find_all("td")
                if len(cols) > 1:
                    table_data.append(normalize_table_names(cols))
            if table_data:
                sorted_tables.append(table_data)
        return sorted_tables

    data = {
        "title": normalize_text(soup.title.string) if soup.title and soup.title.string else "No Title",
        "headings": extract_texts(soup.find_all(['h1', 'h2', 'h3', 'h4', 'h5', 'h6'])),
        "paragraphs": extract_texts(filtered_paragraphs),
        "tables": sort_table_data(soup.find_all("table")),
        "links": {
            k: v for k, v in sorted(
                {normalize_text(a.get("href") or ""): normalize_text(a.text) for a in soup.find_all("a")}.items()
            )
        },
        "spans": extract_texts(soup.find_all("span")),
        "divs": div_texts_sorted
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

def compare_html_reports(file1, file2):
    """Compares two HTML reports and prints structured differences."""
    with open(file1, "r", encoding="utf-8") as f1, open(file2, "r", encoding="utf-8") as f2:
        html_data1 = extract_html_data(f1.read())
        html_data2 = extract_html_data(f2.read())

    differences = {}

    for key in html_data1.keys():
        if html_data1[key] != html_data2[key]:
            if isinstance(html_data1[key], list):  # For headings, paragraphs, spans, divs
                diff = generate_diff_list(html_data1[key], html_data2[key], key, file1, file2)
            elif isinstance(html_data1[key], dict):  # For links dictionary
                diff = generate_diff_list(
                    dict_to_list(html_data1[key]),
                    dict_to_list(html_data2[key]),
                    key, file1, file2
                )
            else:  # Title (single string)
                diff = generate_diff_list([html_data1[key]], [html_data2[key]], key, file1, file2)

            if diff:  # Only store sections that have differences
                differences[key] = diff

    if not differences:
        print("The reports are identical.")
    else:
        print("Differences found:")
        for section, diff_text in differences.items():
            print(f"\n=== {section.upper()} DIFFERENCES ===")
            print(diff_text)
        sys.exit(1)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Compare two HTML reports.")
    parser.add_argument("report1", help="Path to the first HTML report")
    parser.add_argument("report2", help="Path to the second HTML report")
    
    args = parser.parse_args()
    compare_html_reports(args.report1, args.report2)