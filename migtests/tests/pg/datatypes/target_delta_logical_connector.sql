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




