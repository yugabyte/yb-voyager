# We use an already deployed RDS Oracle instance for testing.
# ssinghal-dms-oracle: https://us-west-2.console.aws.amazon.com/rds/home?region=us-west-2#database:id=ssinghal-dms-oracle;is-cluster=false
export SOURCE_DB_HOST=${SOURCE_DB_HOST:-"35.165.118.74"}
export SOURCE_DB_PORT=${SOURCE_DB_PORT:-1521}
export SOURCE_DB_USER=${SOURCE_DB_USER:-"sakila_demo"}
export SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD:-"password"}
export SOURCE_DB_SCHEMA=${SOURCE_DB_SCHEMA:-"sakila_demo"}