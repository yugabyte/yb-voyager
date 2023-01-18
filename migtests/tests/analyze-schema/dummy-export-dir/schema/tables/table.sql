CREATE TABLE salaries2 (
	emp_no bigint NOT NULL,
	salary bigint NOT NULL,
	from_date timestamp NOT NULL,
	to_date timestamp NOT NULL,
	PRIMARY KEY (emp_no,from_date)
) PARTITION BY RANGE (((from_date)::date - '0001-01-01bc')::integer) ;

CREATE TABLE sales (
	cust_id bigint NOT NULL,
	name varchar(40),
	store_id varchar(20) NOT NULL,
	bill_no bigint NOT NULL,
	bill_date timestamp NOT NULL,
	amount decimal(8,2) NOT NULL,
	PRIMARY KEY (bill_date)
) PARTITION BY RANGE (extract(year from date(bill_date))) ;

-- all partition keys are not included in the PK constraint, to be reported during analyze-schema
CREATE TABLE test_1 (
	id numeric NOT NULL,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50),
	PRIMARY KEY (id)
) PARTITION BY LIST (country_code, record_type) ;

-- all partition keys are not included in the PK constraint, to be reported during analyze-schema
CREATE TABLE test_2 (
	id numeric NOT NULL PRIMARY KEY,
	country_code varchar(3),
	record_type varchar(5),
	descriptions varchar(50)
) PARTITION BY LIST (country_code, record_type) ;

-- no PK constraint, no need to report during analyze-schema
CREATE TABLE test_3 (
	id numeric,
	country_code varchar,
	record_type varchar(2)
) PARTITION BY RANGE (id) ;

--  no PK constraint, no need to report during analyze-schema
CREATE TABLE test_4 (
	id numeric,
	country_code varchar,
	record_type varchar
) PARTITION BY LIST (id) ;