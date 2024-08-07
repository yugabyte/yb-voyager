-- https://www.postgresql.org/docs/current/sql-createcollation.html
CREATE SCHEMA collation_ex;
CREATE COLLATION collation_ex.us (locale = 'en_US.utf8');
CREATE COLLATION collation_ex.german_phonebook (provider = icu, locale = 'de-u-co-phonebk');
CREATE COLLATION collation_ex.bad_us FROM "collation_ex"."us";
