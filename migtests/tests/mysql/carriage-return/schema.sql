CREATE TABLE foo (
    id   INTEGER PRIMARY KEY,
    value TEXT
);

INSERT INTO foo (id, value) VALUES (1, '\r\nText with \r');
INSERT INTO foo (id, value) VALUES (2, '\r\nText with \n');
INSERT INTO foo (id, value) VALUES (3, '\r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (4, '\r\nText with \\r');
INSERT INTO foo (id, value) VALUES (5, '\r\nText with \\n');
INSERT INTO foo (id, value) VALUES (6, '\r\nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (7, 'Text with \r\n');
INSERT INTO foo (id, value) VALUES (8, 'Text with \r\nText with \r');
INSERT INTO foo (id, value) VALUES (9, 'Text with \r\nText with \n');
INSERT INTO foo (id, value) VALUES (10, 'Text with \r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (11, 'Text with \r\nText with \\r');
INSERT INTO foo (id, value) VALUES (12, 'Text with \r\nText with \\n');
INSERT INTO foo (id, value) VALUES (13, 'Text with \r\nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (14, 'Text with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (15, 'Text with \rText with \n');
INSERT INTO foo (id, value) VALUES (16, 'Text with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (17, 'Text with \rText with \\r');
INSERT INTO foo (id, value) VALUES (18, 'Text with \rText with \\n');
INSERT INTO foo (id, value) VALUES (19, 'Text with \rText with \\r\\n');

INSERT INTO foo (id, value) VALUES (20, 'Text with \nText with \r');
INSERT INTO foo (id, value) VALUES (21, 'Text with \nText with \n');
INSERT INTO foo (id, value) VALUES (22, 'Text with \nText with \r\n');
INSERT INTO foo (id, value) VALUES (23, 'Text with \nText with \\r');
INSERT INTO foo (id, value) VALUES (24, 'Text with \nText with \\n');
INSERT INTO foo (id, value) VALUES (25, 'Text with \nText with \\r\\n');

INSERT INTO foo (id, value) VALUES (26, 'Text with \r\nText with \rText with \r\n');
INSERT INTO foo (id, value) VALUES (27, 'Text with \r\nText with \nText with \r\n');
INSERT INTO foo (id, value) VALUES (28, 'Text with \r\nText with \r\nText with \r\n');
INSERT INTO foo (id, value) VALUES (29, 'Text with \r\nText with \\rText with \r\n');
INSERT INTO foo (id, value) VALUES (30, 'Text with \r\nText with \\nText with \r\n');
INSERT INTO foo (id, value) VALUES (31, 'Text with \r\nText with \\r\\nText with \r\n');
