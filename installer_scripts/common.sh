LOG_FILE=yb_migrate_installer.log

export GOROOT="/tmp/go"

ORA2PG_VERSION="23.1"
GO_VERSION="go1.18"
GO="$GOROOT/bin/go"

# separate file for bashrc related settings
RC_FILE="$HOME/.yb_migrate_installer_bashrc"
touch $RC_FILE
# just in case last execution of script stopped in between
source $RC_FILE


on_exit() {
        rc=$?
        set +x
        if [ $rc -eq 0 ]
        then
                echo "Done!"
        else
                echo "Script failed. Check log file ${LOG_FILE} ."
        fi
}


update_yb_migrate_bashrc() {
	output "Set environment variables in the $RC_FILE ."
	insert_into_rc_file 'export ORACLE_HOME=/usr/lib/oracle/21/client64'
	insert_into_rc_file 'export LD_LIBRARY_PATH=$ORACLE_HOME/lib:$LD_LIBRARY_PATH'
	insert_into_rc_file 'export PATH=$PATH:$ORACLE_HOME/bin'
	source $RC_FILE
}


install_yb_migrate() {
	output "Installing yb_migrate."
	$GO install github.com/yugabyte/yb-db-migration/yb_migrate@latest
	sudo mv -f $HOME/go/bin/yb_migrate /usr/local/bin
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
	rm -rf $GOROOT
	tar -C /tmp -xzf ${GO_VERSION}.linux-amd64.tar.gz 1>&2
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
	if which ora2pg > /dev/null 2>&1
	then
		output "ora2pg is already installed. Skipping."
		return
	fi

	ora2pg_license_acknowledgement

	output "Installing perl DB drivers required for ora2pg."
	sudo cpanm DBD::mysql Test::NoWarnings DBD::Oracle 1>&2

	output "Installing ora2pg."
	wget --no-verbose https://github.com/darold/ora2pg/archive/refs/tags/v${ORA2PG_VERSION}.tar.gz 1>&2
	tar -xf v${ORA2PG_VERSION}.tar.gz 1>&2
	cd ora2pg-${ORA2PG_VERSION}/
	perl Makefile.PL 1>&2
	make 1>&2
	sudo make install 1>&2
	cd ..
	rm -f v${ORA2PG_VERSION}.tar.gz
	output "ora2pg installed."
}


update_bashrc() {
	line="source $RC_FILE"
	if grep -qxF "$line" $HOME/.bashrc
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
}
