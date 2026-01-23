Name:           kartoza-timesheet
Version:        0.3.1
Release:        1%{?dist}
Summary:        Terminal-based timesheet application

License:        MIT
URL:            https://github.com/kartoza/go-timesheets-go
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.21
BuildRequires:  git

%description
Kartoza Timesheet is a beautiful terminal user interface (TUI)
application for tracking time spent on projects and tasks.

Features include:
- Project and task management
- Time entry tracking with start/stop functionality
- Waybar integration for desktop status display
- ERPNext integration for timesheet submission
- CLI commands for automation and scripting

%prep
%autosetup

%build
export CGO_ENABLED=0
export GOFLAGS="-mod=vendor"
go build -ldflags "-s -w -X main.version=%{version}" -o %{name} .

%install
install -D -m 0755 %{name} %{buildroot}%{_bindir}/%{name}
install -D -m 0644 %{name}.desktop %{buildroot}%{_datadir}/applications/%{name}.desktop

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_datadir}/applications/%{name}.desktop

%changelog
* Fri Jan 17 2026 Tim Sutton <tim@kartoza.com> - 0.3.1-1
- Initial RPM package release
- Full TUI application for timesheet management
- CLI commands: start, stop, status, current, today, list
- Waybar integration with JSON output
- ERPNext integration for timesheet submission
