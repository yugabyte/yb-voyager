-- This test contains reserved words and case sensitive tables/columns

CREATE TABLE `order` (
    id int PRIMARY KEY,
    name varchar(10)
);

CREATE TABLE `user` (
    id int PRIMARY KEY,
    name varchar(10)
);


CREATE TABLE `group` (
    id int PRIMARY KEY,
    name varchar(10)
);

CREATE TABLE reserved_column (
    `user` int,
    `case` text
);

CREATE TABLE `check` (
    `user` int,
    `case` text
);

INSERT into `order` values(1, 'abc');
INSERT into `order` values(2, 'abc');
INSERT into `order` values(3, 'abc');
INSERT into `order` values(4, 'abc');
INSERT into `order` values(5, 'abc');

INSERT into `user` values(1, 'abc');
INSERT into `user` values(2, 'abc');
INSERT into `user` values(3, 'abc');
INSERT into `user` values(4, 'abc');
INSERT into `user` values(5, 'abc');

INSERT into `group` values(1, 'abc');
INSERT into `group` values(2, 'abc');
INSERT into `group` values(3, 'abc');
INSERT into `group` values(4, 'abc');
INSERT into `group` values(5, 'abc');

INSERT into reserved_column values(1, 'abc');
INSERT into reserved_column values(2, 'abc');
INSERT into reserved_column values(3, 'abc');
INSERT into reserved_column values(4, 'abc');
INSERT into reserved_column values(5, 'abc');

INSERT into `check` values(1, 'abc');
INSERT into `check` values(2, 'abc');
INSERT into `check` values(3, 'abc');
INSERT into `check` values(4, 'abc');
INSERT into `check` values(5, 'abc');

drop table if exists session_log;

create table session_log 
( 
   userid int not null, 
   phonenumber int
); 


create table session_log1 
( 
   userid int not null, 
   phonenumber int
); 


create table session_log2 
( 
   userid int not null, 
   phonenumber int
); 

create table session_log3 
( 
   userid int not null, 
   phonenumber int
); 

create table session_log4 
( 
   userid int not null, 
   phonenumber int
); 


DELIMITER $$
CREATE PROCEDURE insert_session_logs()
BEGIN
    DECLARE i INT DEFAULT 1;

    WHILE i <= 100 DO
        INSERT INTO session_log VALUES (i, i);
        INSERT INTO session_log1 VALUES (i, i);
        INSERT INTO session_log2 VALUES (i, i);
        INSERT INTO session_log3 VALUES (i, i);
        INSERT INTO session_log4 VALUES (i, i);
        SET i = i + 1;
    END WHILE;
END $$
DELIMITER ;

CALL insert_session_logs();

drop table if exists `Mixed_Case_Table_Name_Test`;

CREATE TABLE `Mixed_Case_Table_Name_Test` (
	id int primary key auto_increment,
	first_name VARCHAR(50) not null,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into `Mixed_Case_Table_Name_Test` (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');

drop table if exists `Case_Sensitive_Columns`;

CREATE TABLE `Case_Sensitive_Columns` (
	id int primary key auto_increment,
	`First_Name` VARCHAR(50) not null,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');

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

INSERT INTO foo (id, value) VALUES (32, '\n\rText with \r');
INSERT INTO foo (id, value) VALUES (33, 'Text with \n\r');
INSERT INTO foo (id, value) VALUES (34, 'Text with \r\nText with \n\r');


