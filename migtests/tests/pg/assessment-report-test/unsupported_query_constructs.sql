-- Unsupported Query Constructs 

drop extension if exists pg_stat_statements;

create extension pg_stat_statements;

SELECT * FROM pg_stat_statements;

-- System Columns 

SELECT ctid, tableoid, xmin, xmax, cmin, cmax
FROM employees2;

-- XML Functions

SELECT table_to_xml('employees2', true, false, '');

SELECT xmlparse(document '<data><item>A</item></data>') as xmldata;

SELECT xmlforest(first_name AS element1, last_name AS element2) FROM employees2;

SELECT xmlelement(name root, xmlelement(name child, 'value'));

SELECT xml_is_well_formed('<root><child>value</child></root>');

SELECT *
FROM xmltable(
    '/employees/employee'
    PASSING '<employees><employee><name>John</name></employee></employees>'
    COLUMNS 
        name TEXT PATH 'name'
);


-- Advisory Locks

SELECT pg_advisory_lock(1,2); 
SELECT pg_advisory_unlock(1,2); 
SELECT pg_advisory_xact_lock(1,2); 
SELECT pg_advisory_unlock_all();

-- Adding few XMLTABLE() examples
-- Case 1
CREATE TABLE library_nested (
    lib_id INT,
    lib_data XML
);

INSERT INTO library_nested VALUES
(1, '
<library>
    <section name="Fiction">
        <book>
            <title>The Great Gatsby</title>
            <author>F. Scott Fitzgerald</author>
        </book>
        <book>
            <title>1984</title>
            <author>George Orwell</author>
        </book>
    </section>
    <section name="Science">
        <book>
            <title>A Brief History of Time</title>
            <author>Stephen Hawking</author>
        </book>
    </section>
</library>
');

-- Query with nested XMLTABLE() calls
SELECT
    s.section_name,
    b.title,
    b.author
FROM
    library_nested l,
    XMLTABLE(
        '/library/section'
        PASSING l.lib_data
        COLUMNS
            section_name TEXT PATH '@name',
            books XML PATH '.'
    ) AS s,
    XMLTABLE(
        '/section/book'
        PASSING s.books
        COLUMNS
            title TEXT PATH 'title',
            author TEXT PATH 'author'
) AS b;


-- Case 2
CREATE TABLE orders_lateral (
    order_id INT,
    customer_id INT,
    order_details XML
);

INSERT INTO orders_lateral (customer_id, order_details) VALUES
(1, 1, '
    <order>
        <item>
            <product>Keyboard</product>
            <quantity>2</quantity>
        </item>
        <item>
            <product>Mouse</product>
            <quantity>1</quantity>
        </item>
    </order>
'),
(2, '
    <order>
        <item>
            <product>Monitor</product>
            <quantity>1</quantity>
        </item>
    </order>
');

-- Query using XMLTABLE with LATERAL join
SELECT
    o.order_id,
    items.product,
    items.quantity::INT
FROM
    orders_lateral o
    CROSS JOIN LATERAL XMLTABLE(
        '/order/item'
        PASSING o.order_details
        COLUMNS
            product TEXT PATH 'product',
            quantity TEXT PATH 'quantity'
) AS items;


SELECT lo_create('32142');

-- Unsupported COPY constructs

CREATE TABLE IF NOT EXISTS employeesCopyFromWhere (
    id INT PRIMARY KEY,
    name TEXT NOT NULL,
    age INT NOT NULL
);


-- COPY FROM with WHERE clause
COPY employeesCopyFromWhere (id, name, age)
FROM STDIN WITH (FORMAT csv)
WHERE age > 30;
1,John Smith,25
2,Jane Doe,34
3,Bob Johnson,31
\.

CREATE TABLE IF NOT EXISTS employeesCopyOnError (
    id INT PRIMARY KEY,
    name TEXT NOT NULL,
    age INT NOT NULL
);

-- COPY with ON_ERROR clause
COPY employeesCopyOnError (id, name, age)
FROM STDIN WITH (FORMAT csv, ON_ERROR IGNORE );
4,Adam Smith,22
5,John Doe,34
6,Ron Johnson,31
\.

