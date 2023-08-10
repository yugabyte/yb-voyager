CREATE USER ybvoyager_metadata IDENTIFIED BY password;
GRANT CONNECT, RESOURCE TO ybvoyager_metadata;
ALTER USER ybvoyager_metadata QUOTA UNLIMITED ON USERS;
