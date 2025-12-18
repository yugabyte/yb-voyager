insert into num_types(v1, v2, v3, v4, v5, v6) values(182,34453,654385451,453.23,22334.542,120.50);
insert into num_types(v1, v2, v3, v4, v5, v6) values(32762,-3415123,654312385451,999.999,-22334.542,10.4);
insert into num_types(v1, v2, v3, v4, v5, v6) values(-323,53,-90654385451,-459.230,9992334.54290,-12000500.50);

select * from num_types;

insert into decimal_types values(1, 435795334362780682465462748789243337501610978301813276850553121352052192216700289113097427358778598.342434992, 12367890123456789.12);
insert into decimal_types values(2, 790809990636198497784302463464676743730460045716056588284283619572097798777544920701390228264293554.869040822, 55613803484640647.03);
insert into decimal_types values(3, 639331592204741887223305479788137535291488800417414936651322061138931510763125571702251187791371846.884254188, 99999999999999999.99);

insert into datatypes1(bool_type, char_type1, varchar_type, byte_type, enum_type) values(true,'z','this is a string','01010','Mon');
insert into datatypes1(bool_type, char_type1, varchar_type, byte_type, enum_type) values(false,'5','Lorem ipsum dolor sit amet, consectetuer adipiscing elit.','-abcd','Fri');
insert into datatypes1(bool_type, char_type1, varchar_type, byte_type, enum_type) values(true,'z','this is a string','4458','Sun');

select * from datatypes1;

insert into datetime_type(v1, v2, v3, v4) values('1996-12-02', '09:00:00',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP(0));
insert into datetime_type(v1, v2, v3, v4) values('2006-12-02', '12:00:50','2022-11-01 15:55:58.091241',CURRENT_TIMESTAMP(0));
insert into datetime_type(v1, v2, v3, v4) values('1992-01-23', null,current_timestamp,'2022-11-01 15:58:02');

select * from datetime_type;

insert into datetime_type2(v1) values('2022-11-01 15:55:58.091241');
insert into datetime_type2(v1) values('2022-11-01 15:58:02');

select * from datetime_type2;

insert into datatypes2(v1,v2,v3,v4,v5) values ('{"key1": "value1", "key2": "value2"}',B'1001100101','{20000, 14600, 23500, 13250}', '{{“FD”, “MF”}, {“FD”, “Property”}}',B'0001010101');
insert into datatypes2(v1,v2,v3,v4,v5) values ('["a","b","c",1,2,3]',B'0001010101','{20000, 14600, 23500, 13250}', '{{“FD”, “MF”}, {"act","two"}}',B'0001010');
insert into datatypes2(v1,v2,v3,v4,v5) values (null,B'1001000101',null, '{{“FD”}, {"act"}}', B'00101010101010101010101010001010100101010101010101000');

select * from datatypes2;

insert into null_and_default (id) VALUES (1);
insert into null_and_default VALUES(2, NULL, NULL, NULL);

