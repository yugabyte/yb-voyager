CREATE TABLE TRUNC_TEST_1 (id integer primary key, 
                           d1 DATE,
                           d2 DATE,
                           d3 DATE,
                           tz1 TIMESTAMP WITH TIME ZONE,
                           tz2 TIMESTAMP WITH TIME ZONE);

DECLARE
    d1_date DATE;
    d2_date DATE;
    d3_date DATE;
    tz1_tz TIMESTAMP WITH TIME ZONE;
    tz2_ts TIMESTAMP WITH TIME ZONE;
BEGIN
    SELECT TRUNC(TO_DATE('02-MAR-15','DD-MON-YY'), 'mi') into d1_date FROM DUAL;
    SELECT TRUNC(TO_DATE('02-MAR-16','DD-MON-YY'), 'ww') into d2_date FROM DUAL;
    SELECT TRUNC(TO_DATE('02-MAR-17','DD-MON-YY'), 'iw') into d3_date FROM DUAL;
    SELECT TRUNC(TO_TIMESTAMP('02-MAR-18','DD-MON-YY'), 'mm') into tz1_tz FROM DUAL;
    SELECT TRUNC(TO_TIMESTAMP('02-MAR-19','DD-MON-YY'), 'YEAR') into tz2_ts FROM DUAL;
    INSERT INTO TRUNC_TEST_1 VALUES (d1_date,d2_date,d3_date,tz1_tz,tz2_ts);
END;
/