# Ubuntu Pro app - User Interface

This is the GUI of the Ubuntu Pro app for Windows, which configures Ubuntu instances on WSL.

It is a Flutter monorepo split into separate packages:

- p4w_gui - the application;
- p4w_ms_store - a Flutter plugin to expose the MS Store WinRT APIs to the Dart land.

Even though this project is only meant to run on Windows, the packages listed above must be able to run on Linux for testability purposes.
