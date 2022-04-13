#!/usr/bin/env bash

# This script installs yb_migrate in /usr/local/bin and all of its dependencies.

set -e

source common.sh

trap on_exit EXIT

YUM="sudo yum install -y -q"

main() {
	set -x

	output "Installing RPM dependencies."
	$YUM which wget git gcc make 1>&2
	$YUM perl-5.16.3 perl-DBI-1.627 perl-App-cpanminus 1>&2
	$YUM mysql-devel 1>&2
	$YUM https://download.postgresql.org/pub/repos/yum/reporpms/EL-7-x86_64/pgdg-redhat-repo-latest.noarch.rpm 1>&2 || true
	$YUM postgresql14-server 1>&2
	$YUM perl-ExtUtils-MakeMaker 1>&2

	install_golang
	install_oracle_instant_clients
	update_yb_migrate_bashrc
	install_ora2pg
	install_yb_migrate
	update_bashrc

	set +x
}


install_oracle_instant_clients() {
	output "Installing Oracle instant clients."
	OIC_URL="https://download.oracle.com/otn_software/linux/instantclient/215000"
	$YUM \
		${OIC_URL}/oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm 1>&2 || true
}


main 2>> yb_migrate_installer.log
