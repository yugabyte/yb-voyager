INSERT INTO num_types(v1, v2, v3, v4, v5, v6)
VALUES (32767, 214748364, 922337203685477587, 123.456, 12345.6789, 123456.789);

UPDATE num_types
SET v1 = v1 * 0.5, v2 = v2 + 2, v3 = v3 * 2, v4 = v4 - 2, v5 = v5 * 2, v6 = v6 * 2;

UPDATE num_types
SET v1 = v1 - 100, v2 = v2 + 500, v3 = v3 - 1000, v5 = v5 + 5000, v6 = v6 - 10000.00::money;

DELETE FROM num_types WHERE v3 < 0;

insert into decimal_types values(0, 987455334362780682465462748789243337501610978301813276850553121352052192654789289113097427358778598.34278992, 12367890123456789.12);

UPDATE decimal_types
SET n1 = n1 + 1000, n2 = n2 - 100;

UPDATE decimal_types
SET n1 = n1 - 5012233222233.332220, n2 = n2 + 50;

INSERT INTO datatypes1(bool_type, char_type1, varchar_type, byte_type, enum_type)
VALUES (TRUE, 'A', 'Hello, world!', E'\\xDEABDAEF', 'Mon');

UPDATE datatypes1
SET bool_type = FALSE, char_type1 = 'X', varchar_type = 'Updated!', enum_type = 'Sun' where id=1;

DELETE FROM datatypes1 WHERE enum_type = 'Fri';

