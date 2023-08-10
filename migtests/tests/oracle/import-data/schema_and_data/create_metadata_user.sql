BEGIN
    DECLARE
        user_exists NUMBER;
    BEGIN
        SELECT COUNT(*) INTO user_exists FROM all_users WHERE username = UPPER('ybvoyager_metadata');
        IF user_exists = 0 THEN
            EXECUTE IMMEDIATE 'CREATE USER ybvoyager_metadata IDENTIFIED BY "password"';
        END IF;
    END;
END;
