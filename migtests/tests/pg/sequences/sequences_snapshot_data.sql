insert into sequence_check1(name, balance) values('def', 10000);

insert into sequence_check1(name, balance) values('abc', 10000);


insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');


insert into sequence_check3(name) values('abc');
insert into sequence_check3(name) values('abc');
insert into sequence_check3(name) values('abc');


insert into multiple_identity_columns(name, balance, name2, balance2) values('def', 10000, 'def', 10000);
insert into multiple_identity_columns(name, balance, name2, balance2) values('abc', 10000, 'abc', 10000);


insert into multiple_serial_columns(name, balance, name2, balance2) values('def', 10000, 'def', 10000);
insert into multiple_serial_columns(name, balance, name2, balance2) values('abc', 10000, 'abc', 10000);


insert into "Case_Sensitive_Seq"(name, balance, name2, balance2) values('def', 10000, 'def', 10000);
insert into "Case_Sensitive_Seq"(name, balance, name2, balance2) values('abc', 10000, 'abc', 10000);



insert into schema1.sequence_check1(name, balance) values('def', 10000);

insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into schema1.sequence_check2 (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');



insert into schema1.sequence_check3(name) values('abc');
insert into schema1.sequence_check3(name) values('abc');


insert into schema1.multiple_identity_columns(name, balance, name2, balance2) values('def', 10000, 'def', 10000);
insert into schema1.multiple_identity_columns(name, balance, name2, balance2) values('abc', 10000, 'abc', 10000);


insert into schema1.multiple_serial_columns(name, balance, name2, balance2) values('def', 10000, 'def', 10000);
insert into schema1.multiple_serial_columns(name, balance, name2, balance2) values('abc', 10000, 'abc', 10000);

insert into foo (value) values ('Hello');
insert into bar (value) values (now());
insert into foo (value) values ('World');
insert into bar (value) values (now());


insert into schema2.foo (value) values ('Hello');
insert into schema2.bar (value) values (now());
insert into schema2.foo (value) values ('World');
insert into schema2.bar (value) values (now());




insert into schema3.foo (value) values ('Hello');
insert into schema4.bar (value) values (now());
insert into schema3.foo (value) values ('World');
insert into schema4.bar (value) values (now());

insert into foo_bar (value, value2) values ('Hello', 'World');
insert into foo_bar (value, value2) values ('World', 'Hello');


WITH region_list AS (
     SELECT '{"London", "Boston", "Sydney"}'::TEXT[] region
     ), amount_list AS (
        SELECT '{1000, 2000, 5000}'::INT[] amount
        ) 
        INSERT INTO sales_region  
        (amount, branch, region) 
            SELECT 
                amount[1 + mod(n, array_length(amount, 1))], 
                'Branch ' || n as branch, 
                region[1 + mod(n, array_length(region, 1))] 
                    FROM amount_list, region_list, generate_series(1,1000) as n;


INSERT INTO users (name) 
VALUES ('John Doe');

INSERT INTO users (name) 
VALUES ('ABC');

INSERT INTO users (name) 
VALUES ('XYZ');

SELECT * FROM users;
