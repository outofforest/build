Name:    outofforest-build
Version: %version
Release: 1
Summary: Tool to build and run project build tool
URL:     https://github.com/outofforest/build
License: MIT

Requires: golang

%description
Tool to build and run project build tool

%prep
%setup

%setup

%install
mkdir -p %{buildroot}/usr/bin
cp ./build %{buildroot}/usr/bin/outofforest-build

%files
/usr/bin/outofforest-build

%post
