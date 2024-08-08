-- https://www.postgresql.org/docs/current/sql-createdomain.html
CREATE USER domain_owner;
CREATE SCHEMA domain_examples;

CREATE DOMAIN domain_examples.us_postal_code AS TEXT
  CONSTRAINT check_postal_code_regex
    CHECK(VALUE ~ '^\d{5}$' OR VALUE ~ '^\d{5}-\d{4}$');
COMMENT ON DOMAIN domain_examples.us_postal_code is 'US postal code';

CREATE TABLE domain_examples.us_snail_addy (
  address_id SERIAL PRIMARY KEY
  , street1 TEXT NOT NULL
  , street2 TEXT
  , street3 TEXT
  , city TEXT NOT NULL
  , postal domain_examples.us_postal_code NOT NULL
);


CREATE FUNCTION domain_examples.is_positive(i INTEGER) RETURNS BOOLEAN IMMUTABLE AS $$
  SELECT i > 0
$$ LANGUAGE SQL;

CREATE DOMAIN domain_examples.positive_number AS INTEGER
  CONSTRAINT should_be_positive
    CHECK(domain_examples.is_positive(VALUE));

CREATE FUNCTION domain_examples.is_even(i domain_examples.positive_number) RETURNS BOOLEAN IMMUTABLE AS $$
  SELECT (i % 2) = 0
$$ LANGUAGE SQL;

CREATE DOMAIN domain_examples.positive_even_number AS domain_examples.positive_number
  CONSTRAINT should_be_even
    CHECK(domain_examples.is_even(VALUE));

CREATE TABLE domain_examples.even_numbers (e domain_examples.positive_even_number);
-- CREATE INDEX domain_examples.even_number_idx ON domain_examples.even_numbers(e);
