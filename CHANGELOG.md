# v5.0.0

## Features

* Add support for making tables sortable by including the `{.sortable}` class
* Add support for embedding PDF files

## Bug fixes

* All default pages are now created properly, previously only the first
  missing page was created
* The first created user is now given admin access properly. Previously you
  had to restart the wiki after creating the first user to upgrade them to an
  admin

# v4.0.0

## Features

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

# v3.0.0

## Features

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

## Bug fixes

* Fix the file list always using windows formatted paths
* Fix performance issues with the recent changes list
* Fixed some error pages being sent as plain text

# v2.0.0

## Features

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

## Bug fixes

* Don't allow page renames if the source page doesn't exist
* Renaming a page now redirects you to the new location, instead of the wrong one

# v1.0.1

## Features

* Allow use of safe HTML tags in pages, and add flag to enable dangerous ones