# Copyright 2022 Google Inc.

%include %build_rpm_options

Summary: "Google Cloud Agent for SQL Server."
Group: Application
License: ASL 2.0
Vendor: Google, Inc.
Provides: google-cloud-sql-server-agent

%description
"Google Cloud Agent for SQL Server."

%define _confdir /etc/%{name}
%define _bindir /usr/bin
%define _docdir /usr/share/doc/%{name}
%define _servicedir /usr/share/%{name}/service

%install
# clean away any previous RPM build root
/bin/rm --force --recursive "${RPM_BUILD_ROOT}"

%include %build_rpm_install

%files
%defattr(-,root,root)
%attr(755,root,root) %{_bindir}/google_cloud_sql_server_agent
%config(noreplace) %attr(0666,root,root) %{_confdir}/config-default.json
%attr(0644,root,root) %{_servicedir}/%{name}.service
%attr(0644,root,root) %{_docdir}/LICENSE
%attr(0644,root,root) %{_docdir}/README.md
%attr(0644,root,root) %{_docdir}/THIRD_PARTY_NOTICES

%pre
# If we need to check install / upgrade ($1 = 1 is install, $1 = 2 is upgrade)

# SQL Server Agent
# if the agent is running - stop it
if `systemctl is-active --quiet %{name} > /dev/null 2>&1`; then
    systemctl stop %{name}
fi

%post

# create configuration.json file
if ! test -f %{_confdir}/configuration.json; then
  mv %{_confdir}/config-default.json %{_confdir}/configuration.json
fi

# link the systemd service and reload the daemon
# RHEL
if [ -d "/lib/systemd/system/" ]; then
    cp -f %{_servicedir}/%{name}.service /lib/systemd/system/%{name}.service
    systemctl daemon-reload
fi
# SLES
if [ -d "/usr/lib/systemd/system/" ]; then
    cp -f %{_servicedir}/%{name}.service /usr/lib/systemd/system/%{name}.service
    systemctl daemon-reload
fi

# enable and start the agent
systemctl enable %{name}
systemctl start %{name}

# next steps instructions
echo ""
echo "##########################################################################"
echo "Google Cloud Agent for SQL Server has been installed"
echo ""
echo "You can view the logs in /var/log/%{name}.log"
echo ""
echo "Verify the agent is running with: "
echo  "    sudo systemctl status %{name}"
echo "Configuration is available in %{_confdir}/configuration.json"
echo ""
echo "Documentation can be found at https://cloud.google.com/sql-server"
echo "##########################################################################"
echo ""

%preun
# $1 == 0 is uninstall, $1 == 1 is upgrade
if [ "$1" = "0" ]; then
  # Uninstall
  # if the agent is running - stop it
  if `type "systemctl" > /dev/null 2>&1 && systemctl is-active --quiet %{name}`; then
      systemctl stop %{name}
  fi
  # if the agent is enabled - disable it
  if `type "systemctl" > /dev/null 2>&1 && systemctl is-enabled --quiet %{name}`; then
      systemctl disable %{name}
  fi
fi

%postun
# $1 == 0 is uninstall, $1 == 1 is upgrade
if [ "$1" = "0" ]; then
  # Uninstall
  rm -f /lib/systemd/system/%{name}.service
  rm -f /usr/lib/systemd/system/%{name}.service
  rm -f /tmp/%{name}*
  rm -fr %{_docdir}
  rm -fr %{_confdir}
  rm -fr /var/log/%{name}*
fi
