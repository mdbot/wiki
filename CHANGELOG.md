# Changelog

## 5.0.4 - 2025-05-09

### Bug fixes

* Fixed bad request errors being served with 500 error codes

## 5.0.3 - 2023-12-08

_No code changes, just changes to the release process._

## 5.0.2 - 2023-12-08

_No code changes, just changes to the release process._

## 5.0.1 - 2023-12-08

_No code changes, just changes to the release process._

## 5.0.0 - 2023-12-08

### Features

* Add support for making tables sortable by including the `{.sortable}` class
* Add support for embedding PDF files

### Bug fixes

* All default pages are now created properly, previously only the first
  missing page was created
* The first created user is now given admin access properly. Previously you
  had to restart the wiki after creating the first user to upgrade them to an
  admin

## 4.0.0 - 2021-04-26

### Features

* Support for embedding video and audio files
* Edit links are no longer shown if the user lacks permission
* The sidebar is now shown on error pages if the user has permission to read it
  (previously it was always hidden for errors)
* The wiki version number is now shown in the footer
* Add ability to apply classes to elements in markdown, e.g.:
   ```markdown
   {.thumbnail}
   [[image]]
   ```

## 3.0.0 - 2021-04-09

### Features

* Added basic search and go-to-page functionality
* Added API endpoint to list files and pages
* File names and wiki links now auto-complete in the editor
* Pages can now be viewed as of a specific change
* Pages can now be reverted to a specific change
* You can now view the diff between any pair of changes
* Improve the layout of the recent changes and page history pages
* Add an RSS feed for recent changes
* Files uploaded through drag-and-drop are now automatically placed in the
  same folder as the page they're inserted into

### Bug fixes

* Fix the file list always using windows formatted paths
* Fix performance issues with the recent changes list
* Fixed some error pages being sent as plain text

## 2.0.0 - 2021-04-06

### Features

* Support for deleting files
* Site name, favicon and logo are now customisable
* Files can be uploaded by dragging and dropping on the editor
* The editor is now powered by CodeMirror, enabling syntax highlighting
* Add recent changes feed
* User sessions are now invalidated when their password is changed
* Users can now change their own passwords
* Admins can now set permission levels for users. Valid permissions are:
  login, read, write, admin.
* Added customisable site-wide side bar

### Bug fixes

* Don't allow page renames if the source page doesn't exist
* Renaming a page now redirects you to the new location, instead of the wrong one

## 1.0.1 - 2021-04-03

### Features

* Allow use of safe HTML tags in pages, and add flag to enable dangerous ones

## 1.0.0 - 2021-04-03

_Initial release._
