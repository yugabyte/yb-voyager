-- TODO: create user as per User creation steps in docs and use that in tests

-- Grant CREATE, ALTER, DROP privileges globally to 'ybvoyager'
GRANT CREATE, ALTER, DROP ON *.* TO 'ybvoyager'@'%' WITH GRANT OPTION;
-- Apply the changes
FLUSH PRIVILEGES;