echo "clang before version 21 fails to compile code that depends on MSVC STL variant implementation."
echo "Please upgrade Visual Studio toolchain to provide clang-tidy version 21 or later and then delete this file."

@echo off
REM For reference, here are some of the errors seen with clang-tidy version 19.
REM
REM "D:\UP4W\main\msix\gui\gui.vcxproj" (build target) (4:5) ->
REM (Build target) ->
REM  C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.44.35207\include\compare(347,18): error : satisfaction of constraint 'requires { { _Left < _Right } -> _Boolean_te
REM stable; { _Right < _Left } -> _Boolean_testable; }' depends on itself [clang-diagnostic-error] [D:\UP4W\main\gui\packages\ubuntupro\build\windows\x64\plugins\p4w_ms_store\p4w_ms_store_plugi
REM n.vcxproj] [D:\UP4W\main\msix\gui\gui.vcxproj]
REM  C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.44.35207\include\variant(1292,20): error : no matching function for call to object of type 'std::less<void>' [clan
REM g-diagnostic-error] [D:\UP4W\main\gui\packages\ubuntupro\build\windows\x64\plugins\p4w_ms_store\p4w_ms_store_plugin.vcxproj] [D:\UP4W\main\msix\gui\gui.vcxproj]
REM  D:\UP4W\main\gui\packages\ubuntupro\windows\flutter\ephemeral\.plugin_symlinks\p4w_ms_store\windows\p4w_ms_store_plugin_impl.cpp(32,17): error : no matching member function for call to 'S
REM uccess' [clang-diagnostic-error] [D:\UP4W\main\gui\packages\ubuntupro\build\windows\x64\plugins\p4w_ms_store\p4w_ms_store_plugin.vcxproj] [D:\UP4W\main\msix\gui\gui.vcxproj]
REM  D:\UP4W\main\msix\gui\gui.targets(22,3): error MSB3073: The command "flutter build windows --debug  " exited with code 1. [D:\UP4W\main\msix\gui\gui.vcxproj]
@echo on