INSERT INTO hstore_example (data) 
VALUES 
    -- 1 Basic key-value pair
    ('"key1"=>"value1", "key2"=>"value2"'),
    -- 2 Escaped quotes in key/value
    (hstore('a"b', 'd"a')),
    -- 3 NULL value
    (NULL),
    -- 4 Empty string
    (''),
    -- 5 Hstore from arrays
    (hstore(ARRAY['key1', 'key2'], ARRAY['value1', 'value2'])),
    -- 6 Mixed types as strings (text, number, boolean)
    ('key7 => value7, key8 => 123, key9 => true'),
    -- 7 Multi-line key-value
    ('"paperback" => "243",
      "publisher" => "postgresqltutorial.com",
      "language"  => "English",
      "ISBN-13"   => "978-1449370000",
      "weight"    => "11.2 ounces"'),
    -- 8 Hstore from ROW constructor
    (hstore(ROW(1,'{"key1=value1, key2=value2"}'))),
    -- 9 JSON-like string stored as value
    (hstore('json_field', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')),
    -- 10 Escaped nested quotes
    ('"{\"key1=value1, key2=value2\"}"=>"{\"key1=value1, key2={\"key1=value1, key2=value2\"}\"}"'),
    -- 11 Double quotes in key
    (hstore('"{""key1"":""value1"",""key2"":""value2""}"', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')),
    -- 12 Braces in key
    (hstore('"{key1:value1,key2:value2}"', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')),
    -- 13 Special characters
    (hstore('key=,=>', 'value\n\t')),
    -- 14 Empty key
    (hstore('', 'emptykey')),
    -- 15 Empty value
    (hstore('emptyvalue', '')),
    -- 16 Very long string (key or value)
    (hstore('longkey', repeat('x', 10000))),
    -- 17 UTF-8 multi-byte letters
    (hstore('ключ', 'значение')),
    -- 18 UTF-8 Emoji
    (hstore('emoji_😀', '😎🔥')),
    -- 19 UTF-8 accented letters
    (hstore('café', 'naïve')),
    -- 20 SQL reserved word as key (representative)
    (hstore('select', 'statement')),
    -- 21 Duplicate key
    ('dup => first, dup => second'),
    -- 22 Key with spaces
    (hstore('key with spaces', 'value with spaces')),
    -- 23 Key/value with leading/trailing space
    (hstore(' space_key ', ' space_value ')),
    -- 24 Nested double quotes
    (hstore('key"quote', 'value"quote')),
    -- 25 Nested single quotes
    (hstore('key''single', 'value''single')),
    -- 26 JSON-like key/value stored as text
    (hstore('{"json_like_key":1}', '{"json_like_value":2}'));

-- TSVECTOR table data
INSERT INTO tsvector_table (title, content, title_tsv, content_tsv)
VALUES 
    ('PostgreSQL Tutorial', 
     'PostgreSQL is a powerful open-source database system',
     to_tsvector('english', 'PostgreSQL Tutorial'),
     to_tsvector('english', 'PostgreSQL is a powerful open-source database system')),
    ('Advanced SQL', 
     'Learn advanced SQL queries and optimization techniques',
     to_tsvector('english', 'Advanced SQL'),
     to_tsvector('english', 'Learn advanced SQL queries and optimization techniques')),
    ('Data Migration', 
     'Migrating data from one database to another requires careful planning',
     to_tsvector('english', 'Data Migration'),
     to_tsvector('english', 'Migrating data from one database to another requires careful planning'));

select * from tsvector_table;

-- Enum array table data
INSERT INTO enum_array_table (day_name, week_days, description)
VALUES 
    ('Mon', ARRAY['Mon', 'Wed', 'Fri']::week[], 'Work days example 1'),
    ('Tue', ARRAY['Tue', 'Thu']::week[], 'Work days example 2'),
    ('Sat', ARRAY['Sat', 'Sun']::week[], 'Weekend days');

select * from enum_array_table;

-- Composite types data
INSERT INTO composite_types (address) VALUES (ROW('City1I', 'Street 1I', 1)::full_address),(ROW('City2I', 'Street 2I', 2)::full_address);

-- Composite array types data
INSERT INTO composite_array_types (addresses)
VALUES (
    ARRAY[
        ROW('CityA1I', 'StreetA 1I', 1)::full_address,
        ROW('CityB1I', 'StreetB 1I', 2)::full_address
    ]
),
(
    ARRAY[
        ROW('CityA2I', 'StreetA 2I', 2)::full_address,
        ROW('CityB2I', 'StreetB 2I', 3)::full_address
    ]
);

-- Domain types data
INSERT INTO domain_types (ssn, email, rating, prefs)
VALUES (
    '001-00-0001'::social_security_number,
    'user1@example.com'::email_address,
    1::rating_1_to_5,
    '{"version":"1","theme":"dark"}'::app_settings
),
(
    '002-00-0002'::social_security_number,
    'user2@example.com'::email_address,
    2::rating_1_to_5,
    '{"version":"2","theme":"dark"}'::app_settings
),
(
    '003-00-0003'::social_security_number,
    'user3@example.com'::email_address,
    3::rating_1_to_5,
    '{"version":"3","theme":"dark"}'::app_settings
);

-- Domain array types data

INSERT INTO domain_array_types (ssn_list, phone_list, name_list)
VALUES (
    ARRAY[
        '123-45-0001'::social_security_number,
        '987-65-0001'::social_security_number
    ],
    ARRAY[
        '+91123456701'::phone_number,
        '+91987654301'::phone_number
    ],
    ARRAY[
        'ABC DEF'::full_name,
        'GHI JKL'::full_name
    ]
),
(
    ARRAY[
        '123-45-0002'::social_security_number,
        '987-65-0002'::social_security_number
    ],
    ARRAY[
        '+91123456702'::phone_number,
        '+91987654302'::phone_number
    ],
    ARRAY[
        'MNO PQR'::full_name,
        'STU VWX'::full_name
    ]
),
(
    ARRAY[
        '123-45-0003'::social_security_number,
        '987-65-0003'::social_security_number
    ],
    ARRAY[
        '+91123456703'::phone_number,
        '+91987654303'::phone_number
    ],
    ARRAY[
        'ABC XYZ'::full_name,
        'DEF ABC'::full_name
    ]
);

-- Range Types
INSERT INTO range_types (
    price_range_col, discount_range_col, period_range_col, active_ts_range_col
)
VALUES (
    '[0.1,5.1)'::price_range,
    '[0.0,1.0)'::discount_range,
    ( '[' || current_date::text || ',' || (current_date + 7)::text || ')' )::period_range,
    ( '[' || now()::text || ',' || (now() + interval '24 hours')::text || ')' )::active_ts_range
),
(
    '[0.2,5.2)'::price_range,
    '[0.1,1.1)'::discount_range,
    ( '[' || (current_date + 2)::text || ',' || (current_date + 9)::text || ')' )::period_range,
    ( '[' || (now() + interval '2 hours')::text || ',' ||
           (now() + interval '26 hours')::text || ')' )::active_ts_range
),
(
    '[0.3,5.3)'::price_range,
    '[0.2,1.2)'::discount_range,
    ( '[' || (current_date + 3)::text || ',' || (current_date + 10)::text || ')' )::period_range,
    ( '[' || (now() + interval '3 hours')::text || ',' ||
           (now() + interval '27 hours')::text || ')' )::active_ts_range
);


-- Range array types data
INSERT INTO range_array_types (
  price_ranges, discount_ranges, period_ranges, ts_ranges
)
VALUES (
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ('[' || current_date + 2001 || ',' || current_date + 2002 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '2001 hours' || ',' ||
           now() + interval '2002 hours' || ')')::active_ts_range
  ]
),
(
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ('[' || current_date + 2002 || ',' || current_date + 2003 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '2002 hours' || ',' ||
           now() + interval '2003 hours' || ')')::active_ts_range
  ]
),
(
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ('[' || current_date + 2003 || ',' || current_date + 2004 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '2003 hours' || ',' ||
           now() + interval '2004 hours' || ')')::active_ts_range
  ]
);

-- Extension types data
INSERT INTO extension_types (col_hstore, col_citext, col_ltree)
VALUES (
    'key1=>1'::hstore,
    'text_1'::citext,
    'Top.1'::ltree
),
(
    'key2=>2'::hstore,
    'text_2'::citext,
    'Top.2'::ltree
),
(
    'key3=>3'::hstore,
    'text_3'::citext,
    'Top.3'::ltree
);

-- Extension arrays data
INSERT INTO extension_arrays (col_hstore, col_citext, col_ltree)
VALUES (
    ARRAY['key1=>1'::hstore, 'value1=>1'::hstore],
    ARRAY['text_1'::citext, 'sample_1'::citext],
    ARRAY['Top.1'::ltree, 'Category.1'::ltree]
),
(
    ARRAY['key2=>2'::hstore, 'value2=>2'::hstore],
    ARRAY['text_2'::citext, 'sample_2'::citext],
    ARRAY['Top.2'::ltree, 'Category.2'::ltree]
),
(
    ARRAY['key3=>3'::hstore, 'value3=>3'::hstore],
    ARRAY['text_3'::citext, 'sample_3'::citext],
    ARRAY['Top.3'::ltree, 'Category.3'::ltree]
);

-- Nested datatypes data
INSERT INTO audit_log (
  involved_employees, affected_clients, transaction_refs
)
VALUES (
  ARRAY[
    ROW('Emp_1', 'active', 'emp_1@company.com')::employee_info,
    ROW('EmpAlt_1', 'inactive', 'empalt_1@company.com')::employee_info
  ],
  ARRAY[
    ROW('Client_1', 'north', 'client_1@example.com')::client_info,
    ROW('ClientAlt_1', 'south', 'clientalt_1@example.com')::client_info
  ],
  ARRAY[1, 2, 3]
),
(
  ARRAY[
    ROW('Emp_2', 'active', 'emp_2@company.com')::employee_info,
    ROW('EmpAlt_2', 'inactive', 'empalt_2@company.com')::employee_info
  ],
  ARRAY[
    ROW('Client_2', 'north', 'client_2@example.com')::client_info,
    ROW('ClientAlt_2', 'south', 'clientalt_2@example.com')::client_info
  ],
  ARRAY[2, 3, 4]
);