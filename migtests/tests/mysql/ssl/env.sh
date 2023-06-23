# jenkins automation image has the necessary key/certs created
export SOURCE_DB_TYPE="mysql"
export SOURCE_DB_NAME=${SOURCE_DB_NAME:-"datatypes"}
export SOURCE_DB_SSL_MODE="verify-ca"
export SOURCE_DB_SSL_CERT="/home/ubuntu/.mysql/client-cert.pem"
export SOURCE_DB_SSL_KEY="/home/ubuntu/.mysql/client-key.pem"
export SOURCE_DB_SSL_ROOT_CERT="/home/ubuntu/.mysql/ca.pem"