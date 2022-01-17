Name:           gdu
Version:        5.12.1
Release:        1
Summary:        Pretty fast disk usage analyzer written in Go.
BuildArch:      x86_64

License:        MIT

Source0:        %{name}-%{version}.tar.gz

#BuildRequires:  golang
Requires:       bash

Provides:       %{name} = %{version}

%description
Pretty fast disk usage analyzer written in Go.

%global debug_package %{nil}

%prep
%autosetup


%build
# go build -v -o %{name}
GO111MODULE=on CGO_ENABLED=0 go build \
-trimpath \
-buildmode=pie \
-mod=readonly \
-modcacherw \
-ldflags \
"-s -w \
-X 'github.com/dundee/gdu/v5/build.Version=$(git describe)' \
-X 'github.com/dundee/gdu/v5/build.User=$(id -u -n)' \
-X 'github.com/dundee/gdu/v5/build.Time=$(LC_ALL=en_US.UTF-8 date)'" \
-o %{name} github.com/dundee/gdu/v5/cmd/gdu

%install
rm -rf $RPM_BUILD_ROOT
install -Dpm 0755 %{name} %{buildroot}%{_bindir}/%{name}
# install -Dpm 0755 %{name}.1 $RPM_BUILD_ROOT%{_mandir}/man1/gdu.1
install -Dpm 0755 %{name}.1 $RPM_BUILD_ROOT%{_mandir}/man1/gdu.1

%check

%post

%preun

%files
%{_bindir}/gdu
# %{_mandir}/man1/gdu.1.gz
%{_mandir}/man1/gdu.1.gz

%changelog
* Tue Dec 14 2021 Danie de Jager - 5.12.1-1
- Bump to 5.12.1-1
- fixed listing devices on NetBSD
- escape file names (#111)
- fixed filtering
* Fri Dec 3 2021 Danie de Jager - 5.12.0-1
- Bump to 5.12.0-1
* Fri Dec 3 2021 Danie de Jager - 5.11.0-2
- Compile with go 1.17.4
* Sun Nov 28 2021 Danie de Jager - 5.11.0-1
- Bump to 5.11.0
* Tue Nov 23 2021 Danie de Jager - 5.10.1-1
- Bump to 5.10.1
* Wed Nov 10 2021 Danie de Jager - 5.10.0-1
- Bump to 5.10.01
* Mon Oct 25 2021 Danie de Jager - 5.9.0-1
- Bump to 5.9.0
* Mon Sep 27 2021 Danie de Jager - 5.8.1-2
- Remove pandoc requirement.
* Sun Sep 26 2021 Danie de Jager - 5.8.1-1
- Bump to 5.8.1
* Thu Sep 23 2021 Danie de Jager - 5.8.0-2
- Bump to 5.8.0
* Tue Sep 7 2021 Danie de Jager - 5.7.0-1
- Bump to 5.7.0
* Sat Aug 28 2021 Danie de Jager - 5.6.2-1
- Bump to 5.6.2
- Compiled with go 1.17
* Fri Aug 27 2021 Danie de Jager - 5.6.1-1
- Bump to 5.6.1
* Mon Aug 23 2021 Danie de Jager - 5.6.0-1
- Bump to 5.6.0
* Fri Aug 13 2021 Danie de Jager - 5.5.0-2
- Compiled with go 1.16.7
* Mon Aug 2 2021 Danie de Jager - 5.5.0-1
- Bump to 5.5.0
* Mon Jul 26 2021 Danie de Jager - 5.4.0-1
- Bump to 5.4.0
* Thu Jul 22 2021 Danie de Jager - 5.3.0-2
- First release
