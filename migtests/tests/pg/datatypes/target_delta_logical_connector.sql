-- Additional DMLs for logical replication connector
-- These work with logical replication but not with YB CDC GRPC connector

-- Inserts
INSERT INTO hstore_example (data) 
VALUES ('key7 => value7, key8 => value8');

INSERT INTO hstore_example (data) 
VALUES (
    hstore('"{""key1"":""value1"",""key2"":""value2""}"', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')
);

-- Insert some edge case rows
INSERT INTO hstore_example (data) VALUES (hstore('empty_row', ''));
INSERT INTO hstore_example (data) VALUES (NULL);

-- Complex new insert to add intra-file dependency
INSERT INTO hstore_example (data) 
VALUES (
    'complex_key1=>"val1", "multi key 2"=>"value with spaces", "escaped\"quote"=>"line1\nline2", "unicode_αβ"=>"Ωmega"'
);

-- Insert some edge case rows
INSERT INTO hstore_example (data) 
VALUES (
    (hstore(ROW(1,'{"key1=value1, key2=value2"}')))
);

INSERT INTO hstore_example (data) 
VALUES (
    'key5=>value5, key6=>value6'
);

-- Updates
UPDATE hstore_example 
SET data = delete(data, 'key2')
WHERE id = 8;

UPDATE hstore_example 
SET data = data || 'new_key=>"new_val"'
WHERE id = 10;  -- append key to snapshot row

UPDATE hstore_example 
SET data = hstore('{"key1=value1, key2=value2"}', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')
WHERE id = 15;

UPDATE hstore_example 
SET data = '"{\"key1=value1, key2=value2\"}"=>"{\"key1=value1, key2={\"key1=value1, key2=value2\"}\"}"'
WHERE id = 14;

UPDATE hstore_example 
SET data = NULL
WHERE id = 11;  -- NULL update edge case

UPDATE hstore_example 
SET data = ''
WHERE id = 12;  -- empty string update edge case

-- Intra-file dependency updates
UPDATE hstore_example 
SET data = data || 'intra_key=>"intra_val"'
WHERE data @> 'complex_key1=>"val1"';

-- Deletes
DELETE FROM hstore_example WHERE data ? 'key5';
DELETE FROM hstore_example WHERE id = 13; -- snapshot row deletion
DELETE FROM hstore_example WHERE data @> 'k1=>v1'; -- source CDC dependency
DELETE FROM hstore_example WHERE data @> 'complex_key1=>"val1"'; -- intra-file dependency

-- TSVECTOR table operations (supported with logical connector)
INSERT INTO tsvector_table (title, content, title_tsv, content_tsv)
VALUES 
    ('Logical Replication', 
     'Logical replication support for tsvector columns',
     to_tsvector('english', 'Logical Replication'),
     to_tsvector('english', 'Logical replication support for tsvector columns'));

INSERT INTO tsvector_table (title, content, title_tsv, content_tsv)
VALUES 
    ('YugabyteDB Features', 
     'YugabyteDB distributed SQL database features',
     to_tsvector('english', 'YugabyteDB Features'),
     to_tsvector('english', 'YugabyteDB distributed SQL database features'));

UPDATE tsvector_table
SET title = 'Updated Tutorial',
    title_tsv = to_tsvector('english', 'Updated Tutorial')
WHERE id = 3;

DELETE FROM tsvector_table WHERE title = 'Full Text Search';

-- Enum array table operations (supported with logical connector)
INSERT INTO enum_array_table (day_name, week_days, description)
VALUES ('Sun', ARRAY['Sun']::week[], 'Sunday only');

INSERT INTO enum_array_table (day_name, week_days, description)
VALUES ('Wed', ARRAY['Mon', 'Wed', 'Fri']::week[], 'Alternate weekdays');

UPDATE enum_array_table
SET week_days = ARRAY['Sat', 'Sun']::week[], description = 'Complete weekend'
WHERE id = 3;

DELETE FROM enum_array_table WHERE day_name = 'Thu';

-- Composite types table operations
INSERT INTO composite_types (address) VALUES (ROW('City4', 'Street 4', 4)::full_address), (ROW('City5', 'Street 5', 5)::full_address), (ROW('City6', 'Street 6', 6)::full_address);

UPDATE composite_types
SET address = ROW('UpdatedCity', 'UpdatedStreet', 99999)::full_address
WHERE id = 1;

DELETE FROM composite_types
WHERE id = 5;

-- Composite array types table operations
INSERT INTO composite_array_types (addresses)
VALUES (
    ARRAY[
        ROW('CityA4', 'StreetA 4', 4)::full_address,
        ROW('CityB4', 'StreetB 4', 5)::full_address
    ]
),
(
    ARRAY[
        ROW('CityA5', 'StreetA 5', 5)::full_address,
        ROW('CityB5', 'StreetB 5', 6)::full_address
    ]
),
(
    ARRAY[
        ROW('CityA6', 'StreetA 6', 6)::full_address,
        ROW('CityB6', 'StreetB 6', 7)::full_address
    ]
);


UPDATE composite_array_types
SET addresses = ARRAY[
    ROW('UpdatedCity4', 'UpdatedStreet4', 44444)::full_address,
    ROW('UpdatedCity5', 'UpdatedStreet5', 55555)::full_address
]
WHERE id = 1;


DELETE FROM composite_array_types
WHERE id = 6;

-- Domain types data
INSERT INTO domain_types (ssn, email, rating, prefs)
VALUES (
    '001-00-0007'::social_security_number,
    'user7@example.com'::email_address,
    4::rating_1_to_5,
    '{"version":"7","theme":"dark"}'::app_settings
),
(
    '002-00-0008'::social_security_number,
    'user8@example.com'::email_address,
    5::rating_1_to_5,
    '{"version":"8","theme":"dark"}'::app_settings
),
(
    '003-00-0009'::social_security_number,
    'user9@example.com'::email_address,
    5::rating_1_to_5,
    '{"version":"9","theme":"dark"}'::app_settings
);

UPDATE domain_types
SET
    ssn = '123-45-6789'::social_security_number,
    email = 'updated@example.com'::email_address,
    rating = 5::rating_1_to_5,
    prefs = '{"version":"updated","theme":"light"}'::app_settings
WHERE id = 7;

DELETE FROM domain_types
WHERE id = 8;

-- Domain array types data
INSERT INTO domain_array_types (ssn_list, phone_list, name_list)
VALUES (
    ARRAY[
        '123-45-0007'::social_security_number,
        '987-65-0007'::social_security_number
    ],
    ARRAY[
        '+91123456707'::phone_number,
        '+91987654307'::phone_number
    ],
    ARRAY[
        'ABCD EFGH'::full_name,
        'IJKL MNOP'::full_name
    ]
),
(
    ARRAY[
        '123-45-0008'::social_security_number,
        '987-65-0008'::social_security_number
    ],
    ARRAY[
        '+91123456708'::phone_number,
        '+91987654308'::phone_number
    ],
    ARRAY[
        'MNOP QRST'::full_name,
        'UVWXYZ ABCD'::full_name
    ]
),
(
    ARRAY[
        '123-45-0009'::social_security_number,
        '987-65-0009'::social_security_number
    ],
    ARRAY[
        '+91123456709'::phone_number,
        '+91987654309'::phone_number
    ],
    ARRAY[
        'EFGH IJKL'::full_name,
        'UVWXYZ ABCD'::full_name
    ]
);

UPDATE domain_array_types
SET
    ssn_list = ARRAY[
        '111-22-3333'::social_security_number,
        '444-55-6666'::social_security_number
    ],
    phone_list = ARRAY[
        '+911111111111'::phone_number,
        '+922222222222'::phone_number
    ],
    name_list = ARRAY[
        'Updated Person'::full_name,
        'Another Update'::full_name
    ]
WHERE id = 7;

DELETE FROM domain_array_types
WHERE id = 8;

-- Range types data
INSERT INTO range_types (
    price_range_col, discount_range_col, period_range_col, active_ts_range_col
)
VALUES (
    '[1.0,6.0)'::price_range,
    '[0.2,1.2)'::discount_range,
    ( '[' || (current_date + 10)::text || ',' || (current_date + 18)::text || ')' )::period_range,
    ( '[' || now()::text || ',' || (now() + interval '20 hours')::text || ')' )::active_ts_range
),
(
    '[1.5,6.5)'::price_range,
    '[0.3,1.3)'::discount_range,
    ( '[' || (current_date + 11)::text || ',' || (current_date + 19)::text || ')' )::period_range,
    ( '[' || (now() + interval '1 hour')::text || ',' ||
           (now() + interval '23 hours')::text || ')' )::active_ts_range
),
(
    '[2.0,7.0)'::price_range,
    '[0.4,1.4)'::discount_range,
    ( '[' || (current_date + 12)::text || ',' || (current_date + 20)::text || ')' )::period_range,
    ( '[' || (now() + interval '2 hours')::text || ',' ||
           (now() + interval '25 hours')::text || ')' )::active_ts_range
);


UPDATE range_types
SET
  price_range_col    = '[10,20)'::price_range,
  discount_range_col = '[0.5,1.5)'::discount_range,
  period_range_col   = ( '[' || (current_date + 7)::text || ',' || (current_date + 14)::text || ')' )::period_range,
  active_ts_range_col= ( '[' || now()::text || ',' || (now() + interval '1 day')::text || ')' )::active_ts_range
WHERE id = 8;


DELETE FROM range_types
WHERE id = 9;

-- Range array types data
INSERT INTO range_array_types (
  price_ranges, discount_ranges, period_ranges, ts_ranges
)
VALUES (
  ARRAY['[2,6)'::price_range, '[12,22)'::price_range],
  ARRAY['[0.6,1.1)'::discount_range, '[1.2,2.3)'::discount_range],
  ARRAY[
    ('[' || current_date + 3000 || ',' || current_date + 3001 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '3000 hours' || ',' ||
           now() + interval '3001 hours' || ')')::active_ts_range
  ]
),
(
  ARRAY['[3,7)'::price_range, '[14,24)'::price_range],
  ARRAY['[0.7,1.2)'::discount_range, '[1.3,2.4)'::discount_range],
  ARRAY[
    ('[' || current_date + 3001 || ',' || current_date + 3002 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '3001 hours' || ',' ||
           now() + interval '3002 hours' || ')')::active_ts_range
  ]
),
(
  ARRAY['[4,8)'::price_range, '[16,26)'::price_range],
  ARRAY['[0.8,1.3)'::discount_range, '[1.4,2.5)'::discount_range],
  ARRAY[
    ('[' || current_date + 3002 || ',' || current_date + 3003 || ')')::period_range
  ],
  ARRAY[
    ('[' || now() + interval '3002 hours' || ',' ||
           now() + interval '3003 hours' || ')')::active_ts_range
  ]
);


UPDATE range_array_types
SET
  price_ranges    = ARRAY['[50,60)'::price_range],
  discount_ranges = ARRAY['[0.1,0.2)'::discount_range],
  period_ranges   = ARRAY[
      ('[' || current_date || ',' || (current_date + 2) || ')')::period_range
  ],
  ts_ranges       = ARRAY[
      ('[' || now() || ',' || (now() + interval '3 hours') || ')')::active_ts_range
  ]
WHERE id = 8;


DELETE FROM range_array_types
WHERE id = 9;

-- Extension types
INSERT INTO extension_types (col_hstore, col_citext, col_ltree)
VALUES (
    'key7=>7'::hstore,
    'text_7'::citext,
    'Top.7'::ltree
),
(
    'key8=>8'::hstore,
    'text_8'::citext,
    'Top.8'::ltree
),
(
    'key9=>9'::hstore,
    'text_9'::citext,
    'Top.9'::ltree
);

UPDATE extension_types
SET
    col_hstore = 'updated=>1'::hstore,
    col_citext = 'updated_text'::citext,
    col_ltree  = 'Updated.Node'::ltree
WHERE id = 7;

DELETE FROM extension_types
WHERE id = 9;

-- Extension arrays
INSERT INTO extension_arrays (col_hstore, col_citext, col_ltree)
VALUES (
    ARRAY['key7=>7'::hstore, 'value7=>7'::hstore],
    ARRAY['text_7'::citext, 'sample_7'::citext],
    ARRAY['Top.7'::ltree, 'Category.7'::ltree]
),
(
    ARRAY['key8=>8'::hstore, 'value8=>8'::hstore],
    ARRAY['text_8'::citext, 'sample_8'::citext],
    ARRAY['Top.8'::ltree, 'Category.8'::ltree]
),
(
    ARRAY['key9=>9'::hstore, 'value9=>9'::hstore],
    ARRAY['text_9'::citext, 'sample_9'::citext],
    ARRAY['Top.9'::ltree, 'Category.9'::ltree]
);

UPDATE extension_arrays
SET 
    col_hstore = ARRAY['a=>1'::hstore, 'b=>2'::hstore],
    col_citext = ARRAY['updated1'::citext, 'updated2'::citext],
    col_ltree  = ARRAY['Top.Science'::ltree, 'Top.Arts'::ltree]
WHERE id = 7;

DELETE FROM extension_arrays
WHERE id = 8;

-- Nested datatypes
INSERT INTO audit_log (
  involved_employees, affected_clients, transaction_refs
)
VALUES (
  ARRAY[
    ROW('Emp_4', 'active', 'emp_4@company.com')::employee_info,
    ROW('EmpAlt_4', 'terminated', 'empalt_4@company.com')::employee_info
  ],
  ARRAY[
    ROW('Client_4', 'east', 'client_4@example.com')::client_info,
    ROW('ClientAlt_4', 'west', 'clientalt_4@example.com')::client_info
  ],
  ARRAY[4, 5, 6]
);

UPDATE audit_log
SET
  involved_employees = ARRAY[
    ROW('FollowupEmp_1', 'inactive', 'followup1@company.com')::employee_info,
    ROW('FollowupEmp_2', 'active', 'followup2@company.com')::employee_info
  ],
  affected_clients = ARRAY[
    ROW('FollowupClient_1', 'south', 'followupc1@example.com')::client_info,
    ROW('FollowupClient_2', 'north', 'followupc2@example.com')::client_info
  ],
  transaction_refs = ARRAY[50, 60, 70]
WHERE id = 4;

DELETE FROM audit_log WHERE id = 2;