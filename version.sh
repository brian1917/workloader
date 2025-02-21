#!/bin/bash

# Read the version from the 'version' file
VERSION=$(cat version)

# Create the version.rc file
cat <<EOF > version.rc
#include <winver.h>

VS_VERSION_INFO VERSIONINFO
FILEVERSION 1,0,0,0
PRODUCTVERSION 1,0,0,0
FILEFLAGSMASK 0x3fL
FILEFLAGS 0x0L
FILEOS VOS_NT_WINDOWS32
FILETYPE VFT_APP
FILESUBTYPE VFT2_UNKNOWN
BEGIN
    BLOCK "StringFileInfo"
    BEGIN
        BLOCK "040904b0"
        BEGIN
            VALUE "FileDescription", "Workloader is an open-source CLI tool that leverages the Illumio API to manage resources and automate common tasks."
            VALUE "InternalName", "workloader"
            VALUE "OriginalFilename", "workloader.exe"
            VALUE "ProductName", "Workloader"
            VALUE "ProductVersion", "$VERSION"
        END
    END
    BLOCK "VarFileInfo"
    BEGIN
        VALUE "Translation", 0x0409, 1200
    END
END
EOF