SELECT 'CREATE USER ybvoyager SUPERUSER PASSWORD ''Test@123#$%^&*()!'''
WHERE NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'ybvoyager')\gexec
