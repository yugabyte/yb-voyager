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