Create Table `order` (
    id int PRIMARY KEY,
    name varchar(10)
);

Create Table `user` (
    id int PRIMARY KEY,
    name varchar(10)
);


Create Table `group` (
    id int PRIMARY KEY,
    name varchar(10)
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