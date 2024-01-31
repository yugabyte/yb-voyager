CREATE TABLE foo (
    id   INTEGER PRIMARY KEY,
    value TEXT
);

INSERT INTO foo (id, value) VALUES (1, E'\r\nText with \r');
INSERT INTO foo (id, value) VALUES (2, E'\r\nText with \n');
INSERT INTO foo (id, value) VALUES (3, E'\r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (4, E'\r\nText with \\r');
INSERT INTO foo (id, value) VALUES (5, E'\r\nText with \\n');
INSERT INTO foo (id, value) VALUES (6, E'\r\nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (7, E'Text with \r\n');
INSERT INTO foo (id, value) VALUES (8, E'Text with \r\nText with \r');
INSERT INTO foo (id, value) VALUES (9, E'Text with \r\nText with \n');
INSERT INTO foo (id, value) VALUES (10, E'Text with \r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (11, E'Text with \r\nText with \\r');
INSERT INTO foo (id, value) VALUES (12, E'Text with \r\nText with \\n');
INSERT INTO foo (id, value) VALUES (13, E'Text with \r\nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (14, E'Text with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (15, E'Text with \rText with \n');
INSERT INTO foo (id, value) VALUES (16, E'Text with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (17, E'Text with \rText with \\r');
INSERT INTO foo (id, value) VALUES (18, E'Text with \rText with \\n');
INSERT INTO foo (id, value) VALUES (19, E'Text with \rText with \\r\\n');

INSERT INTO foo (id, value) VALUES (20, E'Text with \nText with \r');
INSERT INTO foo (id, value) VALUES (21, E'Text with \nText with \n');
INSERT INTO foo (id, value) VALUES (22, E'Text with \nText with \r\n');
INSERT INTO foo (id, value) VALUES (23, E'Text with \nText with \\r');
INSERT INTO foo (id, value) VALUES (24, E'Text with \nText with \\n');
INSERT INTO foo (id, value) VALUES (25, E'Text with \nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (26, E'Text with \r\nText with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (27, E'Text with \r\nText with \nText with \r\n');
INSERT INTO foo (id, value) VALUES (28, E'Text with \r\nText with \r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (29, E'Text with \r\nText with \\rText with \r\n');
INSERT INTO foo (id, value) VALUES (30, E'Text with \r\nText with \\nText with \r\n');
INSERT INTO foo (id, value) VALUES (31, E'Text with \r\nText with \\r\\nText with \r\n');
