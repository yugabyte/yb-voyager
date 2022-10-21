export TARGET_DB_HOST=${TARGET_DB_HOST:-"10.9.200.131"}
export TARGET_DB_PORT=${TARGET_DB_PORT:-5433}
export TARGET_DB_USER=${TARGET_DB_USER:-"ybvoyager"}
export TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD:-"password"}
export TARGET_DB_NAME=${TARGET_DB_PASSWORD:-"postgres"}
# The PG driver, used to connect to YB, is case-sensitive about database name.
if [ "${TARGET_DB_NAME}" == "" ]
then
	export TARGET_DB_NAME=`echo ${SOURCE_DB_NAME} | tr [A-Z] [a-z]`
fi
