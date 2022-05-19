#!/usr/bin/env bash

set -e

source common.sh

trap on_exit EXIT

main() {
	set -x

	output "Installing packages."
	sudo apt update 1>&2
	sudo apt-get -y install wget  1>&2
	sudo apt-get -y install perl  1>&2
	sudo apt-get -y install libdbi-perl 1>&2
	sudo apt-get -y install libaio1 1>&2
	sudo apt-get -y install cpanminus 1>&2
	sudo apt-get -y install libmysqlclient-dev 1>&2

	install_golang
	install_postgres
	#install_oracle_instant_clients
	#update_yb_migrate_bashrc
	#install_ora2pg
	install_yb_migrate
	update_bashrc

	set +x
}

install_postgres() {
	output "Installing postgres."
	line="deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main"
	sudo grep -qxF "$line" /etc/apt/sources.list.d/pgdg.list || echo "$line" | sudo tee /etc/apt/sources.list.d/pgdg.list 1>&2
	wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo apt-key add -
	sudo apt-get update 1>&2
	sudo apt-get -y install postgresql-14 1>&2
	output "Postgres Installed."
}

install_oracle_instant_clients() {
	output "Installing Oracle Instant Clients."
	sudo apt-get -y install alien 1>&2
	install_oic oracle-instantclient-basic
	install_oic oracle-instantclient-devel
	install_oic oracle-instantclient-jdbc
	install_oic oracle-instantclient-sqlplus
	output "Installed Oracle Instance Clients."
}

install_oic() {
	if dpkg -l | grep -q -w $1
	then
		echo "$1 is already installed."
	else
		rpm_name="$1-21.5.0.0.0-1.x86_64.rpm"
		wget https://download.oracle.com/otn_software/linux/instantclient/215000/${rpm_name} 1>&2
		sudo alien -i ${rpm_name} 1>&2
		rm ${rpm_name}
	fi
}

main 2> $LOG_FILE
