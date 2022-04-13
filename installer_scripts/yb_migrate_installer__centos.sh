#!/usr/bin/env bash

# This script installs yb_migrate in /usr/local/bin and all of its dependencies.

LOG_FILE=yb_migrate_installer.log

export GOROOT="/tmp/go"

ORA2PG_VERSION="23.1"
GO_VERSION="go1.18"
YUM="sudo yum install -y -q"
GO="$GOROOT/bin/go"

# separate file for bashrc related settings
RC_FILE="$HOME/.yb_migrate_installer_bashrc"
touch $RC_FILE
# just in case last execution of script stopped in between
source $RC_FILE


main() {
	set -x
	output "Installing RPM dependencies."
	$YUM which wget git gcc make 1>&2
	$YUM perl-5.16.3 perl-DBI-1.627 perl-App-cpanminus 1>&2
	$YUM mysql-devel 1>&2
	$YUM https://download.postgresql.org/pub/repos/yum/reporpms/EL-7-x86_64/pgdg-redhat-repo-latest.noarch.rpm 1>&2
	$YUM postgresql14-server 1>&2
	$YUM perl-ExtUtils-MakeMaker 1>&2

	install_golang

	output "Installing Oracle instant clients."
	OIC_URL="https://download.oracle.com/otn_software/linux/instantclient/215000"
	$YUM \
		${OIC_URL}/oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm \
		${OIC_URL}/oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm 1>&2

	output "Set environment variables in the $RC_FILE ."
	insert_into_rc_file 'export ORACLE_HOME=/usr/lib/oracle/21/client64'
	insert_into_rc_file 'export LD_LIBRARY_PATH=$ORACLE_HOME/lib:$LD_LIBRARY_PATH'
	insert_into_rc_file 'export PATH=$PATH:$ORACLE_HOME/bin'
	source $RC_FILE

	install_ora2pg

	output "Installing yb_migrate."
	$GO install github.com/yugabyte/ybm/yb_migrate@latest
	sudo mv -f $HOME/go/bin/yb_migrate /usr/local/bin

	update_bashrc
}


# Output a line to both STDOUT and STDERR. Use this function to output something to
# user as well as in the log file.
output() {
	set +x
	echo $1
	>&2 echo $1
	set -x
}


install_golang() {
	if [ -x ${GO} ]; then
		output "Found golang."
		return
	fi
	output "Installing golang."

	wget --no-verbose https://golang.org/dl/${GO_VERSION}.linux-amd64.tar.gz 1>&2
	if [ $? -ne 0 ]; then
		output "GoLang not installed! Check $LOG_FILE for more details."
		exit
	fi

	rm -rf $GOROOT
	tar -C /tmp -xzf ${GO_VERSION}.linux-amd64.tar.gz 1>&2
	if [ $? -ne 0 ]; then
		output "GoLang not installed! Check $LOG_FILE for more details."
		exit
	fi

	rm ${GO_VERSION}.linux-amd64.tar.gz
}


# Insert a line into the RC_FILE, if the line does not exist already.
insert_into_rc_file() {
	local line=$1
	grep -qxF "$line" $RC_FILE || echo $line >> $RC_FILE
}


ora2pg_license_acknowledgement() {
cat << EOT
	 ---------------------------------------------------------------------------------------------------------------
	|                                                        IMPORTANT                                              |
	| Ora2Pg is licensed under the GNU General Public License available at https://ora2pg.darold.net/license.html   |
	| By indicating "I Accept" through an affirming action, you indicate that you accept the terms                  |
	| of the GNU General Public License  Agreement and you also acknowledge that you have the authority,            |
	| on behalf of your company, to bind your company to such terms.                                                |
	| You may then download or install the file.                                                                    |
	 ---------------------------------------------------------------------------------------------------------------
EOT

	while true; do
	    echo -n "Do you Accept? (Y/N) "
	    read yn
	    case $yn in
		[Yy]* )
			break;;
		[Nn]* )
			exit;;
		* ) ;;
	    esac
	done
}


install_ora2pg() {
	which ora2pg 1>&2
	if  [ $? -eq 0 ]; then
		output "ora2pg is already installed. Skipping."
		return
	fi

	ora2pg_license_acknowledgement

	output "Installing perl DB drivers required for ora2pg."
	sudo cpanm DBD::mysql Test::NoWarnings DBD::Oracle 1>&2

	output "Installing ora2pg."
	wget --no-verbose https://github.com/darold/ora2pg/archive/refs/tags/v${ORA2PG_VERSION}.tar.gz 1>&2
	if [ $? -ne 0 ]; then
	    output "ora2pg not installed! Check $LOG_FILE for more details."
	    exit
	fi

	tar -xf v${ORA2PG_VERSION}.tar.gz 1>&2

	cd ora2pg-${ORA2PG_VERSION}/
	perl Makefile.PL 1>&2
	if [ $? -ne 0 ]; then
	    output "ora2pg not installed! Check $LOG_FILE for more details."
	    exit
	fi
	make 1>&2 && sudo make install 1>&2
	if [ $? -ne 0 ]; then
	    output "ora2pg not installed! Check $LOG_FILE for more details."
	    exit
	fi
	cd ..
	rm -f v${ORA2PG_VERSION}.tar.gz
	output "ora2pg installed."
}


update_bashrc() {
	line="source $RC_FILE"
	grep -qxF "$line" $HOME/.bashrc
	if [ $? -eq 0 ]
	then
		output "No need to update bashrc again."
		return
	fi
	while true; do
		printf "\n*** File - $RC_FILE contents ***\n\n"
		cat $RC_FILE

		printf "\n\nAdd $RC_FILE to $HOME/.bashrc file (Y/N)? "
		read yn
		case $yn in
		[Yy]* )
			grep -qxF "$line" $HOME/.bashrc || echo $line >> $HOME/.bashrc
			echo "execute: \"source $HOME/.bashrc\" before continuing in same shell"
			break;;
		[Nn]* )
			echo "execute: \"source $RC_FILE\" before continuing to have paths set in current shell"
			break;;
		* ) ;;
		esac
	done

	output "Done"
}


main 2>> yb_migrate_installer.log
