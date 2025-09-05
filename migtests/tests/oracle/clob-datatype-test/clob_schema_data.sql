CREATE TABLE clob_test (
    id        NUMBER PRIMARY KEY,
    c_text    CLOB,
    n_text    NCLOB,
    b_data    BLOB
);

-- Multi-line text with quotes and backslashes
INSERT INTO clob_test (id, c_text)
VALUES (1, 'Multi-line CLOB test:
Line 2: Special chars ''single quotes'', "double quotes", backslashes \\
Line 3: Testing line breaks and escaping
End of quote/backslash test.');

-- JSON-like structure with nested braces and escaped quotes
INSERT INTO clob_test (id, c_text)
VALUES (2, '{"user": "testuser", "data": {"nested": "value", "quotes": ""escaped quotes""}, "symbols": "@#$%^&*()"}');

-- SQL injection attempt with numbers and symbols
INSERT INTO clob_test (id, c_text)
VALUES (3, 'Numbers: 123-456-789, Symbols: @#$%^&*()
SQL injection: DROP TABLE users; DELETE FROM important; --comment
End of injection test.');

-- Empty CLOB using EMPTY_CLOB() function
INSERT INTO clob_test (id, c_text)
VALUES (4, EMPTY_CLOB());

-- All LOB types: CLOB, NCLOB, BLOB (Note: NCLOB and BLOB data not exported)
INSERT INTO clob_test (id, c_text, n_text, b_data)
VALUES (
    5,
    'Simple CLOB text for LOB types test',
    N'NCLOB test data',  -- NCLOB data (will not be exported)
    UTL_RAW.CAST_TO_RAW('BinaryDataHere') -- BLOB data (will not be exported)
);

-- Large CLOB generation: 3 rows of ~200MB CLOBs filled with 'X' characters
DECLARE
  l_clob   CLOB;
  l_chunk  VARCHAR2(32767);
  l_size_mb PLS_INTEGER := 200;  -- target size in MB
  l_id     NUMBER := 100;        -- starting ID
BEGIN
  -- Create a chunk of 32 KB filled with 'X' characters
  l_chunk := RPAD('X', 32767, 'X');

  -- Insert multiple rows of big CLOBs
  FOR i IN 1..3 LOOP  -- IDs: 101, 102, 103
    DBMS_LOB.CREATETEMPORARY(l_clob, TRUE);
    
    -- Append chunks until ~200MB is reached
    FOR j IN 1..(l_size_mb * 1024 * 1024 / LENGTH(l_chunk)) LOOP
      DBMS_LOB.APPEND(l_clob, l_chunk);
    END LOOP;
    
    INSERT INTO clob_test (id, c_text)
    VALUES (l_id + i, l_clob);
    
    DBMS_LOB.FREETEMPORARY(l_clob);
  END LOOP;
  
  COMMIT;
END;
/