INSERT INTO datetime_type(v1, v2, v3, v4)
VALUES ('2024-02-07', '15:30:00', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT INTO datetime_type(v1, v2, v3, v4)
VALUES ('2024-03-15', '12:45:00', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day');

UPDATE datetime_type
SET v1 = '2024-02-08', v2 = '16:30:00', v3 = CURRENT_TIMESTAMP, v4 = CURRENT_TIMESTAMP where id=4;

DELETE FROM datetime_type WHERE EXTRACT(YEAR FROM v1) < 2000;

INSERT INTO datetime_type2(v1)
VALUES ('2024-02-07 12:00:00');

DELETE FROM datetime_type2 WHERE v1 = '2024-02-07 12:00:00';

INSERT INTO datatypes2(v1, v2, v3, v4, v5)
VALUES ('{"key": "value"}', B'1010101010', ARRAY[1, 2, 3, 4], '{{"a", "b"}, {"c", "d"}}', B'001010101');

INSERT INTO datatypes2(v1, v2, v3, v4, v5)
VALUES ('{"key": "value"}', B'0000101010', ARRAY[1, 2, 3, 4], '{{"a", "b"}, {"c", "d"}}', B'1010101010101010101010');

UPDATE datatypes2
SET v1 = '{"updated": true}', v2 = B'0101010101', v5 = B'101010101010101010101010101010', v3 = ARRAY[5, 6, 7, 8], v4 = '{{"e", "f"}, {"g", "h"}}'
WHERE v1 IS NULL;

UPDATE hstore_example 
SET data = data || 'key3 => value3'
WHERE id = 1;

UPDATE hstore_example 
SET data = hstore('{"key1=value1, key2=value2"}', '{"key1=value1, key2={"key1=value1, key2=value2"}"}')
WHERE id = 3;

UPDATE hstore_example 
SET data = '"{\"key1=value1, key2=value2\"}"=>"{\"key1=value1, key2={\"key1=value1, key2=value2\"}\"}"'
WHERE id = 7;

-- Insert new row, then update it later (intra-file dependency)
INSERT INTO hstore_example (data) 
VALUES ('k1=>v1');

-- Insert new row, then delete it later (intra-file dependency)
INSERT INTO hstore_example (data) 
VALUES ('temp_key=>temp_val');

-- More inserts for corner cases
INSERT INTO hstore_example (data) 
VALUES ('key5 => value5, key6 => value6');

INSERT INTO hstore_example (data) 
VALUES (hstore('{"key1=value1, key2=value2"}', '{"key1=value1, key2={"key1=value1, key2=value2"}"}'));

INSERT INTO hstore_example (data) 
VALUES ('');

INSERT INTO hstore_example (data) 
VALUES (
    'multi_key1=>"val1", "multi key 2"=>"value with spaces", "escaped\"quote"=>"line1\nline2", "unicode_αβ"=>"Ωmega"'
);

UPDATE hstore_example 
SET data = NULL
WHERE id = 5;

UPDATE hstore_example 
SET data = ''
WHERE id = 6;

-- Intra-file update on inserted row (chained updates)
UPDATE hstore_example 
SET data = data || 'k2=>v2'
WHERE data @> 'k1=>v1';

UPDATE hstore_example 
SET data = data || 'k3=>v3'
WHERE data @> 'k1=>v1';

DELETE FROM hstore_example WHERE data @> 'temp_key=>temp_val';
DELETE FROM hstore_example WHERE id = 2;
DELETE FROM hstore_example WHERE id = 4;
DELETE FROM hstore_example WHERE id = 8;
DELETE FROM hstore_example WHERE id = 9;

-- TSVECTOR table operations
INSERT INTO tsvector_table (title, content, title_tsv, content_tsv)
VALUES 
    ('Full Text Search', 
     'Full text search capabilities in PostgreSQL',
     to_tsvector('english', 'Full Text Search'),
     to_tsvector('english', 'Full text search capabilities in PostgreSQL'));

INSERT INTO tsvector_table (title, content, title_tsv, content_tsv)
VALUES 
    ('Database Performance', 
     'Optimizing database queries for better performance',
     to_tsvector('english', 'Database Performance'),
     to_tsvector('english', 'Optimizing database queries for better performance'));

UPDATE tsvector_table
SET content = 'Updated content about PostgreSQL database system',
    content_tsv = to_tsvector('english', 'Updated content about PostgreSQL database system')
WHERE id = 1;

DELETE FROM tsvector_table WHERE id = 2;

-- Enum array table operations
INSERT INTO enum_array_table (day_name, week_days, description)
VALUES ('Thu', ARRAY['Mon', 'Tue', 'Wed', 'Thu', 'Fri']::week[], 'All weekdays');

INSERT INTO enum_array_table (day_name, week_days, description)
VALUES ('Fri', ARRAY['Fri']::week[], 'Friday only');

UPDATE enum_array_table
SET week_days = ARRAY['Sat']::week[], description = 'Saturday only'
WHERE id = 1;

DELETE FROM enum_array_table WHERE id = 2;

-- Composite types table operations
INSERT INTO composite_types (address) VALUES (ROW('City1', 'Street 1', 1)::full_address), (ROW('City2', 'Street 2', 2)::full_address), (ROW('City3', 'Street 3', 3)::full_address);

UPDATE composite_types
SET address = ROW('UpdatedCity', 'UpdatedStreet', 99999)::full_address
WHERE id = 3;

DELETE FROM composite_types
WHERE id = 4;

-- Composite array types table operations
INSERT INTO composite_array_types (addresses)
VALUES (
    ARRAY[
        ROW('CityA1', 'StreetA 1', 1)::full_address,
        ROW('CityB1', 'StreetB 1', 2)::full_address
    ]
),
(
    ARRAY[
        ROW('CityA2', 'StreetA 2', 2)::full_address,
        ROW('CityB2', 'StreetB 2', 3)::full_address
    ]
),
(
    ARRAY[
        ROW('CityA3', 'StreetA 3', 3)::full_address,
        ROW('CityB3', 'StreetB 3', 4)::full_address
    ]
);

UPDATE composite_array_types
SET addresses = ARRAY[
    ROW('UpdatedCity1', 'UpdatedStreet1', 11111)::full_address,
    ROW('UpdatedCity2', 'UpdatedStreet2', 22222)::full_address
]
WHERE id = 3;

DELETE FROM composite_array_types
WHERE id = 5;

-- Domain types data
INSERT INTO domain_types (ssn, email, rating, prefs)
VALUES (
    '001-00-0004'::social_security_number,
    'user4@example.com'::email_address,
    4::rating_1_to_5,
    '{"version":"4","theme":"dark"}'::app_settings
),
(
    '002-00-0005'::social_security_number,
    'user5@example.com'::email_address,
    5::rating_1_to_5,
    '{"version":"5","theme":"dark"}'::app_settings
),
(
    '003-00-0006'::social_security_number,
    'user6@example.com'::email_address,
    5::rating_1_to_5,
    '{"version":"6","theme":"dark"}'::app_settings
);

UPDATE domain_types
SET
    ssn = '123-45-6789'::social_security_number,
    email = 'updated@example.com'::email_address,
    rating = 5::rating_1_to_5,
    prefs = '{"version":"updated","theme":"light"}'::app_settings
WHERE id = 4;

DELETE FROM domain_types
WHERE id = 5;

-- Domain array types data
INSERT INTO domain_array_types (ssn_list, phone_list, name_list)
VALUES (
    ARRAY[
        '123-45-0004'::social_security_number,
        '987-65-0004'::social_security_number
    ],
    ARRAY[
        '+91123456704'::phone_number,
        '+91987654304'::phone_number
    ],
    ARRAY[
        'ABCD DEFG'::full_name,
        'HIJK LMNO'::full_name
    ]
),
(
    ARRAY[
        '123-45-0005'::social_security_number,
        '987-65-0005'::social_security_number
    ],
    ARRAY[
        '+91123456705'::phone_number,
        '+91987654305'::phone_number
    ],
    ARRAY[
        'MNOP PQRS'::full_name,
        'TUV WXYZ'::full_name
    ]
),
(
    ARRAY[
        '123-45-0006'::social_security_number,
        '987-65-0006'::social_security_number
    ],
    ARRAY[
        '+91123456706'::phone_number,
        '+91987654306'::phone_number
    ],
    ARRAY[
        'ABCD XYZ'::full_name,
        'DEFG ABC'::full_name
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
WHERE id = 4;

DELETE FROM domain_array_types
WHERE id = 5;

-- Range Types
INSERT INTO range_types (
    price_range_col, discount_range_col, period_range_col, active_ts_range_col
)
VALUES (
    '[0.4,5.4)'::price_range,
    '[0.4,1.4)'::discount_range,
    ( '[' || (current_date + 4)::text || ',' || (current_date + 11)::text || ')' )::period_range,
    ( '[' || now()::text || ',' || (now() + interval '24 hours')::text || ')' )::active_ts_range
),
(
    '[0.5,5.5)'::price_range,
    '[0.5,1.5)'::discount_range,
    ( '[' || (current_date + 5)::text || ',' || (current_date + 12)::text || ')' )::period_range,
    ( '[' || (now() + interval '2 hours')::text || ',' ||
           (now() + interval '26 hours')::text || ')' )::active_ts_range
),
(
    '[0.6,5.6)'::price_range,
    '[0.6,1.6)'::discount_range,
    ( '[' || (current_date + 6)::text || ',' || (current_date + 13)::text || ')' )::period_range,
    ( '[' || (now() + interval '3 hours')::text || ',' ||
           (now() + interval '27 hours')::text || ')' )::active_ts_range
);

UPDATE range_types
SET
    price_range_col    = '[5.0,12.0)'::price_range,
    discount_range_col = '[0.2,1.0)'::discount_range,
    period_range_col   = ( '[' || (current_date + 15)::text || ',' || (current_date + 25)::text || ')' )::period_range,
    active_ts_range_col= ( '[' || (now() + interval '6 hours')::text || ',' ||
                                 (now() + interval '36 hours')::text || ')' )::active_ts_range
WHERE id = 4;

DELETE FROM range_types
WHERE id = 6;

-- Range array types
INSERT INTO range_array_types (
  price_ranges, discount_ranges, period_ranges, ts_ranges
)
VALUES (
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ( '[' || (current_date + 2003)::text || ',' || (current_date + 2004)::text || ')' )::period_range
  ],
  ARRAY[
    ( '[' || (now() + interval '2003 hours')::text || ',' ||
           (now() + interval '2004 hours')::text || ')' )::active_ts_range   -- FIXED upper > lower
  ]
),
(
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ( '[' || (current_date + 2004)::text || ',' || (current_date + 2005)::text || ')' )::period_range
  ],
  ARRAY[
    ( '[' || (now() + interval '2004 hours')::text || ',' ||
           (now() + interval '2005 hours')::text || ')' )::active_ts_range
  ]
),
(
  ARRAY['[1,5)'::price_range, '[10,20)'::price_range],
  ARRAY['[0.5,1.0)'::discount_range, '[1.5,2.0)'::discount_range],
  ARRAY[
    ( '[' || (current_date + 2005)::text || ',' || (current_date + 2006)::text || ')' )::period_range
  ],
  ARRAY[
    ( '[' || (now() + interval '2005 hours')::text || ',' ||
           (now() + interval '2006 hours')::text || ')' )::active_ts_range
  ]
);


UPDATE range_array_types
SET
  price_ranges    = ARRAY['[50,60)'::price_range],
  discount_ranges = ARRAY['[0.1,0.2)'::discount_range],
  period_ranges   = ARRAY[
      ( '[' || current_date::text || ',' || (current_date + 2)::text || ')' )::period_range
  ],
  ts_ranges       = ARRAY[
      ( '[' || now()::text || ',' || (now() + interval '3 hours')::text || ')' )::active_ts_range
  ]
WHERE id = 5;


DELETE FROM range_array_types
WHERE id = 6;

-- Extension types
INSERT INTO extension_types (col_hstore, col_citext, col_ltree)
VALUES (
    'key4=>4'::hstore,
    'text_4'::citext,
    'Top.4'::ltree
),
(
    'key5=>5'::hstore,
    'text_5'::citext,
    'Top.5'::ltree
),
(
    'key6=>6'::hstore,
    'text_6'::citext,
    'Top.6'::ltree
);

UPDATE extension_types
SET
    col_hstore = 'updated=>1'::hstore,
    col_citext = 'updated_text'::citext,
    col_ltree  = 'Updated.Node'::ltree
WHERE id = 4;

DELETE FROM extension_types
WHERE id = 6;

-- Extension arrays
INSERT INTO extension_arrays (col_hstore, col_citext, col_ltree)
VALUES (
    ARRAY['key4=>4'::hstore, 'value4=>4'::hstore],
    ARRAY['text_4'::citext, 'sample_4'::citext],
    ARRAY['Top.4'::ltree, 'Category.4'::ltree]
),
(
    ARRAY['key5=>5'::hstore, 'value5=>5'::hstore],
    ARRAY['text_5'::citext, 'sample_5'::citext],
    ARRAY['Top.5'::ltree, 'Category.5'::ltree]
),
(
    ARRAY['key6=>6'::hstore, 'value6=>6'::hstore],
    ARRAY['text_6'::citext, 'sample_6'::citext],
    ARRAY['Top.6'::ltree, 'Category.6'::ltree]
);

UPDATE extension_arrays
SET 
    col_hstore = ARRAY['a=>1'::hstore, 'b=>2'::hstore],
    col_citext = ARRAY['updated1'::citext, 'updated2'::citext],
    col_ltree  = ARRAY['Top.Science'::ltree, 'Top.Arts'::ltree]
WHERE id = 5;

DELETE FROM extension_arrays
WHERE id = 6;

-- Nested datatypes
INSERT INTO audit_log (
  involved_employees, affected_clients, transaction_refs
)
VALUES (
  ARRAY[
    ROW('Emp_3', 'active', 'emp_3@company.com')::employee_info,
    ROW('EmpAlt_3', 'inactive', 'empalt_3@company.com')::employee_info
  ],
  ARRAY[
    ROW('Client_3', 'north', 'client_3@example.com')::client_info,
    ROW('ClientAlt_3', 'south', 'clientalt_3@example.com')::client_info
  ],
  ARRAY[3, 4, 5]
);

UPDATE audit_log
SET
  involved_employees = ARRAY[
    ROW('UpdatedEmp_A', 'active', 'updated_a@company.com')::employee_info,
    ROW('UpdatedEmp_B', 'terminated', 'updated_b@company.com')::employee_info
  ],
  affected_clients = ARRAY[
    ROW('UpdatedClient_A', 'east', 'client_a_updated@example.com')::client_info,
    ROW('UpdatedClient_B', 'west', 'client_b_updated@example.com')::client_info
  ],
  transaction_refs = ARRAY[100, 200, 300]
WHERE id = 2;

DELETE FROM audit_log WHERE id = 3;

-- Numeric types
INSERT INTO numeric_types (real_col, double_col, small_serial_col, big_serial_col) VALUES (9.9, 8.8, 10, 11);

UPDATE numeric_types SET real_col = 7.7, double_col = 6.6, small_serial_col = 12, big_serial_col = 13 WHERE id = 2;

DELETE FROM numeric_types WHERE id = 3;

-- Numeric arrays
INSERT INTO numeric_arrays (real_col, double_col) VALUES (ARRAY[9.9, 8.8, 7.7], ARRAY[6.6, 5.5, 4.4]);

UPDATE numeric_arrays SET real_col = ARRAY[3.3, 2.2, 1.1], double_col = ARRAY[9.9, 9.1, 8.2] WHERE id = 2;

DELETE FROM numeric_arrays WHERE id = 3;

-- Datetime types
INSERT INTO datetime_types (timestamptz_col, timetz_col, interval_col) VALUES ('2026-06-10 12:45:00+04', '08:05:00+04:30', '11 hours');

UPDATE datetime_types SET timestamptz_col = '2028-01-01 05:00:00+07', interval_col = '22 minutes' WHERE id = 2;

DELETE FROM datetime_types WHERE id = 3;

-- Datetime arrays
INSERT INTO datetime_arrays (timestamptz_col, timetz_col, interval_col) VALUES (ARRAY['2029-03-01 01:00:00+00','2030-05-05 05:45:00+06']::timestamptz[], ARRAY['11:22:33+02:00','04:55:10-01:00']::timetz[], ARRAY['7 hours','19 minutes']::interval[]);

UPDATE datetime_arrays SET timestamptz_col = ARRAY['2031-07-07 07:00:00+03','2032-08-08 08:30:00-02']::timestamptz[], interval_col = ARRAY['15 minutes','27 hours']::interval[] WHERE id = 2;

DELETE FROM datetime_arrays WHERE id = 3;

-- Geometry types
INSERT INTO geometry_types (point_col, line_col, lseg_col, box_col, path_col, polygon_col, circle_col)
VALUES (
  '(12,14)',
  '[(2,2),(9,9)]',
  '[(3,4),(12,15)]',
  '((2,3),(8,11))',
  '[(3,3),(9,12)]',
  '((2,2),(10,2),(10,9),(2,9))',
  '<(6,8),4.5>'
);

UPDATE geometry_types
SET point_col = '(14,15)',
    line_col = '[(4,5),(14,16)]',
    lseg_col = '[(5,6),(18,20)]',
    box_col = '((3,4),(12,13))',
    path_col = '[(6,6),(13,14)]',
    polygon_col = '((3,3),(11,3),(11,11),(3,11))',
    circle_col = '<(9,10),7.5>'
WHERE id = 2;

DELETE FROM geometry_types WHERE id = 3;

-- Geometry arrays
INSERT INTO geometry_arrays (point_col, line_col, lseg_col, box_col, path_col, polygon_col, circle_col)
VALUES (
  ARRAY['(5,5)','(6,6)','(7,7)']::point[],
  ARRAY['[(4,4),(6,6)]']::line[],
  ARRAY['[(2,6),(10,12)]']::lseg[],
  ARRAY['((3,3),(9,9))']::box[],
  ARRAY['[(4,4),(7,10)]']::path[],
  ARRAY['((4,4),(9,4),(9,9),(4,9))']::polygon[],
  ARRAY['<(4,4),3>','<(9,9),5>']::circle[]
);

UPDATE geometry_arrays
SET point_col = ARRAY['(8,8)','(9,9)']::point[],
    line_col  = ARRAY['[(5,5),(9,9)]']::line[],
    lseg_col  = ARRAY['[(6,6),(12,16)]']::lseg[],
    box_col   = ARRAY['((4,4),(10,12))']::box[],
    path_col  = ARRAY['[(5,5),(11,13)]']::path[],
    polygon_col = ARRAY['((5,5),(12,5),(12,12),(5,12))']::polygon[],
    circle_col = ARRAY['<(5,5),4>']::circle[]
WHERE id = 2;

DELETE FROM geometry_arrays WHERE id = 3;

-- Network types
INSERT INTO network_types (cidr_col, inet_col, macaddr_col, macaddr8_col)
VALUES ('8.8.0.0/16','8.8.8.8','AA:00:11:22:33:44','AA:00:11:FF:FE:22:33:44');

UPDATE network_types
SET cidr_col = '6.6.0.0/16',
    inet_col = '6.6.6.6',
    macaddr_col = 'BB:22:33:44:55:66',
    macaddr8_col = 'BB:22:33:FF:FE:44:55:66'
WHERE id = 2;

DELETE FROM network_types WHERE id = 3;

-- Network arrays
INSERT INTO network_arrays (cidr_col, inet_col, macaddr_col, macaddr8_col)
VALUES (
  ARRAY['6.0.0.0/8','6.10.0.0/16','6.20.30.0/24']::cidr[],
  ARRAY['6.0.0.1','6.10.0.5','6.20.30.99']::inet[],
  ARRAY['CC:11:22:33:44:55','CC:66:77:88:99:AA','CC:BB:CC:DD:EE:FF']::macaddr[],
  ARRAY['CC:11:22:FF:FE:33:44:55','CC:66:77:FF:FE:88:99:AA','CC:BB:CC:FF:FE:DD:EE:FF']::macaddr8[]
);

UPDATE network_arrays
SET cidr_col = ARRAY['7.0.0.0/8','7.10.0.0/16']::cidr[],
    inet_col = ARRAY['7.0.0.1','7.10.0.5']::inet[],
    macaddr_col = ARRAY['DD:11:22:33:44:55','DD:66:77:88:99:AA']::macaddr[],
    macaddr8_col = ARRAY['DD:11:22:FF:FE:33:44:55','DD:66:77:FF:FE:88:99:AA']::macaddr8[]
WHERE id = 2;

DELETE FROM network_arrays WHERE id = 3;

-- Misc types
INSERT INTO misc_types (pg_lsn_col, txid_snapshot_col)
VALUES ('0/ABCDEF0','350:360:355');

UPDATE misc_types
SET pg_lsn_col = '0/1234500',
    txid_snapshot_col = '120:130:125'
WHERE id = 2;

DELETE FROM misc_types WHERE id = 3;

-- Misc arrays
INSERT INTO misc_arrays (pg_lsn_col, txid_snapshot_col)
VALUES (
  ARRAY['0/AAA0000','0/BBB0001','0/CCC0002']::pg_lsn[],
  ARRAY['300:305:302','400:410:405','600:610:608']::txid_snapshot[]
);

UPDATE misc_arrays
SET pg_lsn_col = ARRAY['0/DDD0000','0/EEE0002']::pg_lsn[],
    txid_snapshot_col = ARRAY['700:705:702','800:810:805']::txid_snapshot[]
WHERE id = 2;

DELETE FROM misc_arrays WHERE id = 3;
