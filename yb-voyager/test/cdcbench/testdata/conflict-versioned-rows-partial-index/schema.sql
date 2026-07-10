DROP TABLE IF EXISTS versions CASCADE;
CREATE TABLE versions (id int PRIMARY KEY, entity_id int, most_recent boolean, payload text);
CREATE UNIQUE INDEX versions_entity_uk ON versions (entity_id) WHERE most_recent;
ALTER TABLE versions REPLICA IDENTITY FULL;
