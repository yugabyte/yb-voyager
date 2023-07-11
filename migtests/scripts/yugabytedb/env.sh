export TARGET_DB_HOST=${TARGET_DB_HOST:-"172.151.16.201"}
export TARGET_DB_PORT=${TARGET_DB_PORT:-5433}
export TARGET_DB_USER=${TARGET_DB_USER:-"ybvoyager"}
export TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD:-'password'}
export TARGET_DB_ADMIN_USER=${TARGET_DB_ADMIN_USER:-"yugabyte"}
export TARGET_DB_ADMIN_PASSWORD=${TARGET_DB_ADMIN_PASSWORD:-''}
export TARGET_DB_SCHEMA=${TARGET_DB_SCHEMA:-'public'}

# The PG driver, used to connect to YB, is case-sensitive about database name.
if [ "${TARGET_DB_NAME}" == "" ]
then
	export TARGET_DB_NAME=`echo ${SOURCE_DB_NAME} | tr [A-Z] [a-z]`
fi