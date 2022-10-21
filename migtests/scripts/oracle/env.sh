# We use an already deployed RDS Oracle instance for testing.
# ssinghal-dms-oracle: https://us-west-2.console.aws.amazon.com/rds/home?region=us-west-2#database:id=ssinghal-dms-oracle;is-cluster=false
export SOURCE_DB_HOST=${SOURCE_DB_HOST:-"10.9.8.213"}
export SOURCE_DB_PORT=${SOURCE_DB_PORT:-1521}
export SOURCE_DB_USER=${SOURCE_DB_USER:-"ADMIN"}
export SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD:-"Password1"}
export SOURCE_DB_SCHEMA=${SOURCE_DB_SCHEMA:-"ADMIN"}
