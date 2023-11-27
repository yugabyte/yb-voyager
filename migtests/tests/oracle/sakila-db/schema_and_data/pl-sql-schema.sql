BEGIN
  FOR r IN (SELECT * FROM user_types) LOOP
    EXECUTE IMMEDIATE 'DROP TYPE ' || r.type_name || ' FORCE';
  END LOOP;
END;
/

CREATE TYPE LANGUAGE_T AS OBJECT (
  language_id SMALLINT,
  name CHAR(20),
  last_update TIMESTAMP
);
/

CREATE TYPE LANGUAGES_T AS TABLE OF LANGUAGE_T;
/

CREATE TYPE FILM_T AS OBJECT (
  film_id int,
  title VARCHAR(255),
  description CLOB,
  release_year VARCHAR(4),
  language LANGUAGE_T,
  original_language LANGUAGE_T,
  rental_duration SMALLINT,
  rental_rate DECIMAL(4,2),
  length SMALLINT,
  replacement_cost DECIMAL(5,2),
  rating VARCHAR(10),
  special_features VARCHAR(100),
  last_update TIMESTAMP
);
/

CREATE TYPE FILMS_T AS TABLE OF FILM_T;
/

CREATE TYPE ACTOR_T AS OBJECT (
  actor_id numeric,
  first_name VARCHAR(45),
  last_name VARCHAR(45),
  last_update TIMESTAMP
);
/

CREATE TYPE ACTORS_T AS TABLE OF ACTOR_T;
/

CREATE TYPE CATEGORY_T AS OBJECT (
  category_id SMALLINT,
  name VARCHAR(25),
  last_update TIMESTAMP
);
/

CREATE TYPE CATEGORIES_T AS TABLE OF CATEGORY_T;
/

CREATE TYPE FILM_INFO_T AS OBJECT (
  film FILM_T,
  actors ACTORS_T,
  categories CATEGORIES_T
);
/

CREATE TYPE COUNTRY_T AS OBJECT (
  country_id SMALLINT,
  country VARCHAR(50),
  last_update TIMESTAMP
);
/

CREATE TYPE CITY_T AS OBJECT (
  city_id int,
  city VARCHAR(50),
  country COUNTRY_T,
  last_update TIMESTAMP
);
/

CREATE TYPE ADDRESS_T AS OBJECT (
  address_id int,
  address VARCHAR(50),
  address2 VARCHAR(50),
  district VARCHAR(20),
  city CITY_T,
  postal_code VARCHAR(10),
  phone VARCHAR(20),
  last_update TIMESTAMP
);
/

CREATE TYPE CUSTOMER_T AS OBJECT (
  customer_id INT,
  first_name VARCHAR(45),
  last_name VARCHAR(45),
  email VARCHAR(50),
  address ADDRESS_T,
  active CHAR(1),
  create_date TIMESTAMP,
  last_update TIMESTAMP
);
/

CREATE TYPE CUSTOMERS_T AS TABLE OF CUSTOMER_T;
/

CREATE TYPE CUSTOMER_RENTAL_HISTORY_T AS OBJECT (
  customer CUSTOMER_T,
  films FILMS_T
);
/


