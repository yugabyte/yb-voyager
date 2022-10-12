# We use an already deployed RDS Oracle instance for testing.
# ssinghal-dms-oracle: https://us-west-2.console.aws.amazon.com/rds/home?region=us-west-2#database:id=ssinghal-dms-oracle;is-cluster=false
export SOURCE_DB_TYPE="oracle"
export SOURCE_DB_HOST="cnaidu-oracle-database.cbtcvpszcgdq.us-west-2.rds.amazonaws.com"
export SOURCE_DB_PORT="1521"
export SOURCE_DB_USER="ADMIN"
export SOURCE_DB_PASSWORD="Password1"
export SOURCE_DB_NAME="ORCL"
export SOURCE_DB_SCHEMA="ADMIN"
